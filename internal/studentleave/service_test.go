package studentleave

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
	requests    map[int64]RequestDetail
	nextID      int64
	letterSeq   map[[2]string]int32 // key: [schoolID, year]
	students    map[int64]struct{ name, nis string }
	homeroom    map[[3]int64]int64 // [schoolID, yearID, studentID] -> teacher user_id
	verifyIndex map[string]int64   // token -> request id
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		requests:    map[int64]RequestDetail{},
		letterSeq:   map[[2]string]int32{},
		students:    map[int64]struct{ name, nis string }{},
		homeroom:    map[[3]int64]int64{},
		verifyIndex: map[string]int64{},
	}
}

var _ studentLeaveRepository = (*fakeRepo)(nil)

func (f *fakeRepo) CreateRequest(ctx context.Context, in CreateRequestInput) (int64, error) {
	f.nextID++
	st, _ := f.students[in.StudentID]
	f.requests[f.nextID] = RequestDetail{
		ID: f.nextID, SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, StudentID: in.StudentID,
		SubmittedBy: in.SubmittedBy, Type: in.Type, DateStart: in.DateStart, DateEnd: in.DateEnd, Reason: in.Reason,
		Attachment: in.Attachment, AttachmentName: in.AttachmentName, AttachmentMime: in.AttachmentMime,
		Status: StatusPendingHomeroom, CreatedAt: time.Now(),
		StudentName: st.name, StudentNIS: st.nis, SubmittedByName: "Pengaju",
	}
	return f.nextID, nil
}

