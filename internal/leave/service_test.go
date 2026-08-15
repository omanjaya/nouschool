package leave

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
	"github.com/omanjaya/nouschool/internal/platform/storage"
)

// fixedNow — waktu "sekarang" tetap dipakai seluruh test (10:00 WIB, 16 Agustus 2026).
var fixedNow = time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)

// -- fakeRepo: implementasi leaveRepository in-memory (tanpa DB) — sama pola
// dengan internal/attendance/service_test.go.

type fakeRepo struct {
	settings Settings
	requests map[int64]RequestRecord
	steps    map[int64]StepRecord
	order    map[int64][]int64 // requestID -> step IDs berurutan
	users    map[int64]string
	nextReq  int64
	nextStep int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		settings: DefaultSettings(),
		requests: map[int64]RequestRecord{},
		steps:    map[int64]StepRecord{},
		order:    map[int64][]int64{},
		users:    map[int64]string{},
	}
}

func (f *fakeRepo) GetSettings(ctx context.Context, schoolID int64) (Settings, error) {
	return f.settings, nil
}

func (f *fakeRepo) SubmitRequest(ctx context.Context, in SubmitRequestParams) (RequestRecord, []StepRecord, error) {
	f.nextReq++
	id := f.nextReq
	status := StatusPending
	if in.AutoApprove {
		status = StatusApproved
	}
	rec := RequestRecord{
		ID: id, SchoolID: in.SchoolID, TeacherID: in.TeacherID, TeacherName: f.users[in.TeacherID],
		Type: in.Type, DateStart: in.DateStart, DateEnd: in.DateEnd, Reason: in.Reason,
		Attachment: in.Attachment, AttachmentName: in.AttachmentName, AttachmentMime: in.AttachmentMime,
		Status: status, CreatedAt: fixedNow,
	}
	f.requests[id] = rec

	stepRecs := make([]StepRecord, 0, len(in.Steps))
	for _, sp := range in.Steps {
		f.nextStep++
		sid := f.nextStep
		st := StepRecord{
			ID: sid, SchoolID: in.SchoolID, LeaveRequestID: id, StepOrder: sp.StepOrder,
			ApproverRole: sp.ApproverRole, ApproverUserID: sp.ApproverUserID,
		}
		f.steps[sid] = st
		f.order[id] = append(f.order[id], sid)
		stepRecs = append(stepRecs, st)
	}
	return rec, stepRecs, nil
}

