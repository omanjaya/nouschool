package counseling

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
	"github.com/omanjaya/nouschool/internal/platform/storage"
)

// -- consumer-side interface (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI). Semua dipenuhi *identity.Service /
// *tenant.Service / *duty.Service secara STRUKTURAL — counseling TIDAK
// mengimpor package-package itu.

// IdentityGateway adalah kebutuhan modul counseling dari modul identity: audit log.
type IdentityGateway interface {
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// AcademicYearLookup adalah kebutuhan modul counseling dari modul tenant.
type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// DutyGateway adalah kebutuhan modul counseling dari modul duty: otorisasi
// berdasarkan CAPABILITY FLAGS (docs/12-sion-parity.md Gelombang B —
// "otorisasi alur membaca flags, bukan role"). Pemegang flag
// FlagLeaveIssuance = Guru BK.
type DutyGateway interface {
	UserHasFlag(ctx context.Context, schoolID, userID int64, flag string) (bool, error)
}

// ErrNoActiveAcademicYear — sekolah belum mengaktifkan tahun ajaran.
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// Service berisi aturan bisnis modul counseling.
type Service struct {
	repo     counselingRepository
	identity IdentityGateway
	years    AcademicYearLookup
	duties   DutyGateway
	files    *storage.Store
	clock    clock.Clock
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup, duties DutyGateway, files *storage.Store, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	if files == nil {
		files = storage.FromEnv()
	}
	return &Service{repo: repo, identity: identity, years: years, duties: duties, files: files, clock: clk}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja.
func newServiceForTest(repo counselingRepository, identity IdentityGateway, years AcademicYearLookup, duties DutyGateway, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, years: years, duties: duties, files: storage.New(""), clock: clk}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

// canManage — pemegang flag duty leave_issuance (Guru BK) ATAU admin_sekolah
// (docs tugas Fase 14 Gelombang D "Konseling BK"). userID diambil dari
// reqctx (bukan parameter) supaya pemanggilan seragam di seluruh service.
func (s *Service) canManage(ctx context.Context, schoolID int64) (bool, error) {
	if reqctx.Role(ctx) == RoleAdminSekolah {
		return true, nil
	}
	return s.duties.UserHasFlag(ctx, schoolID, reqctx.UserID(ctx), FlagLeaveIssuance)
}

func (s *Service) requireManage(ctx context.Context, schoolID int64) error {
	ok, err := s.canManage(ctx, schoolID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.ErrForbidden
	}
	return nil
}

// requireRead — manage() ATAU kepala_sekolah (read-only, docs tugas).
// Siswa/ortu TIDAK PERNAH lolos (privat BK) — mereka bukan admin_sekolah,
// bukan pemegang flag leave_issuance, dan bukan kepala_sekolah.
func (s *Service) requireRead(ctx context.Context, schoolID int64) error {
	if reqctx.Role(ctx) == RoleKepalaSekolah {
		return nil
	}
	return s.requireManage(ctx, schoolID)
}

func (s *Service) activeYear(ctx context.Context, schoolID int64) (int64, error) {
	id, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoActiveAcademicYear
	}
	return id, nil
}

func rowView(r Row) CounselingView {
	return CounselingView{
		ID:          r.ID,
		Student:     StudentRef{ID: r.StudentID, Name: r.StudentName, NIS: r.StudentNIS, ClassName: r.ClassName},
		CounselorID: r.CounselorID, CounselorName: r.CounselorName,
		CareerGoals: r.CareerGoals, ProblemDescription: r.ProblemDescription, FollowUpPlan: r.FollowUpPlan,
		HasEvidence: r.Evidence != "", EvidenceName: r.EvidenceName,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// -- GET /api/counselings?student_id=&page= --

const counselingPerPage = 20

func (s *Service) ListCounselings(ctx context.Context, schoolID, studentID int64, page int) (ListResult, error) {
	if err := s.requireRead(ctx, schoolID); err != nil {
		return ListResult{}, err
	}
	if page <= 0 {
		page = 1
	}
	rows, err := s.repo.ListRows(ctx, schoolID, studentID, counselingPerPage, (page-1)*counselingPerPage)
	if err != nil {
		return ListResult{}, err
	}
	total, err := s.repo.CountRows(ctx, schoolID, studentID)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]CounselingView, 0, len(rows))
	for _, r := range rows {
		items = append(items, rowView(r))
	}
	return ListResult{Items: items, Page: page, PerPage: counselingPerPage, TotalCount: total}, nil
}

// -- POST /api/counselings (multipart) --

// maxEvidenceSize — batas ukuran bukti (docs tugas: "jpg/png/pdf ≤5MB", pola
// sama internal/leave.maxAttachmentSize).
const maxEvidenceSize = 5 << 20

var allowedEvidenceTypes = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
}

