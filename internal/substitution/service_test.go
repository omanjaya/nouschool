package substitution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fake repository (in-memory, tanpa DB) --

type fakeRepo struct {
	rows      map[int64]Row
	nextID    int64
	slots     map[int64]SlotBasic
	teacherUs map[int64]int64 // teacherID -> userID
	activeGrs map[int64]bool  // userID -> guru aktif
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		rows: map[int64]Row{}, slots: map[int64]SlotBasic{},
		teacherUs: map[int64]int64{}, activeGrs: map[int64]bool{},
	}
}

var _ substitutionRepository = (*fakeRepo)(nil)

func (f *fakeRepo) Create(ctx context.Context, in CreateInput) (Record, error) {
	// tegakkan unique aktif (pending/accepted) per (slot,date) — pola sama migrasi.
	for _, r := range f.rows {
		if r.ScheduleSlotID == in.ScheduleSlotID && sameDate(r.Date, in.Date) && (r.Status == StatusPending || r.Status == StatusAccepted) {
			return Record{}, ErrConflict
		}
	}
	f.nextID++
	rec := Record{
		ID: f.nextID, SchoolID: in.SchoolID, ScheduleSlotID: in.ScheduleSlotID, Date: in.Date,
		RequestedBy: in.RequestedBy, SubstituteUserID: in.SubstituteUserID, Reason: in.Reason, Status: StatusPending,
	}
	f.rows[rec.ID] = Row{
		Record: rec, ClassName: "XII RPL 1", SubjectName: "Basis Data", DayOfWeek: int(in.Date.Weekday()),
		PeriodStart: 1, PeriodEnd: 2, RequestedByName: "Pengaju", SubstituteName: "Pengganti",
	}
	return rec, nil
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func (f *fakeRepo) GetByID(ctx context.Context, schoolID, id int64) (Record, error) {
	r, ok := f.rows[id]
	if !ok || r.SchoolID != schoolID {
		return Record{}, ErrNotFound
	}
	return r.Record, nil
}

func (f *fakeRepo) GetRow(ctx context.Context, schoolID, id int64) (Row, error) {
	r, ok := f.rows[id]
	if !ok || r.SchoolID != schoolID {
		return Row{}, ErrNotFound
	}
	return r, nil
}

func (f *fakeRepo) ListRows(ctx context.Context, schoolID int64, requestedBy, substituteUserID int64, date string) ([]Row, error) {
	var out []Row
	for _, r := range f.rows {
		if r.SchoolID != schoolID {
			continue
		}
		if requestedBy != 0 && r.RequestedBy != requestedBy {
			continue
		}
		if substituteUserID != 0 && r.SubstituteUserID != substituteUserID {
			continue
		}
		if date != "" && r.Date.Format("2006-01-02") != date {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRepo) Transition(ctx context.Context, schoolID, id int64, from, to string, decidedAt time.Time) (Record, error) {
	r, ok := f.rows[id]
	if !ok || r.SchoolID != schoolID || r.Status != from {
		return Record{}, ErrStateChanged
	}
	r.Status = to
	dt := decidedAt
	r.DecidedAt = &dt
	f.rows[id] = r
	return r.Record, nil
}

func (f *fakeRepo) GetSlotBasic(ctx context.Context, schoolID, slotID int64) (SlotBasic, error) {
	s, ok := f.slots[slotID]
	if !ok {
		return SlotBasic{}, ErrNotFound
	}
	return s, nil
}

func (f *fakeRepo) GetTeacherUserID(ctx context.Context, schoolID, teacherID int64) (int64, error) {
	id, ok := f.teacherUs[teacherID]
	if !ok {
		return 0, ErrNotFound
	}
	return id, nil
}

func (f *fakeRepo) IsActiveGuru(ctx context.Context, schoolID, userID int64) (bool, error) {
	return f.activeGrs[userID], nil
}

func (f *fakeRepo) ActiveSubstituteForSlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (int64, string, bool, error) {
	for _, r := range f.rows {
		if r.ScheduleSlotID == slotID && sameDate(r.Date, date) && r.Status == StatusAccepted {
			return r.SubstituteUserID, r.SubstituteName, true, nil
		}
	}
	return 0, "", false, nil
}

func (f *fakeRepo) SlotIDsAcceptedSubstituteForDate(ctx context.Context, schoolID, substituteUserID int64, date time.Time) ([]int64, error) {
	var out []int64
	for _, r := range f.rows {
		if r.SubstituteUserID == substituteUserID && sameDate(r.Date, date) && r.Status == StatusAccepted {
			out = append(out, r.ScheduleSlotID)
		}
	}
	return out, nil
}

type fakeIdentity struct{}

func (fakeIdentity) HasPermission(role, perm string) bool { return role == RoleAdminSekolah }
func (fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

func newTestService() (*Service, *fakeRepo) {
	repo := newFakeRepo()
	return newServiceForTest(repo, fakeIdentity{}, clock.Fixed{T: time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC)}), repo
}

func ctxAs(userID int64, role string) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Demo", Slug: "demo", Timezone: "Asia/Jakarta", Status: "active"})
	return ctx
}

func domainStatus(t *testing.T, err error) int {
	t.Helper()
	var de *httpx.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain error, got: %v", err)
	}
	return de.Status
}

// slotDate — Sabtu 2026-08-15 -> Minggu berikutnya 2026-08-16 (day_of_week 0).
const tomorrowSunday = "2026-08-16" // day_of_week 0 (Minggu)

func setupSlot(repo *fakeRepo, slotID, teacherID, ownerUserID int64, dayOfWeek int) {
	repo.slots[slotID] = SlotBasic{ID: slotID, TeacherID: teacherID, DayOfWeek: dayOfWeek}
	repo.teacherUs[teacherID] = ownerUserID
}

func TestRequest_OwnerAllowed(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0) // Minggu
	repo.activeGrs[20] = true

	ctx := ctxAs(10, "guru")
	view, err := svc.Request(ctx, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20, Reason: "sakit"})
	if err != nil {
		t.Fatalf("pemilik slot harus boleh mengajukan: %v", err)
	}
	if view.Status != StatusPending {
		t.Fatalf("expected status pending, got %s", view.Status)
	}
}