func (f *fakeRepo) GetRequestByID(ctx context.Context, schoolID, id int64) (RequestRecord, error) {
	rec, ok := f.requests[id]
	if !ok || rec.SchoolID != schoolID {
		return RequestRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) ListRequests(ctx context.Context, schoolID, teacherID int64, status string) ([]RequestRecord, error) {
	var out []RequestRecord
	for id := int64(1); id <= f.nextReq; id++ {
		rec, ok := f.requests[id]
		if !ok || rec.SchoolID != schoolID {
			continue
		}
		if teacherID != 0 && rec.TeacherID != teacherID {
			continue
		}
		if status != "" && rec.Status != status {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (f *fakeRepo) ListStepsForRequest(ctx context.Context, requestID int64) ([]StepRecord, error) {
	ids := f.order[requestID]
	out := make([]StepRecord, 0, len(ids))
	for _, id := range ids {
		out = append(out, f.steps[id])
	}
	return out, nil
}

func (f *fakeRepo) ListStepsForRequests(ctx context.Context, requestIDs []int64) ([]StepRecord, error) {
	var out []StepRecord
	for _, rid := range requestIDs {
		steps, _ := f.ListStepsForRequest(ctx, rid)
		out = append(out, steps...)
	}
	return out, nil
}

func (f *fakeRepo) CancelRequest(ctx context.Context, schoolID, id, teacherID int64) (int64, error) {
	rec, ok := f.requests[id]
	if !ok || rec.SchoolID != schoolID || rec.TeacherID != teacherID || rec.Status != StatusPending {
		return 0, nil
	}
	rec.Status = StatusCanceled
	f.requests[id] = rec
	return 1, nil
}

func (f *fakeRepo) GetStepByID(ctx context.Context, schoolID, stepID int64) (StepRecord, error) {
	st, ok := f.steps[stepID]
	if !ok || st.SchoolID != schoolID {
		return StepRecord{}, ErrNotFound
	}
	return st, nil
}

func (f *fakeRepo) ListApprovalQueue(ctx context.Context, schoolID, userID int64, role string) ([]StepRecord, error) {
	var out []StepRecord
	for id := int64(1); id <= f.nextReq; id++ {
		rec, ok := f.requests[id]
		if !ok || rec.SchoolID != schoolID || rec.Status != StatusPending {
			continue
		}
		steps, _ := f.ListStepsForRequest(ctx, id)
		for _, st := range steps {
			if st.Decision == "" {
				if isStepApprover(st.ApproverUserID, st.ApproverRole, userID, role) {
					out = append(out, st)
				}
				break // hanya step_order terendah yang belum diputuskan
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) DecideStep(ctx context.Context, in DecideStepInput) (StepRecord, RequestRecord, error) {
	st, ok := f.steps[in.StepID]
	if !ok || st.SchoolID != in.SchoolID || st.Decision != "" {
		return StepRecord{}, RequestRecord{}, ErrConflict
	}
	st.Decision = in.Decision
	st.DecidedBy = in.DeciderID
	st.DecidedByName = f.users[in.DeciderID]
	now := fixedNow
	st.DecidedAt = &now
	st.Comment = in.Comment
	f.steps[in.StepID] = st

	if in.NewReqStatus != "" {
		rec, ok := f.requests[in.RequestID]
		if !ok || rec.Status != StatusPending {
			return StepRecord{}, RequestRecord{}, ErrConflict
		}
		rec.Status = in.NewReqStatus
		f.requests[in.RequestID] = rec
	}
	return st, f.requests[in.RequestID], nil
}

// Summary meniru agregasi SQL asli: satu baris per (teacher,type), hari
// dihitung terpotong (clipped) ke rentang [from,to] — lihat LeaveSummaryByRange.
func (f *fakeRepo) Summary(ctx context.Context, schoolID int64, from, to time.Time) ([]SummaryRowRaw, error) {
	rows := map[[2]any]int64{}
	var keys [][2]any
	for id := int64(1); id <= f.nextReq; id++ {
		rec, ok := f.requests[id]
		if !ok || rec.SchoolID != schoolID || rec.Status != StatusApproved {
			continue
		}
		if rec.DateStart.After(to) || rec.DateEnd.Before(from) {
			continue
		}
		start, end := rec.DateStart, rec.DateEnd
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		days := int64(end.Sub(start).Hours()/24) + 1
		k := [2]any{rec.TeacherID, rec.Type}
		if _, ok := rows[k]; !ok {
			keys = append(keys, k)
		}
		rows[k] += days
	}
	result := make([]SummaryRowRaw, 0, len(keys))
	for _, k := range keys {
		teacherID := k[0].(int64)
		typ := k[1].(string)
		result = append(result, SummaryRowRaw{TeacherID: teacherID, TeacherName: f.users[teacherID], Type: typ, Days: rows[k]})
	}
	return result, nil
}

func (f *fakeRepo) ApprovedOn(ctx context.Context, schoolID, teacherUserID int64, date time.Time) (bool, error) {
	for id := int64(1); id <= f.nextReq; id++ {
		rec, ok := f.requests[id]
		if !ok || rec.SchoolID != schoolID || rec.TeacherID != teacherUserID || rec.Status != StatusApproved {
			continue
		}
		if !date.Before(rec.DateStart) && !date.After(rec.DateEnd) {
			return true, nil
		}
	}
	return false, nil
}

// seedRequest menyisipkan request+step LANGSUNG (bypass Service.SubmitRequest
// dan filter self-skip-nya) — dipakai menguji guard eksplisit "approver tidak
// boleh memutuskan request miliknya sendiri" secara terisolasi dari logika skip.
func (f *fakeRepo) seedRequest(schoolID, teacherID int64, status string, stepApproverRole string, stepApproverUserID int64) (requestID, stepID int64) {
	f.nextReq++
	requestID = f.nextReq
	f.requests[requestID] = RequestRecord{
		ID: requestID, SchoolID: schoolID, TeacherID: teacherID, TeacherName: f.users[teacherID],
		Type: "izin", DateStart: fixedNow, DateEnd: fixedNow, Reason: "uji", Status: status, CreatedAt: fixedNow,
	}
	f.nextStep++
	stepID = f.nextStep
	f.steps[stepID] = StepRecord{
		ID: stepID, SchoolID: schoolID, LeaveRequestID: requestID, StepOrder: 1,
		ApproverRole: stepApproverRole, ApproverUserID: stepApproverUserID,
	}
	f.order[requestID] = []int64{stepID}
	return requestID, stepID
}

// -- fakeIdentity: implementasi IdentityGateway untuk test --

type auditEntry struct {
	action   string
	oldValue any
	newValue any
}

type fakeIdentity struct {
	perms       map[string]map[string]bool
	logs        []auditEntry
	usersByRole map[string][]int64 // role -> user_id (fase 9, UsersWithRole)
}

func newFakeIdentity() *fakeIdentity {
	return &fakeIdentity{perms: map[string]map[string]bool{
		RoleAdminSekolah:  {PermLeaveManage: true},
		RoleKepalaSekolah: {PermLeaveApprove: true},
		RoleGuru:          {PermLeaveRequest: true, PermLeaveApprove: true},
	}}
}

func (f *fakeIdentity) HasPermission(role, perm string) bool { return f.perms[role][perm] }
func (f *fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	f.logs = append(f.logs, auditEntry{action: action, oldValue: oldValue, newValue: newValue})
	return nil
}

// UsersWithRole (fase 9) — fakeIdentity mengembalikan daftar kosong secara
// default (test yang butuh resolusi role-only mengisi f.usersByRole).
func (f *fakeIdentity) UsersWithRole(ctx context.Context, schoolID int64, role string) ([]int64, error) {
	return f.usersByRole[role], nil
}

func ctxAs(role string, userID int64) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Sekolah Uji", Slug: "uji", Timezone: "Asia/Jakarta"})
	return ctx
}

func newTestService(t *testing.T, repo *fakeRepo, identity *fakeIdentity) *Service {
	t.Helper()
	return newServiceForTest(repo, identity, storage.New(t.TempDir()), clock.Fixed{T: fixedNow})
}

func mustSubmit(t *testing.T, svc *Service, ctx context.Context, teacherID int64, in SubmitInput) Request {
	t.Helper()
	req, err := svc.SubmitRequest(ctx, teacherID, 1, in)
	if err != nil {
		t.Fatalf("SubmitRequest gagal: %v", err)
	}
	return req
}

func domainStatus(t *testing.T, err error) int {
	t.Helper()
	var de *httpx.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain error, got: %v", err)
	}
	return de.Status
}

// -- Settings.Validate --

func TestSettingsValidate(t *testing.T) {
	t.Run("types kosong ditolak", func(t *testing.T) {
		s := Settings{Types: nil, Chain: nil}
		if err := s.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
	t.Run("key tidak snake_case ditolak", func(t *testing.T) {
		s := Settings{Types: []LeaveType{{Key: "Izin Dinas", Label: "x"}}}
		if err := s.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
	t.Run("key duplikat ditolak", func(t *testing.T) {
		s := Settings{Types: []LeaveType{{Key: "izin", Label: "Izin"}, {Key: "izin", Label: "Izin 2"}}}
		if err := s.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
	t.Run("chain role tidak dikenal ditolak", func(t *testing.T) {
		s := Settings{Types: DefaultSettings().Types, Chain: []ChainStep{{Role: "waka"}}}
		if err := s.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
	t.Run("chain role guru tanpa user_id ditolak", func(t *testing.T) {
		s := Settings{Types: DefaultSettings().Types, Chain: []ChainStep{{Role: RoleGuru}}}
		if err := s.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
	t.Run("chain role guru dengan user_id diterima", func(t *testing.T) {
		s := Settings{Types: DefaultSettings().Types, Chain: []ChainStep{{Role: RoleGuru, UserID: 42}}}
		if err := s.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("default valid", func(t *testing.T) {
		s := DefaultSettings()
		if err := s.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// -- SubmitRequest: validasi tanggal & tipe --

func TestSubmitRequestValidation(t *testing.T) {
	repo := newFakeRepo()
	repo.users[100] = "Guru Uji"
	svc := newTestService(t, repo, newFakeIdentity())
	ctx := ctxAs(RoleGuru, 100)

	cases := []struct {
		name string
		in   SubmitInput
	}{
		{"start setelah end", SubmitInput{Type: "izin", DateStart: "2026-08-20", DateEnd: "2026-08-18", Reason: "x"}},
		{"lampau lebih dari 30 hari", SubmitInput{Type: "izin", DateStart: "2026-07-01", DateEnd: "2026-07-02", Reason: "x"}},
		{"reason kosong", SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "  "}},
		{"tipe tidak dikenal", SubmitInput{Type: "cuti_besar", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"}},
		{"format tanggal salah", SubmitInput{Type: "izin", DateStart: "17-08-2026", DateEnd: "2026-08-17", Reason: "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.SubmitRequest(ctx, 100, 1, c.in)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if status := domainStatus(t, err); status != 422 {
				t.Fatalf("expected 422, got %d", status)
			}
		})
	}

	t.Run("valid diterima", func(t *testing.T) {
		req := mustSubmit(t, svc, ctx, 100, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-18", Reason: "Acara keluarga"})
		if req.Days != 2 {
			t.Fatalf("expected 2 hari, got %d", req.Days)
		}
		if req.Status != StatusPending {
			t.Fatalf("expected pending, got %s", req.Status)
		}
	})
}

// -- SNAPSHOT: perubahan settings setelah submit tidak mengubah steps request lama --

func TestSubmitRequest_SnapshotImmuneToLaterSettingsChange(t *testing.T) {
	repo := newFakeRepo()
	repo.users[100] = "Guru Uji"
	svc := newTestService(t, repo, newFakeIdentity())
	ctx := ctxAs(RoleGuru, 100)

	req := mustSubmit(t, svc, ctx, 100, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"})
	if len(req.Steps) != 1 {
		t.Fatalf("expected 1 step (default chain), got %d", len(req.Steps))
	}

	// admin mengubah chain settings SETELAH request dibuat (mis. tambah tingkat approval).
	repo.settings.Chain = []ChainStep{{Role: RoleAdminSekolah}, {Role: RoleKepalaSekolah}, {Role: RoleGuru, UserID: 999}}

	steps, err := repo.ListStepsForRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("snapshot request lama harus tetap 1 step walau settings berubah, got %d", len(steps))
	}
	if steps[0].ApproverRole != RoleKepalaSekolah {
		t.Fatalf("step lama harus tetap kepala_sekolah, got %s", steps[0].ApproverRole)
	}

	// request BARU setelah perubahan settings harus memakai chain yang baru
	// (3 step, tidak ada yang self-skip karena teacher=100/guru tidak match salah satu).
	req2 := mustSubmit(t, svc, ctx, 100, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"})
	if len(req2.Steps) != 3 {
		t.Fatalf("request baru harus pakai chain baru (3 step), got %d", len(req2.Steps))
	}
}

// -- Sequential: step 2 tidak bisa diputuskan sebelum step 1 --

func TestDecideStep_Sequential(t *testing.T) {
	repo := newFakeRepo()
	repo.users[200] = "Guru Pengaju"
	repo.users[5] = "Kepala Sekolah"
	repo.users[6] = "Admin Sekolah"
	repo.settings.Chain = []ChainStep{{Role: RoleKepalaSekolah}, {Role: RoleAdminSekolah}}
	svc := newTestService(t, repo, newFakeIdentity())

	req := mustSubmit(t, svc, ctxAs(RoleGuru, 200), 200, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"})
	if len(req.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(req.Steps))
	}
	step1ID, step2ID := req.Steps[0].ID, req.Steps[1].ID

	// step 2 dulu -> ditolak.
	_, err := svc.DecideStep(ctxAs(RoleAdminSekolah, 6), 6, 1, step2ID, DecisionApproved, "")
	if err == nil {
		t.Fatal("expected error memutuskan step 2 sebelum step 1, got nil")
	}
	if !errors.Is(err, errStepNotActive) {
		t.Fatalf("expected errStepNotActive, got: %v", err)
	}

	// step 1 -> diterima.
	result, err := svc.DecideStep(ctxAs(RoleKepalaSekolah, 5), 5, 1, step1ID, DecisionApproved, "oke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusPending {
		t.Fatalf("request harus tetap pending (step 2 belum), got %s", result.Status)
	}

	// step 1 tidak bisa diputuskan lagi.
	_, err = svc.DecideStep(ctxAs(RoleKepalaSekolah, 5), 5, 1, step1ID, DecisionApproved, "")
	if !errors.Is(err, errStepAlreadyDecided) {
		t.Fatalf("expected errStepAlreadyDecided, got: %v", err)
	}

	// sekarang step 2 boleh.
	result, err = svc.DecideStep(ctxAs(RoleAdminSekolah, 6), 6, 1, step2ID, DecisionApproved, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApproved {
		t.Fatalf("semua step approved -> request approved, got %s", result.Status)
	}
}

// -- Notify wiring (fase 9, docs/08-notification.md) --

// fakeLeaveNotifier mencatat setiap panggilan Notify.
type fakeLeaveNotifier struct {
	calls []leaveNotifyCall
}

type leaveNotifyCall struct {
	schoolID int64
	event    string
	userIDs  []int64
}

func (f *fakeLeaveNotifier) Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error {
	f.calls = append(f.calls, leaveNotifyCall{schoolID: schoolID, event: event, userIDs: userIDs})
	return nil
}

// TestSubmitAndDecideNotify — submit menotifikasi approver step aktif
// (role-only, resolusi via IdentityGateway.UsersWithRole); decide step 1
// (approved, chain belum selesai) menotifikasi pengaju (leave.decided) DAN
// approver step berikutnya (leave.submitted); decide step terakhir
// (approved) menotifikasi pengaju SAJA (tidak ada step berikutnya).
func TestSubmitAndDecideNotify(t *testing.T) {
	repo := newFakeRepo()
	repo.users[200] = "Guru Pengaju"
	repo.settings.Chain = []ChainStep{{Role: RoleKepalaSekolah}, {Role: RoleAdminSekolah}}
	identity := newFakeIdentity()
	identity.usersByRole = map[string][]int64{RoleKepalaSekolah: {5}, RoleAdminSekolah: {6}}
	svc := newTestService(t, repo, identity)
	notifier := &fakeLeaveNotifier{}
	svc.SetNotifier(notifier)

	req := mustSubmit(t, svc, ctxAs(RoleGuru, 200), 200, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"})
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notifikasi setelah submit, got %d", len(notifier.calls))
	}
	if notifier.calls[0].event != EventLeaveSubmitted || len(notifier.calls[0].userIDs) != 1 || notifier.calls[0].userIDs[0] != 5 {
		t.Fatalf("expected leave.submitted ke kepala_sekolah (user 5), got: %+v", notifier.calls[0])
	}
	step1ID, step2ID := req.Steps[0].ID, req.Steps[1].ID

	// Step 1 approved -> chain belum selesai -> notify pengaju (decided) + approver berikutnya (submitted).
	if _, err := svc.DecideStep(ctxAs(RoleKepalaSekolah, 5), 5, 1, step1ID, DecisionApproved, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notifier.calls) != 3 {
		t.Fatalf("expected 3 notifikasi setelah step 1 approved (1 submit + 2 decide), got %d: %+v", len(notifier.calls), notifier.calls)
	}
	if notifier.calls[1].event != EventLeaveDecided || notifier.calls[1].userIDs[0] != 200 {
		t.Fatalf("expected leave.decided ke pengaju (user 200), got: %+v", notifier.calls[1])
	}
	if notifier.calls[2].event != EventLeaveSubmitted || notifier.calls[2].userIDs[0] != 6 {
		t.Fatalf("expected leave.submitted ke admin_sekolah (user 6), got: %+v", notifier.calls[2])
	}

	// Step 2 approved -> chain SELESAI -> hanya notify pengaju (decided), TIDAK ada approver berikutnya.
	if _, err := svc.DecideStep(ctxAs(RoleAdminSekolah, 6), 6, 1, step2ID, DecisionApproved, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notifier.calls) != 4 {
		t.Fatalf("expected 4 notifikasi setelah step 2 approved (tidak ada approver berikutnya), got %d: %+v", len(notifier.calls), notifier.calls)
	}
	if notifier.calls[3].event != EventLeaveDecided || notifier.calls[3].userIDs[0] != 200 {
		t.Fatalf("expected leave.decided ke pengaju (user 200), got: %+v", notifier.calls[3])
	}
}

// -- Rejected menghentikan chain --

func TestDecideStep_RejectedStopsChain(t *testing.T) {
	repo := newFakeRepo()
	repo.users[200] = "Guru Pengaju"
	repo.users[5] = "Kepala Sekolah"
	repo.users[6] = "Admin Sekolah"
	repo.settings.Chain = []ChainStep{{Role: RoleKepalaSekolah}, {Role: RoleAdminSekolah}}
	svc := newTestService(t, repo, newFakeIdentity())

	req := mustSubmit(t, svc, ctxAs(RoleGuru, 200), 200, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"})
	step1ID, step2ID := req.Steps[0].ID, req.Steps[1].ID

	result, err := svc.DecideStep(ctxAs(RoleKepalaSekolah, 5), 5, 1, step1ID, DecisionRejected, "tidak disetujui")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusRejected {
		t.Fatalf("expected rejected, got %s", result.Status)
	}

	// step 2 tidak lagi bisa diputuskan (request sudah tidak pending).
	_, err = svc.DecideStep(ctxAs(RoleAdminSekolah, 6), 6, 1, step2ID, DecisionApproved, "")
	if !errors.Is(err, errRequestNotPending) {
		t.Fatalf("expected errRequestNotPending, got: %v", err)
	}
}

// -- Self-approval: step di-skip saat submit, chain kosong -> auto approved --

func TestSubmitRequest_SelfApprovalSkipAutoApproves(t *testing.T) {
	repo := newFakeRepo()
	repo.users[5] = "Kepala Sekolah"
	identity := newFakeIdentity()
	svc := newTestService(t, repo, identity)
	// chain default: [{role: kepala_sekolah}] (tanpa user_id spesifik) — kepsek
	// mengajukan izin sendiri, role sesi kepsek == role step -> step di-skip.
	ctx := ctxAs(RoleKepalaSekolah, 5)

	req := mustSubmit(t, svc, ctx, 5, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"})
	if len(req.Steps) != 0 {
		t.Fatalf("expected 0 steps (seluruh chain di-skip), got %d", len(req.Steps))
	}
	if req.Status != StatusApproved {
		t.Fatalf("expected auto approved, got %s", req.Status)
	}

	found := false
	for _, l := range identity.logs {
		if l.action == "leave.auto_approved_self_chain" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected audit log leave.auto_approved_self_chain")
	}
}

// -- Guard eksplisit: approver tidak boleh memutuskan request miliknya sendiri --

func TestDecideStep_ForbidsSelfApproval(t *testing.T) {
	repo := newFakeRepo()
	repo.users[300] = "Guru Merangkap"
	svc := newTestService(t, repo, newFakeIdentity())

	// Seed langsung (bypass filter self-skip submit) — mensimulasikan step
	// yang approver-nya adalah pengaju sendiri (mis. data anomali / multi-role).
	_, stepID := repo.seedRequest(1, 300, StatusPending, RoleKepalaSekolah, 300)

	_, err := svc.DecideStep(ctxAs(RoleKepalaSekolah, 300), 300, 1, stepID, DecisionApproved, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status := domainStatus(t, err); status != 403 {
		t.Fatalf("expected 403, got %d", status)
	}
}

// -- Cancel: hanya pending & hanya pemilik --

func TestCancelRequest(t *testing.T) {
	repo := newFakeRepo()
	repo.users[100] = "Guru Uji"
	svc := newTestService(t, repo, newFakeIdentity())
	ctx := ctxAs(RoleGuru, 100)

	req := mustSubmit(t, svc, ctx, 100, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"})

	t.Run("bukan pemilik ditolak", func(t *testing.T) {
		err := svc.CancelRequest(ctxAs(RoleGuru, 999), 999, 1, req.ID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status := domainStatus(t, err); status != 403 {
			t.Fatalf("expected 403, got %d", status)
		}
	})

	t.Run("pemilik berhasil", func(t *testing.T) {
		if err := svc.CancelRequest(ctx, 100, 1, req.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rec, err := repo.GetRequestByID(ctx, 1, req.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Status != StatusCanceled {
			t.Fatalf("expected canceled, got %s", rec.Status)
		}
	})

	t.Run("sudah tidak pending ditolak", func(t *testing.T) {
		err := svc.CancelRequest(ctx, 100, 1, req.ID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status := domainStatus(t, err); status != 422 {
			t.Fatalf("expected 422, got %d", status)
		}
	})
}

// -- ListApprovals: antrian hanya step aktif yang cocok --

func TestListApprovals(t *testing.T) {
	repo := newFakeRepo()
	repo.users[200] = "Guru Pengaju"
	repo.users[5] = "Kepala Sekolah"
	svc := newTestService(t, repo, newFakeIdentity())

	req := mustSubmit(t, svc, ctxAs(RoleGuru, 200), 200, SubmitInput{Type: "izin", DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "x"})

	items, err := svc.ListApprovals(ctxAs(RoleKepalaSekolah, 5), 1, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Request.ID != req.ID {
		t.Fatalf("expected 1 antrian untuk kepsek, got %+v", items)
	}

	// guru lain (bukan approver) tidak melihat apa-apa.
	items, err = svc.ListApprovals(ctxAs(RoleGuru, 201), 1, 201)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected antrian kosong, got %+v", items)
	}
}