var (
	errEvidenceTooLarge = httpx.Validation("Ukuran bukti maksimal 5MB.")
	errEvidenceType     = httpx.Validation("Bukti harus berformat PDF, JPG, atau PNG.")
)

// EvidenceUpload adalah bukti mentah dari multipart form (opsional).
type EvidenceUpload struct {
	Filename string
	Content  []byte
}

func detectEvidenceExt(content []byte) (string, string, error) {
	if len(content) > maxEvidenceSize {
		return "", "", errEvidenceTooLarge
	}
	mime := http.DetectContentType(content)
	for prefix, ext := range allowedEvidenceTypes {
		if mime == prefix {
			return mime, ext, nil
		}
	}
	return "", "", errEvidenceType
}

// CreateInput adalah parameter Service.CreateCounseling.
type CreateInput struct {
	StudentID          int64
	CareerGoals        string
	ProblemDescription string
	FollowUpPlan       string
	Evidence           *EvidenceUpload
}

// CreateCounseling — POST /api/counselings. counselor_id SELALU actor
// (pembuat sesi ini), TIDAK bisa dititipkan lewat body (docs tugas).
func (s *Service) CreateCounseling(ctx context.Context, actorUserID, schoolID int64, in CreateInput) (CounselingView, error) {
	if err := s.requireManage(ctx, schoolID); err != nil {
		return CounselingView{}, err
	}
	if in.StudentID == 0 {
		return CounselingView{}, httpx.Validation("student_id wajib diisi.")
	}
	if _, err := s.repo.GetStudentBasic(ctx, schoolID, in.StudentID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return CounselingView{}, httpx.Validation("Siswa tidak ditemukan.")
		}
		return CounselingView{}, err
	}
	yearID, err := s.activeYear(ctx, schoolID)
	if err != nil {
		return CounselingView{}, err
	}

	var evidence, evidenceName, evidenceMime string
	if in.Evidence != nil {
		mime, ext, err := detectEvidenceExt(in.Evidence.Content)
		if err != nil {
			return CounselingView{}, err
		}
		filename, err := storage.RandomFilename(ext)
		if err != nil {
			return CounselingView{}, err
		}
		relPath := fmt.Sprintf("%d/counseling/%s", schoolID, filename)
		if err := s.files.Save(relPath, bytes.NewReader(in.Evidence.Content)); err != nil {
			return CounselingView{}, err
		}
		evidence, evidenceName, evidenceMime = relPath, in.Evidence.Filename, mime
	}

	rec, err := s.repo.CreateCounseling(ctx, CreateRecordInput{
		SchoolID: schoolID, AcademicYearID: yearID, StudentID: in.StudentID, CounselorID: actorUserID,
		CareerGoals: strings.TrimSpace(in.CareerGoals), ProblemDescription: strings.TrimSpace(in.ProblemDescription),
		FollowUpPlan: strings.TrimSpace(in.FollowUpPlan), Evidence: evidence, EvidenceName: evidenceName, EvidenceMime: evidenceMime,
	})
	if err != nil {
		return CounselingView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "counseling.create", "counseling", rec.ID, nil,
		map[string]any{"student_id": in.StudentID})

	row, err := s.repo.GetRow(ctx, schoolID, rec.ID)
	if err != nil {
		return CounselingView{}, err
	}
	return rowView(row), nil
}

