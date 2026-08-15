package student

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interfaces (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject dari main.go). Semua
// signature sengaja hanya memakai tipe primitif supaya *identity.Service dan
// *tenant.Service memenuhi interface ini secara struktural TANPA student
// perlu mengimpor package identity/tenant.

// IdentityGateway adalah kebutuhan modul student dari modul identity: buat
// akun (guru & aktivasi undangan), membership, sesi (auto-login aktivasi),
// kode undangan, cek permission, dan audit log.
type IdentityGateway interface {
	HashPassword(password string) (string, error)
	HasPermission(role, perm string) bool
	CreateAccount(ctx context.Context, email, username, passwordHash, name string) (userID int64, err error)
	UserIDByEmail(ctx context.Context, email string) (userID int64, ok bool, err error)
	UserIDByUsername(ctx context.Context, username string) (userID int64, ok bool, err error)
	CreateMembership(ctx context.Context, userID, schoolID int64, role string) error
	IssueSession(ctx context.Context, userID, schoolID int64, role, ip, userAgent string) (token string, expiresAt time.Time, err error)
	SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time)
	GenerateInvitationCode(ctx context.Context, schoolID int64, role string, targetID int64, ttl time.Duration) (code string, expiresAt time.Time, err error)
	ValidateInvitationCode(ctx context.Context, code string) (schoolID int64, role string, targetID int64, ok bool, err error)
	MarkInvitationUsed(ctx context.Context, code string) error
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// AcademicYearLookup adalah kebutuhan modul student dari modul tenant.
type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// Permission kanonik dipakai modul student (nilai HARUS sama persis dengan
// docs/02-identity.md — didefinisikan ulang di sini karena student tidak
// boleh mengimpor identity).
const (
	PermStudentManage = "student:manage"
	PermStudentRead   = "student:read"
	PermScheduleRead  = "schedule:read"
)

const invitationTTL = 30 * 24 * time.Hour

// ErrNoActiveAcademicYear — sekolah belum mengaktifkan tahun ajaran, operasi
// yang butuh konteks tahun ajaran (buat rombel, import) tidak bisa jalan.
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// ErrInvitationInvalid — kode undangan tidak ditemukan/sudah dipakai/kedaluwarsa (GET info).
var ErrInvitationInvalid = &httpx.Error{Status: http.StatusNotFound, Code: "invitation_not_found", Message: "Kode tidak ditemukan atau sudah dipakai."}

// ErrInvitationUsed — kode undangan tidak valid saat AKTIVASI (dibedakan dari
// ErrInvitationInvalid supaya status code sesuai spesifikasi: 410 Gone).
var ErrInvitationUsed = &httpx.Error{Status: http.StatusGone, Code: "invitation_used", Message: "Kode undangan sudah dipakai atau kedaluwarsa."}

// Service berisi aturan bisnis modul student.
type Service struct {
	repo        studentRepository
	identity    IdentityGateway
	years       AcademicYearLookup
	importStore *ImportStore
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup) *Service {
	return &Service{repo: repo, identity: identity, years: years, importStore: NewImportStore()}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go, import_test.go).
func newServiceForTest(repo studentRepository, identity IdentityGateway, years AcademicYearLookup) *Service {
	return &Service{repo: repo, identity: identity, years: years, importStore: NewImportStore()}
}

// audit mencatat satu baris ke audit_log lewat IdentityGateway (best-effort —
// kegagalan audit tidak membatalkan operasi utama, sama seperti pola di
// internal/tenant).
func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

// requireActiveYear mengembalikan id tahun ajaran aktif, atau error validasi
// bila belum ada — dipakai operasi yang WAJIB tahu tahun ajaran (buat rombel
// tanpa academic_year_id eksplisit, import siswa).
func (s *Service) requireActiveYear(ctx context.Context, schoolID int64) (int64, error) {
	id, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoActiveAcademicYear
	}
	return id, nil
}