func (f *fakeRepo) GetRequestDetail(ctx context.Context, schoolID, id int64) (RequestDetail, error) {
	rec, ok := f.requests[id]
	if !ok || rec.SchoolID != schoolID {
		return RequestDetail{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) ListRequestsForStudents(ctx context.Context, schoolID int64, studentIDs []int64, status string) ([]RequestDetail, error) {
	set := map[int64]bool{}
	for _, id := range studentIDs {
		set[id] = true
	}
	var out []RequestDetail
	for _, r := range f.requests {
		if r.SchoolID == schoolID && set[r.StudentID] && (status == "" || r.Status == status) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListRequestsByStatus(ctx context.Context, schoolID int64, status string) ([]RequestDetail, error) {
	var out []RequestDetail
	for _, r := range f.requests {
		if r.SchoolID == schoolID && (status == "" || r.Status == status) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListRequestsForHomeroomTeacher(ctx context.Context, schoolID, teacherUserID int64, status string) ([]RequestDetail, error) {
	var out []RequestDetail
	for _, r := range f.requests {
		if r.SchoolID != schoolID || r.Status != status {
			continue
		}
		if uid, ok := f.homeroom[[3]int64{schoolID, r.AcademicYearID, r.StudentID}]; ok && uid == teacherUserID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateHomeroomDecision(ctx context.Context, schoolID, id int64, newStatus string, decidedBy int64, comment string) (int64, error) {
	rec, ok := f.requests[id]
	if !ok || rec.SchoolID != schoolID || rec.Status != StatusPendingHomeroom {
		return 0, nil
	}
	rec.Status = newStatus
	rec.HomeroomDecidedBy = decidedBy
	rec.HomeroomDecidedAt = time.Now()
	rec.HomeroomComment = comment
	rec.HomeroomDecidedByName = "Wali"
	f.requests[id] = rec
	return 1, nil
}

func (f *fakeRepo) UpdateBKDecision(ctx context.Context, schoolID, id int64, newStatus string, decidedBy int64, comment, letterNumber, verifyToken string) (int64, error) {
	rec, ok := f.requests[id]
	if !ok || rec.SchoolID != schoolID || rec.Status != StatusPendingBK {
		return 0, nil
	}
	rec.Status = newStatus
	rec.BKDecidedBy = decidedBy
	rec.BKDecidedAt = time.Now()
	rec.BKComment = comment
	rec.BKDecidedByName = "BK"
	rec.LetterNumber = letterNumber
	rec.VerifyToken = verifyToken
	f.requests[id] = rec
	if verifyToken != "" {
		f.verifyIndex[verifyToken] = id
	}
	return 1, nil
}

func (f *fakeRepo) CancelRequest(ctx context.Context, schoolID, id, submittedBy int64) (int64, error) {
	rec, ok := f.requests[id]
	if !ok || rec.SchoolID != schoolID || rec.SubmittedBy != submittedBy || !isPending(rec.Status) {
		return 0, nil
	}
	rec.Status = StatusCanceled
	f.requests[id] = rec
	return 1, nil
}

func (f *fakeRepo) NextLetterSeq(ctx context.Context, schoolID int64, year string) (int32, error) {
	key := [2]string{itoa(schoolID), year}
	f.letterSeq[key]++
	return f.letterSeq[key], nil
}

func (f *fakeRepo) GetByVerifyToken(ctx context.Context, schoolID int64, token string) (VerifyRecord, bool, error) {
	id, ok := f.verifyIndex[token]
	if !ok {
		return VerifyRecord{}, false, nil
	}
	rec, ok := f.requests[id]
	if !ok || rec.SchoolID != schoolID || rec.Status != StatusIssued {
		return VerifyRecord{}, false, nil
	}
	return VerifyRecord{
		LetterNumber: rec.LetterNumber, Type: rec.Type, DateStart: rec.DateStart, DateEnd: rec.DateEnd,
		IssuedAt: rec.BKDecidedAt, StudentName: rec.StudentName, ClassName: rec.ClassName,
	}, true, nil
}

func (f *fakeRepo) StudentBasic(ctx context.Context, schoolID, studentID int64) (string, string, bool, error) {
	st, ok := f.students[studentID]
	if !ok {
		return "", "", false, nil
	}
	return st.name, st.nis, true, nil
}

func (f *fakeRepo) HomeroomTeacherUserID(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, bool, error) {
	id, ok := f.homeroom[[3]int64{schoolID, academicYearID, studentID}]
	return id, ok, nil
}

func seedStudent(f *fakeRepo, id int64, name, nis string) {
	f.students[id] = struct{ name, nis string }{name, nis}
}

// -- fake consumer-side dependencies --

type fakeIdentity struct{}

func (fakeIdentity) HasPermission(role, perm string) bool {
	return role == RoleAdminSekolah && perm == PermManageAll
}
func (fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

type fakeYears struct{ id int64 }

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	return f.id, true, nil
}

type fakeBranding struct{}

func (fakeBranding) BrandingAppName(ctx context.Context, schoolID int64) (string, error) {
	return "Sekolah Uji", nil
}

type fakeStudentAccess struct {
	guardianOf    map[int64][]int64 // studentID -> []guardianUserID
	studentUserOf map[int64]int64   // studentID -> userID (siswa login)
	childrenOf    map[int64][]int64 // guardianUserID -> []studentID
}

func (f fakeStudentAccess) CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error {
	switch role {
	case RoleSiswa:
		if f.studentUserOf[studentID] == userID {
			return nil
		}
	case RoleOrangTua:
		for _, g := range f.guardianOf[studentID] {
			if g == userID {
				return nil
			}
		}
	}
	return httpx.ErrForbidden
}

func (f fakeStudentAccess) GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error) {
	return f.guardianOf[studentID], nil
}

func (f fakeStudentAccess) MyStudentID(ctx context.Context, schoolID, userID int64) (int64, int64, bool, error) {
	for sid, uid := range f.studentUserOf {
		if uid == userID {
			return sid, 0, true, nil
		}
	}
	return 0, 0, false, nil
}

func (f fakeStudentAccess) MyChildStudentIDs(ctx context.Context, schoolID, userID int64) ([]int64, error) {
	return f.childrenOf[userID], nil
}

type fakeDuty struct {
	flagsByUser map[int64][]string // userID -> flags dipegang (TA aktif)
}

func (f fakeDuty) UserHasFlag(ctx context.Context, schoolID, userID int64, flag string) (bool, error) {
	for _, fl := range f.flagsByUser[userID] {
		if fl == flag {
			return true, nil
		}
	}
	return false, nil
}

func (f fakeDuty) UserIDsWithFlag(ctx context.Context, schoolID int64, flag string) ([]int64, error) {
	var out []int64
	for uid, flags := range f.flagsByUser {
		for _, fl := range flags {
			if fl == flag {
				out = append(out, uid)
				break
			}
		}
	}
	return out, nil
}

// -- fake SettingsGateway (Fase 14 Gelombang D: footer surat, school_settings
// module "letters") --

type fakeSettingsGateway struct {
	raw   map[string][]byte
	found map[string]bool
	err   error
}

func (f fakeSettingsGateway) GetSetting(ctx context.Context, schoolID int64, module string) ([]byte, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	return f.raw[module], f.found[module], nil
}

func TestLeaveFooterNote_RenderedWhenSet(t *testing.T) {
	svc := newTestService(newFakeRepo(), fakeStudentAccess{}, fakeDuty{})
	svc.SetSettingsGateway(fakeSettingsGateway{
		raw:   map[string][]byte{"letters": []byte(`{"sp_footer_note":"","leave_footer_note":"Hubungi TU bila ada pertanyaan."}`)},
		found: map[string]bool{"letters": true},
	})
	ctx := ctxAs("admin_sekolah", 1)
	if got := svc.leaveFooterNote(ctx, testSchoolID); got != "Hubungi TU bila ada pertanyaan." {
		t.Fatalf("expected catatan terbaca dari settings, got %q", got)
	}
}

func TestLeaveFooterNote_EmptyWhenGatewayNilOrNotFound(t *testing.T) {
	svc := newTestService(newFakeRepo(), fakeStudentAccess{}, fakeDuty{}) // gateway TIDAK di-set (nil)
	ctx := ctxAs("admin_sekolah", 1)
	if got := svc.leaveFooterNote(ctx, testSchoolID); got != "" {
		t.Fatalf("gateway nil harus menghasilkan string kosong, got %q", got)
	}

	svc.SetSettingsGateway(fakeSettingsGateway{found: map[string]bool{}})
	if got := svc.leaveFooterNote(ctx, testSchoolID); got != "" {
		t.Fatalf("module belum pernah tersimpan harus menghasilkan string kosong, got %q", got)
	}
}

func TestBuildLeavePDF_FooterNoteIncreasesOutputWhenSet(t *testing.T) {
	detail := RequestDetail{
		LetterNumber: "IZ/2026/0001", StudentName: "Budi", StudentNIS: "22103", ClassName: "XII RPL 1",
		Type: TypeSakit, DateStart: time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC), DateEnd: time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC),
		Reason: "demam", VerifyToken: "tok123",
	}
	without, err := BuildLeavePDF(detail, "NouSchool", "https://demo.localhost/verifikasi-surat?token=tok123", "")
	if err != nil {
		t.Fatalf("unexpected error tanpa footer: %v", err)
	}
	if len(without) < 4 || string(without[:4]) != "%PDF" {
		t.Fatal("output harus PDF valid (header %PDF)")
	}
	with, err := BuildLeavePDF(detail, "NouSchool", "https://demo.localhost/verifikasi-surat?token=tok123", "Catatan tambahan dari sekolah.")
	if err != nil {
		t.Fatalf("unexpected error dengan footer: %v", err)
	}
	if len(with) < 4 || string(with[:4]) != "%PDF" {
		t.Fatal("output DENGAN footer harus tetap PDF valid")
	}
	if len(with) <= len(without) {
		t.Fatalf("PDF dengan footer harus lebih besar dari tanpa footer (footer benar-benar dirender): %d vs %d", len(with), len(without))
	}
}

// -- helpers --

const testSchoolID = 1
const testYearID = 1

func ctxAs(role string, userID int64) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: testSchoolID, Name: "Sekolah Uji", Slug: "uji", Timezone: "Asia/Jakarta"})
	return ctx
}

func newTestService(repo *fakeRepo, students fakeStudentAccess, duties fakeDuty) *Service {
	return newServiceForTest(repo, fakeIdentity{}, fakeYears{id: testYearID}, fakeBranding{}, students, duties,
		clock.Fixed{T: time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)})
}

func submitAsStudent(t *testing.T, svc *Service, studentUserID int64) RequestView {
	t.Helper()
	v, err := svc.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, testSchoolID, SubmitInput{
		Type: TypeSakit, DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "demam",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	return v
}

// -- tests --

func TestStateMachine_HappyPath(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 10, "Budi", "22101")
	students := fakeStudentAccess{studentUserOf: map[int64]int64{10: 100}, guardianOf: map[int64][]int64{}}
	repo.homeroom[[3]int64{testSchoolID, testYearID, 10}] = 200 // wali kelas user_id 200
	duties := fakeDuty{flagsByUser: map[int64][]string{300: {FlagLeaveIssuance}}}
	svc := newTestService(repo, students, duties)

	v := submitAsStudent(t, svc, 100)
	if v.Status != StatusPendingHomeroom {
		t.Fatalf("status awal harus pending_homeroom, dapat %s", v.Status)
	}

	v, err := svc.ReviewHomeroom(ctxAs(RoleGuru, 200), 200, testSchoolID, v.ID, DecisionApprove, "oke")
	if err != nil {
		t.Fatalf("review homeroom: %v", err)
	}
	if v.Status != StatusPendingBK {
		t.Fatalf("setelah approve wali kelas harus pending_bk, dapat %s", v.Status)
	}

	v, err = svc.IssueBK(ctxAs(RoleGuru, 300), 300, testSchoolID, v.ID, DecisionApprove, "terbit")
	if err != nil {
		t.Fatalf("issue bk: %v", err)
	}
	if v.Status != StatusIssued {
		t.Fatalf("setelah approve BK harus issued, dapat %s", v.Status)
	}
	if v.LetterNumber == nil || *v.LetterNumber == "" {
		t.Fatal("letter_number harus terisi setelah issued")
	}
	if v.VerifyToken == nil || len(*v.VerifyToken) != verifyTokenLength {
		t.Fatalf("verify_token harus terisi %d karakter, dapat %v", verifyTokenLength, v.VerifyToken)
	}
}

func TestStateMachine_RejectStopsChain(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 11, "Sari", "22102")
	students := fakeStudentAccess{studentUserOf: map[int64]int64{11: 101}, guardianOf: map[int64][]int64{}}
	repo.homeroom[[3]int64{testSchoolID, testYearID, 11}] = 201
	duties := fakeDuty{}
	svc := newTestService(repo, students, duties)

	v := submitAsStudent(t, svc, 101)
	v, err := svc.ReviewHomeroom(ctxAs(RoleGuru, 201), 201, testSchoolID, v.ID, DecisionReject, "tidak lengkap")
	if err != nil {
		t.Fatalf("reject homeroom: %v", err)
	}
	if v.Status != StatusRejected {
		t.Fatalf("mau status rejected, dapat %s", v.Status)
	}

	// Tidak bisa lanjut ke tahap BK setelah rejected.
	_, err = svc.IssueBK(ctxAs(RoleGuru, 300), 300, testSchoolID, v.ID, DecisionApprove, "")
	if err == nil {
		t.Fatal("issue BK atas request yang sudah rejected harus ditolak")
	}
}