func TestRequest_NonOwnerForbidden(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0)
	repo.activeGrs[20] = true

	ctx := ctxAs(11, "guru") // BUKAN pemilik slot (owner=10)
	_, err := svc.Request(ctx, 11, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20})
	if domainStatus(t, err) != 403 {
		t.Fatalf("guru bukan pemilik slot harus 403, dapat: %v", err)
	}
}

func TestRequest_PastDateRejected(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0)
	repo.activeGrs[20] = true

	ctx := ctxAs(10, "guru")
	_, err := svc.Request(ctx, 10, 1, RequestInput{ScheduleSlotID: 50, Date: "2026-08-01", SubstituteUserID: 20})
	if err == nil {
		t.Fatal("tanggal lampau harus ditolak")
	}
}

func TestRequest_WrongWeekdayRejected(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 1) // slot hari Senin (1)
	repo.activeGrs[20] = true

	ctx := ctxAs(10, "guru")
	_, err := svc.Request(ctx, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20}) // tomorrowSunday = Minggu (0)
	if err == nil {
		t.Fatal("tanggal yang tidak jatuh pada hari slot harus ditolak")
	}
}

func TestRequest_SubstituteMustBeActiveGuru(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0)
	// repo.activeGrs[20] TIDAK diset -> bukan guru aktif

	ctx := ctxAs(10, "guru")
	_, err := svc.Request(ctx, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20})
	if err == nil {
		t.Fatal("substitute bukan guru aktif harus ditolak")
	}
}

func TestRequest_DuplicateActiveConflict(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0)
	repo.activeGrs[20] = true
	repo.activeGrs[21] = true

	ctx := ctxAs(10, "guru")
	if _, err := svc.Request(ctx, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20}); err != nil {
		t.Fatalf("request pertama harus sukses: %v", err)
	}
	_, err := svc.Request(ctx, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 21})
	if domainStatus(t, err) != 409 {
		t.Fatalf("request kedua utk slot+tanggal yang sama (masih pending) harus 409, dapat: %v", err)
	}
}