// currentYearID mengembalikan id tahun ajaran aktif, atau 0 bila belum ada
// (dipakai operasi baca yang boleh tetap jalan tanpa tahun ajaran aktif,
// hanya saja info rombel siswa jadi kosong).
func (s *Service) currentYearID(ctx context.Context, schoolID int64) (int64, error) {
	id, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return id, nil
}

func dedupeInt64(in []int64) []int64 {
	seen := make(map[int64]bool, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if v == 0 || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// -- students --

type ListStudentsQuery struct {
	Query   string
	ClassID int64
	Page    int
	PerPage int
}

type ListStudentsResult struct {
	Items []Student `json:"items"`
	Total int64     `json:"total"`
}

func (s *Service) ListStudents(ctx context.Context, schoolID int64, in ListStudentsQuery) (ListStudentsResult, error) {
	yearID, err := s.currentYearID(ctx, schoolID)
	if err != nil {
		return ListStudentsResult{}, err
	}
	page := in.Page
	if page < 1 {
		page = 1
	}
	perPage := in.PerPage
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	recs, total, err := s.repo.ListStudents(ctx, ListStudentsFilter{
		SchoolID: schoolID, AcademicYearID: yearID, Query: strings.TrimSpace(in.Query), ClassID: in.ClassID,
		Limit: int64(perPage), Offset: int64((page - 1) * perPage),
	})
	if err != nil {
		return ListStudentsResult{}, err
	}
	items := make([]Student, 0, len(recs))
	for _, r := range recs {
		items = append(items, r.View())
	}
	return ListStudentsResult{Items: items, Total: total}, nil
}

// checkStudentAccess menegakkan otorisasi GET /api/students/{id}: lolos bila
// role punya permission student:read (admin/kepsek/guru — sudah digerbang
// requirePerm di endpoint LAIN, tapi endpoint ini sengaja tidak memakai
// requirePerm karena siswa/orang_tua juga boleh akses object miliknya), ATAU
// siswa mengakses dirinya sendiri, ATAU orang tua mengakses anak yang
// terhubung lewat guardians (docs/03-student.md "Object-level rules").
func (s *Service) checkStudentAccess(ctx context.Context, rec StudentRecord) error {
	role := reqctx.Role(ctx)
	if s.identity.HasPermission(role, PermStudentRead) {
		return nil
	}
	userID := reqctx.UserID(ctx)
	switch role {
	case RoleSiswa:
		if rec.UserID != 0 && rec.UserID == userID {
			return nil
		}
	case RoleOrangTua:
		ok, err := s.repo.IsGuardianOf(ctx, userID, rec.ID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return httpx.ErrForbidden
}

// CanViewStudent adalah kebutuhan lintas modul (mis. attendance, lewat
// consumer-side interface StudentAccess yang dideklarasikan di sisi pemakai —
// lihat CLAUDE.md) untuk aturan object-level YANG SAMA dengan GetStudent:
// siswa boleh melihat dirinya sendiri, orang tua boleh melihat anak yang
// terhubung lewat guardians. Method ini SENGAJA TIDAK mengecek permission
// modul student (student:read) — pemanggil menggerbang permission MODULNYA
// SENDIRI lebih dulu (mis. attendance:report) dan hanya jatuh ke sini sebagai
// fallback object-level, persis pola checkStudentAccess di bawah.
func (s *Service) CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error {
	rec, err := s.repo.GetStudent(ctx, schoolID, 0, studentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	switch role {
	case RoleSiswa:
		if rec.UserID != 0 && rec.UserID == userID {
			return nil
		}
	case RoleOrangTua:
		ok, err := s.repo.IsGuardianOf(ctx, userID, rec.ID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return httpx.ErrForbidden
}

// MyClassID adalah kebutuhan lintas modul (mis. schedule fase 5, lewat
// consumer-side interface StudentClassLookup dideklarasikan di sisi pemakai —
// lihat CLAUDE.md) untuk object-level check: siswa hanya boleh melihat jadwal
// rombelnya sendiri. Mengembalikan (0, false, nil) bila user bukan siswa
// aktif/belum terdaftar di rombel pada tahun ajaran berjalan.
// MyTeacherID mengembalikan id profil guru milik user (ok=false bila user
// bukan guru di sekolah ini). Dipakai modul schedule via consumer-side
// interface untuk meng-infer jadwal "milik saya" tanpa parameter eksplisit.
func (s *Service) MyTeacherID(ctx context.Context, schoolID, userID int64) (int64, bool, error) {
	t, err := s.repo.GetTeacherByUserID(ctx, schoolID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return t.ID, true, nil
}

func (s *Service) MyClassID(ctx context.Context, schoolID, userID int64) (int64, bool, error) {
	yearID, err := s.currentYearID(ctx, schoolID)
	if err != nil {
		return 0, false, err
	}
	rec, err := s.repo.GetStudentByUserID(ctx, schoolID, yearID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if rec.ClassID == 0 {
		return 0, false, nil
	}
	return rec.ClassID, true, nil
}

// MyStudentID adalah kebutuhan lintas modul (attendance Fase 8, lewat
// consumer-side interface StudentAccess dideklarasikan di sisi pemakai —
// lihat CLAUDE.md) untuk resolve profil siswa (id + rombel) dari user login,
// dipakai self check-in GPS (siswa hanya boleh check-in utk dirinya sendiri,
// pada rombel tempat dia terdaftar di tahun ajaran BERJALAN). ok=false bila
// user bukan siswa aktif/belum terdaftar di rombel manapun tahun ini — sama
// seperti MyClassID.
func (s *Service) MyStudentID(ctx context.Context, schoolID, userID int64) (studentID, classID int64, ok bool, err error) {
	yearID, err := s.currentYearID(ctx, schoolID)
	if err != nil {
		return 0, 0, false, err
	}
	rec, err := s.repo.GetStudentByUserID(ctx, schoolID, yearID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, 0, false, nil
		}
		return 0, 0, false, err
	}
	if rec.ClassID == 0 {
		return 0, 0, false, nil
	}
	return rec.ID, rec.ClassID, true, nil
}

func (s *Service) GetStudent(ctx context.Context, schoolID, id int64) (Student, error) {
	yearID, err := s.currentYearID(ctx, schoolID)
	if err != nil {
		return Student{}, err
	}
	rec, err := s.repo.GetStudent(ctx, schoolID, yearID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Student{}, httpx.ErrNotFound
		}
		return Student{}, err
	}
	if err := s.checkStudentAccess(ctx, rec); err != nil {
		return Student{}, err
	}
	return rec.View(), nil
}

type CreateStudentRequest struct {
	NIS       string
	NISN      string
	Name      string
	Gender    string
	BirthDate *Date
	ClassID   int64
}

func validGender(g string) bool { return g == "" || g == "L" || g == "P" }

func (s *Service) CreateStudent(ctx context.Context, actorUserID, schoolID int64, in CreateStudentRequest) (Student, error) {
	nis := strings.TrimSpace(in.NIS)
	name := strings.TrimSpace(in.Name)
	if nis == "" {
		return Student{}, httpx.Validation("NIS wajib diisi.")
	}
	if name == "" {
		return Student{}, httpx.Validation("Nama siswa wajib diisi.")
	}
	if !validGender(in.Gender) {
		return Student{}, httpx.Validation("Jenis kelamin harus L atau P.")
	}
	if _, err := s.repo.GetStudentByNIS(ctx, schoolID, nis); err == nil {
		return Student{}, httpx.Validation("NIS sudah dipakai siswa lain di sekolah ini.")
	} else if !errors.Is(err, ErrNotFound) {
		return Student{}, err
	}

	rec, err := s.repo.CreateStudent(ctx, CreateStudentInput{
		SchoolID: schoolID, NIS: nis, NISN: in.NISN, Name: name, Gender: in.Gender, BirthDate: in.BirthDate,
	})
	if err != nil {
		return Student{}, err
	}

	if in.ClassID != 0 {
		cls, err := s.repo.GetClassByID(ctx, schoolID, in.ClassID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return Student{}, httpx.Validation("Rombel tidak ditemukan.")
			}
			return Student{}, err
		}
		if _, err := s.repo.EnrollStudentsBatch(ctx, schoolID, cls.ID, cls.AcademicYearID, []int64{rec.ID}); err != nil {
			return Student{}, err
		}
	}

	s.audit(ctx, schoolID, actorUserID, "student.create", "student", rec.ID, nil, rec)

	yearID, _ := s.currentYearID(ctx, schoolID)
	full, err := s.repo.GetStudent(ctx, schoolID, yearID, rec.ID)
	if err != nil {
		return rec.View(), nil
	}
	return full.View(), nil
}

func (s *Service) UpdateStudent(ctx context.Context, actorUserID, schoolID, id int64, in UpdateStudentInput) (Student, error) {
	in.SchoolID = schoolID
	in.ID = id

	if in.NIS != "" {
		nis := strings.TrimSpace(in.NIS)
		if existing, err := s.repo.GetStudentByNIS(ctx, schoolID, nis); err == nil && existing.ID != id {
			return Student{}, httpx.Validation("NIS sudah dipakai siswa lain di sekolah ini.")
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return Student{}, err
		}
		in.NIS = nis
	}
	if in.Name != "" {
		in.Name = strings.TrimSpace(in.Name)
	}
	if in.Gender != "" && !validGender(in.Gender) {
		return Student{}, httpx.Validation("Jenis kelamin harus L atau P.")
	}
	if in.Status != "" {
		switch in.Status {
		case "active", "graduated", "moved", "dropped":
		default:
			return Student{}, httpx.Validation("Status siswa tidak valid.")
		}
	}

	rec, err := s.repo.UpdateStudent(ctx, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Student{}, httpx.ErrNotFound
		}
		return Student{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "student.update", "student", id, nil, rec)

	yearID, _ := s.currentYearID(ctx, schoolID)
	full, err := s.repo.GetStudent(ctx, schoolID, yearID, id)
	if err != nil {
		return rec.View(), nil
	}
	return full.View(), nil
}

// -- classes --

func (s *Service) ListClasses(ctx context.Context, schoolID, academicYearID int64) ([]Class, error) {
	yearID := academicYearID
	if yearID == 0 {
		id, err := s.currentYearID(ctx, schoolID)
		if err != nil {
			return nil, err
		}
		yearID = id
	}
	if yearID == 0 {
		return []Class{}, nil
	}
	return s.repo.ListClasses(ctx, schoolID, yearID)
}

func (s *Service) classView(ctx context.Context, schoolID int64, rec ClassRecord) Class {
	classes, err := s.repo.ListClasses(ctx, schoolID, rec.AcademicYearID)
	if err == nil {
		for _, c := range classes {
			if c.ID == rec.ID {
				return c
			}
		}
	}
	return Class{ID: rec.ID, Name: rec.Name, Grade: rec.Grade, Major: rec.Major}
}

type CreateClassRequest struct {
	Name              string
	Grade             string
	Major             string
	HomeroomTeacherID int64
	AcademicYearID    int64
}

func (s *Service) CreateClass(ctx context.Context, actorUserID, schoolID int64, in CreateClassRequest) (Class, error) {
	name := strings.TrimSpace(in.Name)
	grade := strings.TrimSpace(in.Grade)
	if name == "" {
		return Class{}, httpx.Validation("Nama rombel wajib diisi.")
	}
	if grade == "" {
		return Class{}, httpx.Validation("Tingkat (grade) wajib diisi.")
	}

	yearID := in.AcademicYearID
	if yearID == 0 {
		id, err := s.requireActiveYear(ctx, schoolID)
		if err != nil {
			return Class{}, err
		}
		yearID = id
	}

	if in.HomeroomTeacherID != 0 {
		if _, err := s.repo.GetTeacherByID(ctx, schoolID, in.HomeroomTeacherID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return Class{}, httpx.Validation("Wali kelas (guru) tidak ditemukan.")
			}
			return Class{}, err
		}
	}

	if _, err := s.repo.GetClassByNameYear(ctx, schoolID, yearID, name); err == nil {
		return Class{}, httpx.Validation("Rombel dengan nama ini sudah ada pada tahun ajaran tersebut.")
	} else if !errors.Is(err, ErrNotFound) {
		return Class{}, err
	}

	rec, err := s.repo.CreateClass(ctx, ClassRecord{
		SchoolID: schoolID, AcademicYearID: yearID, Name: name, Grade: grade,
		Major: strings.TrimSpace(in.Major), HomeroomTeacherID: in.HomeroomTeacherID,
	})
	if err != nil {
		return Class{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "class.create", "class", rec.ID, nil, rec)
	return s.classView(ctx, schoolID, rec), nil
}

func (s *Service) UpdateClass(ctx context.Context, actorUserID, schoolID, id int64, in UpdateClassInput) (Class, error) {
	in.SchoolID = schoolID
	in.ID = id

	if in.HomeroomTeacherID != 0 {
		if _, err := s.repo.GetTeacherByID(ctx, schoolID, in.HomeroomTeacherID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return Class{}, httpx.Validation("Wali kelas (guru) tidak ditemukan.")
			}
			return Class{}, err
		}
	}

	rec, err := s.repo.UpdateClass(ctx, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Class{}, httpx.ErrNotFound
		}
		return Class{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "class.update", "class", id, nil, rec)
	return s.classView(ctx, schoolID, rec), nil
}

// -- enrollments --

func (s *Service) EnrollStudents(ctx context.Context, actorUserID, schoolID, classID int64, studentIDs []int64) error {
	ids := dedupeInt64(studentIDs)
	if len(ids) == 0 {
		return httpx.Validation("student_ids wajib diisi.")
	}
	cls, err := s.repo.GetClassByID(ctx, schoolID, classID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	valid, err := s.repo.ListStudentsByIDs(ctx, schoolID, ids)
	if err != nil {
		return err
	}
	if len(valid) != len(ids) {
		return httpx.Validation("Sebagian student_id tidak ditemukan di sekolah ini.")
	}

	results, err := s.repo.EnrollStudentsBatch(ctx, schoolID, cls.ID, cls.AcademicYearID, ids)
	if err != nil {
		return err
	}
	for _, res := range results {
		if res.Moved {
			s.audit(ctx, schoolID, actorUserID, "enrollment.move", "enrollment", res.StudentID,
				map[string]any{"class_id": res.FromClassID}, map[string]any{"class_id": cls.ID})
		}
	}
	s.audit(ctx, schoolID, actorUserID, "enrollment.batch_enroll", "class", classID, nil,
		map[string]any{"student_ids": ids})
	return nil
}

func (s *Service) UnenrollStudent(ctx context.Context, actorUserID, schoolID, classID, studentID int64) error {
	cls, err := s.repo.GetClassByID(ctx, schoolID, classID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	if err := s.repo.UnenrollStudent(ctx, studentID, cls.ID); err != nil {
		return err
	}
	s.audit(ctx, schoolID, actorUserID, "enrollment.remove", "enrollment", studentID, nil,
		map[string]any{"class_id": cls.ID})
	return nil
}

// -- teachers --

func slugifyName(name string) string {
	var b strings.Builder
	prevDot := true
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDot = false
		default:
			if !prevDot {
				b.WriteRune('.')
				prevDot = true
			}
		}
	}
	out := strings.Trim(b.String(), ".")
	if out == "" {
		out = "guru"
	}
	return out
}

func (s *Service) uniqueUsernameFor(ctx context.Context, name string) (string, error) {
	base := slugifyName(name)
	candidate := base
	for i := 0; i < 50; i++ {
		if i > 0 {
			candidate = fmt.Sprintf("%s%d", base, i+1)
		}
		_, ok, err := s.identity.UserIDByUsername(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !ok {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("student: gagal membuat username unik untuk %q", name)
}

func randomPlaceholderPassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type CreateTeacherRequest struct {
	Name  string
	Email string
	NIP   string
}

// createTeacherAccount membuat user (email atau username slug dari nama),
// password placeholder acak (akun aktif lewat undangan/reset — lihat
// docs/03-student.md), profil teachers, dan membership role guru.
func (s *Service) createTeacherAccount(ctx context.Context, schoolID int64, name, email, nip string) (Teacher, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Teacher{}, httpx.Validation("Nama guru wajib diisi.")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	var username string
	if email != "" {
		if _, ok, err := s.identity.UserIDByEmail(ctx, email); err != nil {
			return Teacher{}, err
		} else if ok {
			return Teacher{}, httpx.Validation("Email sudah dipakai akun lain.")
		}
	} else {
		u, err := s.uniqueUsernameFor(ctx, name)
		if err != nil {
			return Teacher{}, err
		}
		username = u
	}

	pw, err := randomPlaceholderPassword()
	if err != nil {
		return Teacher{}, err
	}
	hash, err := s.identity.HashPassword(pw)
	if err != nil {
		return Teacher{}, err
	}
	userID, err := s.identity.CreateAccount(ctx, email, username, hash, name)
	if err != nil {
		return Teacher{}, err
	}
	if err := s.identity.CreateMembership(ctx, userID, schoolID, RoleGuru); err != nil {
		return Teacher{}, err
	}
	return s.repo.CreateTeacher(ctx, schoolID, userID, strings.TrimSpace(nip))
}

func (s *Service) CreateTeacher(ctx context.Context, actorUserID, schoolID int64, in CreateTeacherRequest) (Teacher, error) {
	rec, err := s.createTeacherAccount(ctx, schoolID, in.Name, in.Email, in.NIP)
	if err != nil {
		return Teacher{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "teacher.create", "teacher", rec.ID, nil, rec)
	return rec, nil
}

func (s *Service) UpdateTeacher(ctx context.Context, actorUserID, schoolID, id int64, nip string) (Teacher, error) {
	rec, err := s.repo.UpdateTeacher(ctx, schoolID, id, strings.TrimSpace(nip))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Teacher{}, httpx.ErrNotFound
		}
		return Teacher{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "teacher.update", "teacher", id, nil, rec)
	return rec, nil
}

func (s *Service) ListTeachers(ctx context.Context, schoolID int64) ([]Teacher, error) {
	return s.repo.ListTeachers(ctx, schoolID)
}

// -- subjects --

type CreateSubjectRequest struct {
	Code string
	Name string
}

func (s *Service) CreateSubject(ctx context.Context, actorUserID, schoolID int64, in CreateSubjectRequest) (Subject, error) {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	name := strings.TrimSpace(in.Name)
	if code == "" || name == "" {
		return Subject{}, httpx.Validation("Kode dan nama mapel wajib diisi.")
	}
	if _, err := s.repo.GetSubjectByCode(ctx, schoolID, code); err == nil {
		return Subject{}, httpx.Validation("Kode mapel sudah dipakai.")
	} else if !errors.Is(err, ErrNotFound) {
		return Subject{}, err
	}
	rec, err := s.repo.CreateSubject(ctx, schoolID, code, name)
	if err != nil {
		return Subject{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "subject.create", "subject", rec.ID, nil, rec)
	return rec, nil
}

func (s *Service) UpdateSubject(ctx context.Context, actorUserID, schoolID, id int64, code, name string) (Subject, error) {
	rec, err := s.repo.UpdateSubject(ctx, schoolID, id, strings.ToUpper(strings.TrimSpace(code)), strings.TrimSpace(name))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Subject{}, httpx.ErrNotFound
		}
		return Subject{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "subject.update", "subject", id, nil, rec)
	return rec, nil
}

func (s *Service) ListSubjects(ctx context.Context, schoolID int64) ([]Subject, error) {
	return s.repo.ListSubjects(ctx, schoolID)
}

// ListMyChildren — GET /api/me/children (role orang_tua): daftar anak yang
// terhubung lewat guardians, disertai nama rombel pada tahun ajaran aktif.
func (s *Service) ListMyChildren(ctx context.Context, schoolID, userID int64) ([]ChildRef, error) {
	yearID, err := s.currentYearID(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListChildrenForGuardian(ctx, schoolID, userID, yearID)
}

// -- undangan --

// GenerateInvitations membuat kode undangan untuk tiap siswa di suatu rombel
// yang belum punya akun (kode siswa) dan/atau belum punya wali terdaftar
// (kode wali). Idempoten: kode aktif yang sudah ada dipakai ulang (lihat
// identity.Service.GenerateInvitationCode).
func (s *Service) GenerateInvitations(ctx context.Context, actorUserID, schoolID, classID int64) ([]InvitationCodes, error) {
	cls, err := s.repo.GetClassByID(ctx, schoolID, classID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.ErrNotFound
		}
		return nil, err
	}
	students, _, err := s.repo.ListStudents(ctx, ListStudentsFilter{
		SchoolID: schoolID, AcademicYearID: cls.AcademicYearID, ClassID: cls.ID, Limit: 1000, Offset: 0,
	})
	if err != nil {
		return nil, err
	}

	out := make([]InvitationCodes, 0, len(students))
	for _, st := range students {
		item := InvitationCodes{StudentID: st.ID, StudentName: st.Name}
		if st.UserID == 0 {
			code, _, err := s.identity.GenerateInvitationCode(ctx, schoolID, RoleSiswa, st.ID, invitationTTL)
			if err != nil {
				return nil, err
			}
			item.StudentCode = &code
		}
		hasGuardian, err := s.repo.HasGuardian(ctx, st.ID)
		if err != nil {
			return nil, err
		}
		if !hasGuardian {
			code, _, err := s.identity.GenerateInvitationCode(ctx, schoolID, RoleOrangTua, st.ID, invitationTTL)
			if err != nil {
				return nil, err
			}
			item.GuardianCode = &code
		}
		out = append(out, item)
	}
	s.audit(ctx, schoolID, actorUserID, "invitation.generate", "class", classID, nil, map[string]any{"count": len(out)})
	return out, nil
}

// GetInvitationInfo — GET /api/auth/invitation/{code} (publik).
func (s *Service) GetInvitationInfo(ctx context.Context, schoolID int64, schoolName, code string) (InvitationInfo, error) {
	invSchoolID, role, targetID, ok, err := s.identity.ValidateInvitationCode(ctx, code)
	if err != nil {
		return InvitationInfo{}, err
	}
	if !ok || invSchoolID != schoolID {
		return InvitationInfo{}, ErrInvitationInvalid
	}
	st, err := s.repo.GetStudent(ctx, schoolID, 0, targetID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return InvitationInfo{}, ErrInvitationInvalid
		}
		return InvitationInfo{}, err
	}
	return InvitationInfo{Role: role, StudentName: st.Name, SchoolName: schoolName}, nil
}

// SchoolRef adalah potongan info sekolah disematkan pada ActivatedUserView.
type SchoolRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ActivatedUserView meniru shape identity.UserView supaya response POST
// /api/auth/activate konsisten dengan POST /api/auth/login (docs/03).
type ActivatedUserView struct {
	ID     int64      `json:"id"`
	Name   string     `json:"name"`
	Role   string     `json:"role"`
	School *SchoolRef `json:"school"`
}

type ActivateInput struct {
	Code      string
	Name      string
	Username  string
	Email     string
	Password  string
	IP        string
	UserAgent string
}

type ActivateResult struct {
	Token     string
	ExpiresAt time.Time
	View      ActivatedUserView
}

// Activate — POST /api/auth/activate (publik). Membuat akun dari kode
// undangan (siswa: username wajib; orang_tua: email atau username), lalu
// auto-login (mengeluarkan sesi — handler yang memasang cookie lewat
// SetSessionCookie).
func (s *Service) Activate(ctx context.Context, schoolID int64, school SchoolRef, in ActivateInput) (ActivateResult, error) {
	code := strings.TrimSpace(in.Code)
	if code == "" {
		return ActivateResult{}, httpx.Validation("Kode undangan wajib diisi.")
	}
	if len(in.Password) < 8 {
		return ActivateResult{}, httpx.Validation("Kata sandi minimal 8 karakter.")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ActivateResult{}, httpx.Validation("Nama wajib diisi.")
	}

	invSchoolID, role, targetID, ok, err := s.identity.ValidateInvitationCode(ctx, code)
	if err != nil {
		return ActivateResult{}, err
	}
	if !ok || invSchoolID != schoolID {
		return ActivateResult{}, ErrInvitationUsed
	}

	username := strings.TrimSpace(in.Username)
	email := strings.ToLower(strings.TrimSpace(in.Email))

	switch role {
	case RoleSiswa:
		if username == "" {
			return ActivateResult{}, httpx.Validation("Username wajib diisi untuk akun siswa.")
		}
	case RoleOrangTua:
		if username == "" && email == "" {
			return ActivateResult{}, httpx.Validation("Email atau username wajib diisi untuk akun orang tua.")
		}
	default:
		return ActivateResult{}, ErrInvitationUsed
	}

	if email != "" {
		if _, exists, err := s.identity.UserIDByEmail(ctx, email); err != nil {
			return ActivateResult{}, err
		} else if exists {
			return ActivateResult{}, httpx.Validation("Email sudah dipakai akun lain.")
		}
	}
	if username != "" {
		if _, exists, err := s.identity.UserIDByUsername(ctx, username); err != nil {
			return ActivateResult{}, err
		} else if exists {
			return ActivateResult{}, httpx.Validation("Username sudah dipakai akun lain.")
		}
	}

	hash, err := s.identity.HashPassword(in.Password)
	if err != nil {
		return ActivateResult{}, err
	}
	userID, err := s.identity.CreateAccount(ctx, email, username, hash, name)
	if err != nil {
		return ActivateResult{}, err
	}
	if err := s.identity.CreateMembership(ctx, userID, schoolID, role); err != nil {
		return ActivateResult{}, err
	}

	switch role {
	case RoleSiswa:
		if err := s.repo.SetStudentUserID(ctx, targetID, userID); err != nil {
			return ActivateResult{}, err
		}
	case RoleOrangTua:
		if err := s.repo.CreateGuardian(ctx, schoolID, userID, targetID, RelationWali); err != nil {
			return ActivateResult{}, err
		}
	}

	if err := s.identity.MarkInvitationUsed(ctx, code); err != nil {
		return ActivateResult{}, err
	}

	token, expiresAt, err := s.identity.IssueSession(ctx, userID, schoolID, role, in.IP, in.UserAgent)
	if err != nil {
		return ActivateResult{}, err
	}

	s.audit(ctx, schoolID, userID, "invitation.activate", "user", userID, nil,
		map[string]any{"role": role, "target_id": targetID})

	return ActivateResult{
		Token: token, ExpiresAt: expiresAt,
		View: ActivatedUserView{ID: userID, Name: name, Role: role, School: &school},
	}, nil
}

// SetSessionCookie meneruskan ke IdentityGateway (dipanggil handler setelah Activate sukses).
func (s *Service) SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	s.identity.SetSessionCookie(w, token, expiresAt)
}