func TestCancel_OnlyPendingAndOwner(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 12, "Dedi", "22103")
	students := fakeStudentAccess{studentUserOf: map[int64]int64{12: 102}, guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students, fakeDuty{})

	v := submitAsStudent(t, svc, 102)

	// Bukan pemilik -> forbidden.
	if err := svc.CancelRequest(ctxAs(RoleSiswa, 999), 999, testSchoolID, v.ID); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("cancel oleh bukan pemilik harus 403, dapat %v", err)
	}

	// Pemilik, masih pending -> sukses.
	if err := svc.CancelRequest(ctxAs(RoleSiswa, 102), 102, testSchoolID, v.ID); err != nil {
		t.Fatalf("cancel pemilik saat pending harus sukses: %v", err)
	}

	// Sudah canceled -> tidak bisa cancel lagi.
	if err := svc.CancelRequest(ctxAs(RoleSiswa, 102), 102, testSchoolID, v.ID); err == nil {
		t.Fatal("cancel request yang sudah canceled harus ditolak")
	}
}

func TestAuthorizeReviewer_OtherClassHomeroom403(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 13, "Eka", "22104")
	students := fakeStudentAccess{studentUserOf: map[int64]int64{13: 103}, guardianOf: map[int64][]int64{}}
	repo.homeroom[[3]int64{testSchoolID, testYearID, 13}] = 201 // wali kelas SISWA INI = user 201
	svc := newTestService(repo, students, fakeDuty{})

	v := submitAsStudent(t, svc, 103)

	// User 999 BUKAN wali kelas rombel siswa ini DAN tidak punya flag -> 403.
	_, err := svc.ReviewHomeroom(ctxAs(RoleGuru, 999), 999, testSchoolID, v.ID, DecisionApprove, "")
	if !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("wali kelas rombel LAIN harus 403, dapat %v", err)
	}
}