// -- PATCH /api/counselings/{id} --

// UpdateInput adalah parameter Service.UpdateCounseling — semua field
// menggantikan nilai lama (replace-all, pola sama teaching.PatchInput).
type UpdateInput struct {
	CareerGoals        string
	ProblemDescription string
	FollowUpPlan       string
}

func (s *Service) getOwnerCheckedRecord(ctx context.Context, actorUserID, schoolID, id int64) (Record, error) {
	rec, err := s.repo.GetByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Record{}, httpx.ErrNotFound
		}
		return Record{}, err
	}
	if reqctx.Role(ctx) == RoleAdminSekolah || rec.CounselorID == actorUserID {
		return rec, nil
	}
	return Record{}, httpx.ErrForbidden
}

// UpdateCounseling — PATCH /api/counselings/{id}: pembuat sesi (counselor_id)
// atau admin_sekolah (docs tugas).
func (s *Service) UpdateCounseling(ctx context.Context, actorUserID, schoolID, id int64, in UpdateInput) (CounselingView, error) {
	old, err := s.getOwnerCheckedRecord(ctx, actorUserID, schoolID, id)
	if err != nil {
		return CounselingView{}, err
	}
	rec, err := s.repo.UpdateCounseling(ctx, schoolID, id, UpdateRecordInput{
		CareerGoals: strings.TrimSpace(in.CareerGoals), ProblemDescription: strings.TrimSpace(in.ProblemDescription),
		FollowUpPlan: strings.TrimSpace(in.FollowUpPlan),
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return CounselingView{}, httpx.ErrNotFound
		}
		return CounselingView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "counseling.update", "counseling", id,
		map[string]any{"career_goals": old.CareerGoals, "problem_description": old.ProblemDescription, "follow_up_plan": old.FollowUpPlan},
		map[string]any{"career_goals": rec.CareerGoals, "problem_description": rec.ProblemDescription, "follow_up_plan": rec.FollowUpPlan})

	row, err := s.repo.GetRow(ctx, schoolID, id)
	if err != nil {
		return CounselingView{}, err
	}
	return rowView(row), nil
}

// -- DELETE /api/counselings/{id} --

func (s *Service) DeleteCounseling(ctx context.Context, actorUserID, schoolID, id int64) error {
	old, err := s.getOwnerCheckedRecord(ctx, actorUserID, schoolID, id)
	if err != nil {
		return err
	}
	rows, err := s.repo.DeleteCounseling(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.ErrNotFound
	}
	s.audit(ctx, schoolID, actorUserID, "counseling.delete", "counseling", id,
		map[string]any{"student_id": old.StudentID}, nil)
	return nil
}

// -- GET /api/counselings/{id}/evidence --

func (s *Service) GetEvidence(ctx context.Context, schoolID, id int64) (EvidenceInfo, error) {
	if err := s.requireRead(ctx, schoolID); err != nil {
		return EvidenceInfo{}, err
	}
	rec, err := s.repo.GetByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return EvidenceInfo{}, httpx.ErrNotFound
		}
		return EvidenceInfo{}, err
	}
	if rec.Evidence == "" {
		return EvidenceInfo{}, httpx.ErrNotFound
	}
	return EvidenceInfo{Path: rec.Evidence, Name: rec.EvidenceName, ContentType: rec.EvidenceMime}, nil
}

func (s *Service) OpenEvidence(path string) (io.ReadCloser, error) {
	return s.files.Open(path)
}

// -- GET /api/counselings/{id}/report/html --

func (s *Service) ReportHTML(ctx context.Context, schoolID, id int64) (string, error) {
	if err := s.requireRead(ctx, schoolID); err != nil {
		return "", err
	}
	row, err := s.repo.GetRow(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", httpx.ErrNotFound
		}
		return "", err
	}
	return RenderReportHTML(row, "NouSchool"), nil
}