func TestAcceptRejectCancel_StateMachine(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0)
	repo.activeGrs[20] = true

	owner := ctxAs(10, "guru")
	view, err := svc.Request(owner, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20})
	if err != nil {
		t.Fatalf("gagal request: %v", err)
	}

	// Bukan pengganti yang diminta -> 403.
	other := ctxAs(99, "guru")
	if _, err := svc.Accept(other, 99, 1, view.ID); domainStatus(t, err) != 403 {
		t.Fatalf("bukan pengganti yang diminta harus 403 saat accept: %v", err)
	}

	// Pengganti yang diminta -> boleh accept.
	sub := ctxAs(20, "guru")
	accepted, err := svc.Accept(sub, 20, 1, view.ID)
	if err != nil {
		t.Fatalf("pengganti diminta harus boleh accept: %v", err)
	}
	if accepted.Status != StatusAccepted {
		t.Fatalf("expected status accepted, got %s", accepted.Status)
	}

	// Sudah accepted -> accept lagi ditolak (bukan pending lagi).
	if _, err := svc.Accept(sub, 20, 1, view.ID); err == nil {
		t.Fatal("accept kedua kali pada request yang sudah accepted harus ditolak")
	}

	// SubstituteName sekarang mengembalikan ok=true.
	if name, ok, err := svc.SubstituteName(context.Background(), 1, 50, tomorrowSunday); err != nil || !ok || name != "Pengganti" {
		t.Fatalf("expected SubstituteName ok, got name=%q ok=%v err=%v", name, ok, err)
	}
	// IsSubstituteToday mengembalikan slot 50 utk user 20.
	slotIDs, err := svc.IsSubstituteToday(context.Background(), 1, 20, tomorrowSunday)
	if err != nil || len(slotIDs) != 1 || slotIDs[0] != 50 {
		t.Fatalf("expected IsSubstituteToday=[50], got %v err=%v", slotIDs, err)
	}
}

func TestReject_ByRequester_Forbidden(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0)
	repo.activeGrs[20] = true
	owner := ctxAs(10, "guru")
	view, err := svc.Request(owner, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20})
	if err != nil {
		t.Fatalf("gagal request: %v", err)
	}
	// Pengaju TIDAK boleh menolak permintaannya sendiri (hanya pengganti).
	if _, err := svc.Reject(owner, 10, 1, view.ID); domainStatus(t, err) != 403 {
		t.Fatalf("pengaju mencoba reject harus 403: %v", err)
	}
}

func TestCancel_OnlyRequesterWhilePending(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0)
	repo.activeGrs[20] = true
	owner := ctxAs(10, "guru")
	view, err := svc.Request(owner, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20})
	if err != nil {
		t.Fatalf("gagal request: %v", err)
	}

	other := ctxAs(99, "guru")
	if _, err := svc.Cancel(other, 99, 1, view.ID); domainStatus(t, err) != 403 {
		t.Fatalf("bukan pengaju harus 403 saat cancel: %v", err)
	}

	canceled, err := svc.Cancel(owner, 10, 1, view.ID)
	if err != nil {
		t.Fatalf("pengaju harus boleh cancel selama pending: %v", err)
	}
	if canceled.Status != StatusCanceled {
		t.Fatalf("expected status canceled, got %s", canceled.Status)
	}

	// Setelah accepted, cancel oleh pengaju TIDAK boleh lagi (bukan pending).
	view2, err := svc.Request(owner, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20})
	if err != nil {
		t.Fatalf("gagal request kedua: %v", err)
	}
	sub := ctxAs(20, "guru")
	if _, err := svc.Accept(sub, 20, 1, view2.ID); err != nil {
		t.Fatalf("gagal accept: %v", err)
	}
	if _, err := svc.Cancel(owner, 10, 1, view2.ID); err == nil {
		t.Fatal("cancel setelah accepted (bukan pending lagi) harus ditolak")
	}
}

func TestList_ScopeAll_RequiresScheduleManage(t *testing.T) {
	svc, repo := newTestService()
	setupSlot(repo, 50, 300, 10, 0)
	repo.activeGrs[20] = true
	owner := ctxAs(10, "guru")
	if _, err := svc.Request(owner, 10, 1, RequestInput{ScheduleSlotID: 50, Date: tomorrowSunday, SubstituteUserID: 20}); err != nil {
		t.Fatalf("gagal request: %v", err)
	}

	guru := ctxAs(10, "guru")
	if _, err := svc.List(guru, 10, 1, ListQuery{Scope: "all"}); domainStatus(t, err) != 403 {
		t.Fatalf("guru TANPA schedule:manage harus 403 utk scope=all: %v", err)
	}

	admin := ctxAs(1, "admin_sekolah")
	items, err := svc.List(admin, 1, 1, ListQuery{Scope: "all"})
	if err != nil || len(items) != 1 {
		t.Fatalf("admin_sekolah harus boleh scope=all: items=%v err=%v", items, err)
	}
}