func TestAuthorizeReviewer_FlagFallbackOK(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 14, "Fajar", "22105")
	students := fakeStudentAccess{studentUserOf: map[int64]int64{14: 104}, guardianOf: map[int64][]int64{}}
	// TIDAK ada wali kelas terdaftar utk siswa ini (kelas belum punya wali).
	duties := fakeDuty{flagsByUser: map[int64][]string{500: {FlagLeaveHomeroomReview}}}
	svc := newTestService(repo, students, duties)

	v := submitAsStudent(t, svc, 104)

	// User 500 BUKAN wali kelas manapun, TAPI pemegang flag -> boleh review.
	v, err := svc.ReviewHomeroom(ctxAs(RoleGuru, 500), 500, testSchoolID, v.ID, DecisionApprove, "")
	if err != nil {
		t.Fatalf("pemegang flag leave_homeroom_review harus boleh review (fallback): %v", err)
	}
	if v.Status != StatusPendingBK {
		t.Fatalf("mau pending_bk, dapat %s", v.Status)
	}
}

func TestLetterNumberAndToken_Unique(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 20, "Andi", "22110")
	seedStudent(repo, 21, "Beni", "22111")
	students := fakeStudentAccess{studentUserOf: map[int64]int64{20: 110, 21: 111}, guardianOf: map[int64][]int64{}}
	repo.homeroom[[3]int64{testSchoolID, testYearID, 20}] = 200
	repo.homeroom[[3]int64{testSchoolID, testYearID, 21}] = 200
	duties := fakeDuty{flagsByUser: map[int64][]string{300: {FlagLeaveIssuance}}}
	svc := newTestService(repo, students, duties)

	v1 := submitAsStudent(t, svc, 110)
	v1, _ = svc.ReviewHomeroom(ctxAs(RoleGuru, 200), 200, testSchoolID, v1.ID, DecisionApprove, "")
	v1, err := svc.IssueBK(ctxAs(RoleGuru, 300), 300, testSchoolID, v1.ID, DecisionApprove, "")
	if err != nil {
		t.Fatalf("issue 1: %v", err)
	}

	v2, err := svc.SubmitRequest(ctxAs(RoleSiswa, 111), 111, testSchoolID, SubmitInput{Type: TypeIzin, DateStart: "2026-08-18", DateEnd: "2026-08-18", Reason: "acara keluarga"})
	if err != nil {
		t.Fatalf("submit 2: %v", err)
	}
	v2, _ = svc.ReviewHomeroom(ctxAs(RoleGuru, 200), 200, testSchoolID, v2.ID, DecisionApprove, "")
	v2, err = svc.IssueBK(ctxAs(RoleGuru, 300), 300, testSchoolID, v2.ID, DecisionApprove, "")
	if err != nil {
		t.Fatalf("issue 2: %v", err)
	}

	if *v1.LetterNumber == *v2.LetterNumber {
		t.Fatalf("nomor surat harus unik, keduanya %q", *v1.LetterNumber)
	}
	if *v1.VerifyToken == *v2.VerifyToken {
		t.Fatal("verify_token harus unik")
	}
}

func TestGuardianSubmit_OwnChildOK_OtherChild403(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 30, "Gita", "22120")
	seedStudent(repo, 31, "Hana", "22121")
	students := fakeStudentAccess{
		guardianOf: map[int64][]int64{30: {900}}, // user 900 = ortu siswa 30 SAJA
	}
	svc := newTestService(repo, students, fakeDuty{})

	// Ortu (900) ajukan utk anaknya sendiri (30) -> OK.
	_, err := svc.SubmitRequest(ctxAs(RoleOrangTua, 900), 900, testSchoolID, SubmitInput{
		StudentID: 30, Type: TypeSakit, DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "demam",
	})
	if err != nil {
		t.Fatalf("ortu ajukan utk anak sendiri harus sukses: %v", err)
	}

	// Ortu yang SAMA coba ajukan utk siswa 31 (BUKAN anaknya) -> 403.
	_, err = svc.SubmitRequest(ctxAs(RoleOrangTua, 900), 900, testSchoolID, SubmitInput{
		StudentID: 31, Type: TypeSakit, DateStart: "2026-08-17", DateEnd: "2026-08-17", Reason: "demam",
	})
	if !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("ortu ajukan utk anak orang lain harus 403, dapat %v", err)
	}
}

func TestPublicVerify_Shape(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 40, "Ical", "22130")
	students := fakeStudentAccess{studentUserOf: map[int64]int64{40: 120}, guardianOf: map[int64][]int64{}}
	repo.homeroom[[3]int64{testSchoolID, testYearID, 40}] = 200
	duties := fakeDuty{flagsByUser: map[int64][]string{300: {FlagLeaveIssuance}}}
	svc := newTestService(repo, students, duties)

	v := submitAsStudent(t, svc, 120)
	v, _ = svc.ReviewHomeroom(ctxAs(RoleGuru, 200), 200, testSchoolID, v.ID, DecisionApprove, "")
	v, err := svc.IssueBK(ctxAs(RoleGuru, 300), 300, testSchoolID, v.ID, DecisionApprove, "")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	got, err := svc.PublicVerify(context.Background(), testSchoolID, *v.VerifyToken)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Valid || got.LetterNumber != *v.LetterNumber || got.StudentName != "Ical" {
		t.Fatalf("verify token valid harus mengembalikan shape lengkap, dapat %+v", got)
	}

	bad, err := svc.PublicVerify(context.Background(), testSchoolID, "token-salah-sekali")
	if err != nil {
		t.Fatal(err)
	}
	if bad.Valid {
		t.Fatal("token salah harus valid:false")
	}
	if bad.LetterNumber != "" || bad.StudentName != "" {
		t.Fatalf("token salah TIDAK boleh membocorkan detail apa pun, dapat %+v", bad)
	}
}
