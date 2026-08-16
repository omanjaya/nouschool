package studentleave

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
	"github.com/omanjaya/nouschool/internal/platform/storage"
)

// -- consumer-side interface (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, signature primitif). Semua dipenuhi
// *identity.Service / *tenant.Service / *student.Service / *duty.Service
// secara STRUKTURAL — studentleave TIDAK mengimpor package-package itu.

// IdentityGateway adalah kebutuhan modul studentleave dari modul identity.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// AcademicYearLookup adalah kebutuhan modul studentleave dari modul tenant.
type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// BrandingGateway adalah kebutuhan modul studentleave dari modul tenant: kop
// surat izin PDF/HTML.
type BrandingGateway interface {
	BrandingAppName(ctx context.Context, schoolID int64) (string, error)
}

// StudentAccess adalah kebutuhan modul studentleave dari modul student:
// object-level (siswa sendiri/orang tua anaknya), resolusi anak orang tua
// (scope=mine), resolusi profil siswa yang login (submit sebagai diri
// sendiri), dan resolusi penerima notifikasi ortu.
type StudentAccess interface {
	CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error
	GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error)
	MyStudentID(ctx context.Context, schoolID, userID int64) (studentID, classID int64, ok bool, err error)
	MyChildStudentIDs(ctx context.Context, schoolID, userID int64) ([]int64, error)
}

// DutyGateway adalah kebutuhan modul studentleave dari modul duty:
// otorisasi alur berdasarkan CAPABILITY FLAGS (docs/12-sion-parity.md
// Gelombang B — "otorisasi alur izin membaca flags, bukan role").
type DutyGateway interface {
	UserHasFlag(ctx context.Context, schoolID, userID int64, flag string) (bool, error)
	UserIDsWithFlag(ctx context.Context, schoolID int64, flag string) ([]int64, error)
}

// Notifier adalah kebutuhan modul studentleave dari modul notification.
type Notifier interface {
	Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error
}

// Realtime adalah kebutuhan modul studentleave dari modul realtime.
type Realtime interface {
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

// SettingsGateway adalah kebutuhan modul studentleave dari modul tenant: baca
// RAW json school_settings module "letters" (Fase 14 Gelombang D,
// docs/12-sion-parity.md "template surat") — pola sama
// internal/discipline.SettingsGateway / internal/grading.SettingsGateway.
type SettingsGateway interface {
	GetSetting(ctx context.Context, schoolID int64, module string) (raw []byte, found bool, err error)
}

// ErrNoActiveAcademicYear — pesan sama seperti modul lain (mis. discipline/duty).
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// Service berisi aturan bisnis modul studentleave.
type Service struct {
	repo     studentLeaveRepository
	identity IdentityGateway
	years    AcademicYearLookup
	branding BrandingGateway
	students StudentAccess
	duties   DutyGateway
	files    *storage.Store
	clock    clock.Clock
	notifier Notifier
	realtime Realtime
	settings SettingsGateway // opsional — nil = footer surat izin dilewati (Fase 14 Gelombang D)
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup, branding BrandingGateway, students StudentAccess, duties DutyGateway, files *storage.Store, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	if files == nil {
		files = storage.FromEnv()
	}
	return &Service{repo: repo, identity: identity, years: years, branding: branding, students: students, duties: duties, files: files, clock: clk}
}

func (s *Service) SetNotifier(n Notifier)               { s.notifier = n }
func (s *Service) SetRealtime(r Realtime)               { s.realtime = r }
func (s *Service) SetSettingsGateway(g SettingsGateway) { s.settings = g }

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja.
func newServiceForTest(repo studentLeaveRepository, identity IdentityGateway, years AcademicYearLookup, branding BrandingGateway, students StudentAccess, duties DutyGateway, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, years: years, branding: branding, students: students, duties: duties, files: storage.New(""), clock: clk}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func (s *Service) notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) {
	if s.notifier == nil || len(userIDs) == 0 {
		return
	}
	_ = s.notifier.Notify(ctx, schoolID, event, userIDs, data)
}

func (s *Service) emit(schoolID, requestID int64, extraUserIDs []int64) {
	if s.realtime == nil {
		return
	}
	s.realtime.PublishTo(schoolID, "studentleave", map[string]any{"request_id": requestID},
		[]string{RoleAdminSekolah}, extraUserIDs)
}

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

func schoolToday(now time.Time, tz string) time.Time {
	local := clock.InZone(now, tz)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

func parseDate(raw string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	return t, nil
}

// dedupInt64 mempertahankan urutan kemunculan pertama, membuang 0 & duplikat.
func dedupInt64(ids ...int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// -- lampiran (pola IDENTIK internal/leave) --

type AttachmentUpload struct {
	Filename string
	Content  []byte
}

var errAttachmentTooLarge = httpx.Validation("Ukuran lampiran maksimal 5MB.")
var errAttachmentType = httpx.Validation("Lampiran harus berformat PDF, JPG, atau PNG.")

func detectAttachmentExt(content []byte) (string, string, error) {
	if len(content) > maxAttachmentSize {
		return "", "", errAttachmentTooLarge
	}
	mime := http.DetectContentType(content)
	for prefix, ext := range allowedAttachmentTypes {
		if mime == prefix {
			return mime, ext, nil
		}
	}
	return "", "", errAttachmentType
}

// -- POST /api/student-leave --

// SubmitInput adalah parameter Service.SubmitRequest.
type SubmitInput struct {
	StudentID  int64 // dipakai HANYA untuk role orang_tua (validasi CanViewStudent)
	Type       string
	DateStart  string
	DateEnd    string
	Reason     string
	Attachment *AttachmentUpload
}

// SubmitRequest — siswa (untuk dirinya) atau orang tua (untuk anaknya, via
// object-level CanViewStudent) mengajukan izin terencana. Notif pemegang
// review tahap 1: wali kelas rombel siswa (bila ada) — fallback SEMUA
// pemegang flag leave_homeroom_review bila kelas belum punya wali.
func (s *Service) SubmitRequest(ctx context.Context, actorUserID, schoolID int64, in SubmitInput) (RequestView, error) {
	role := reqctx.Role(ctx)
	var studentID int64
	switch role {
	case RoleSiswa:
		id, _, ok, err := s.students.MyStudentID(ctx, schoolID, actorUserID)
		if err != nil {
			return RequestView{}, err
		}
		if !ok {
			return RequestView{}, httpx.Validation("Akun Anda belum tertaut ke data siswa.")
		}
		studentID = id
	case RoleOrangTua:
		if in.StudentID == 0 {
			return RequestView{}, httpx.Validation("student_id wajib diisi.")
		}
		if err := s.students.CanViewStudent(ctx, actorUserID, role, schoolID, in.StudentID); err != nil {
			return RequestView{}, err
		}
		studentID = in.StudentID
	default:
		return RequestView{}, httpx.ErrForbidden
	}

	if !validType(in.Type) {
		return RequestView{}, httpx.Validation("Jenis izin harus 'sakit' atau 'izin'.")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return RequestView{}, httpx.Validation("Alasan wajib diisi.")
	}
	dateStart, err := parseDate(in.DateStart)
	if err != nil {
		return RequestView{}, err
	}
	dateEnd, err := parseDate(in.DateEnd)
	if err != nil {
		return RequestView{}, err
	}
	if dateStart.After(dateEnd) {
		return RequestView{}, httpx.Validation("Tanggal mulai tidak boleh setelah tanggal selesai.")
	}
	today := schoolToday(s.clock.Now(), schoolTimezone(ctx))
	if dateStart.Before(today.AddDate(0, 0, -30)) {
		return RequestView{}, httpx.Validation("Tanggal mulai tidak boleh lebih dari 30 hari ke belakang.")
	}
	if dateEnd.After(today.AddDate(0, 3, 0)) {
		return RequestView{}, httpx.Validation("Tanggal selesai tidak boleh lebih dari 3 bulan ke depan.")
	}

	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return RequestView{}, err
	}

	var attachment, attachmentName, attachmentMime string
	if in.Attachment != nil {
		mime, ext, err := detectAttachmentExt(in.Attachment.Content)
		if err != nil {
			return RequestView{}, err
		}
		filename, err := storage.RandomFilename(ext)
		if err != nil {
			return RequestView{}, err
		}
		relPath := fmt.Sprintf("%d/student-leave/%s", schoolID, filename)
		if err := s.files.Save(relPath, bytes.NewReader(in.Attachment.Content)); err != nil {
			return RequestView{}, err
		}
		attachment, attachmentName, attachmentMime = relPath, in.Attachment.Filename, mime
	}

	id, err := s.repo.CreateRequest(ctx, CreateRequestInput{
		SchoolID: schoolID, AcademicYearID: yearID, StudentID: studentID, SubmittedBy: actorUserID, Type: in.Type,
		DateStart: dateStart, DateEnd: dateEnd, Reason: reason,
		Attachment: attachment, AttachmentName: attachmentName, AttachmentMime: attachmentMime,
	})
	if err != nil {
		return RequestView{}, err
	}

	detail, err := s.repo.GetRequestDetail(ctx, schoolID, id)
	if err != nil {
		return RequestView{}, err
	}

	s.audit(ctx, schoolID, actorUserID, "studentleave.submit", "student_leave_request", id, nil,
		map[string]any{"student_id": studentID, "type": in.Type, "date_start": in.DateStart, "date_end": in.DateEnd})

	reviewers := s.homeroomReviewerTargets(ctx, schoolID, yearID, studentID)
	s.notify(ctx, schoolID, EventStudentLeaveSubmitted, reviewers, map[string]any{
		"student": detail.StudentName, "type": in.Type, "date_start": in.DateStart, "date_end": in.DateEnd,
	})
	s.emit(schoolID, id, dedupInt64(append([]int64{actorUserID}, reviewers...)...))

	return detail.View(), nil
}

// homeroomReviewerTargets — wali kelas rombel siswa (via join langsung
// classes.homeroom_teacher_id TA berjalan) bila ADA; fallback SEMUA pemegang
// flag leave_homeroom_review bila kelas belum punya wali (docs tugas).
func (s *Service) homeroomReviewerTargets(ctx context.Context, schoolID, academicYearID, studentID int64) []int64 {
	if uid, ok, err := s.repo.HomeroomTeacherUserID(ctx, schoolID, academicYearID, studentID); err == nil && ok {
		return []int64{uid}
	}
	ids, err := s.duties.UserIDsWithFlag(ctx, schoolID, FlagLeaveHomeroomReview)
	if err != nil {
		return nil
	}
	return ids
}

// -- GET /api/student-leave --

func validStatus(st string) bool {
	switch st {
	case StatusPendingHomeroom, StatusPendingBK, StatusIssued, StatusRejected, StatusCanceled:
		return true
	default:
		return false
	}
}

func sortByCreatedAtDesc(items []RequestDetail) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
}

// ListRequests — scope=mine|queue|all (docs tugas Fase 14 Gelombang B1).
func (s *Service) ListRequests(ctx context.Context, actorUserID, schoolID int64, scope, status string) (RequestPage, error) {
	role := reqctx.Role(ctx)
	if status != "" && !validStatus(status) {
		return RequestPage{}, httpx.Validation("status tidak dikenal: " + status)
	}

	var details []RequestDetail
	switch scope {
	case "", "mine":
		var studentIDs []int64
		switch role {
		case RoleSiswa:
			id, _, ok, err := s.students.MyStudentID(ctx, schoolID, actorUserID)
			if err != nil {
				return RequestPage{}, err
			}
			if ok {
				studentIDs = []int64{id}
			}
		case RoleOrangTua:
			ids, err := s.students.MyChildStudentIDs(ctx, schoolID, actorUserID)
			if err != nil {
				return RequestPage{}, err
			}
			studentIDs = ids
		}
		if len(studentIDs) == 0 {
			return RequestPage{Items: []RequestView{}}, nil
		}
		rows, err := s.repo.ListRequestsForStudents(ctx, schoolID, studentIDs, status)
		if err != nil {
			return RequestPage{}, err
		}
		details = rows

	case "queue":
		byID := make(map[int64]RequestDetail)
		homeroomRows, err := s.repo.ListRequestsForHomeroomTeacher(ctx, schoolID, actorUserID, StatusPendingHomeroom)
		if err != nil {
			return RequestPage{}, err
		}
		for _, r := range homeroomRows {
			byID[r.ID] = r
		}
		hasHomeroomFlag, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagLeaveHomeroomReview)
		if err != nil {
			return RequestPage{}, err
		}
		if hasHomeroomFlag {
			allHomeroom, err := s.repo.ListRequestsByStatus(ctx, schoolID, StatusPendingHomeroom)
			if err != nil {
				return RequestPage{}, err
			}
			for _, r := range allHomeroom {
				byID[r.ID] = r
			}
		}
		hasBKFlag, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagLeaveIssuance)
		if err != nil {
			return RequestPage{}, err
		}
		if hasBKFlag {
			allBK, err := s.repo.ListRequestsByStatus(ctx, schoolID, StatusPendingBK)
			if err != nil {
				return RequestPage{}, err
			}
			for _, r := range allBK {
				byID[r.ID] = r
			}
		}
		for _, r := range byID {
			if status != "" && r.Status != status {
				continue
			}
			details = append(details, r)
		}
		sortByCreatedAtDesc(details)

	case "all":
		if !s.identity.HasPermission(role, PermManageAll) {
			return RequestPage{}, httpx.ErrForbidden
		}
		rows, err := s.repo.ListRequestsByStatus(ctx, schoolID, status)
		if err != nil {
			return RequestPage{}, err
		}
		details = rows

	default:
		return RequestPage{}, httpx.Validation("scope harus 'mine', 'queue', atau 'all'.")
	}

	items := make([]RequestView, 0, len(details))
	for _, d := range details {
		items = append(items, d.View())
	}
	return RequestPage{Items: items, Total: int64(len(items))}, nil
}

// -- POST /api/student-leave/{id}/review (tahap wali kelas) --

var errNotPendingHomeroom = httpx.Validation("Pengajuan ini bukan/tidak lagi menunggu review wali kelas.")
var errAlreadyDecidedRace = httpx.Validation("Keputusan gagal disimpan (kemungkinan sudah diputuskan pihak lain). Muat ulang.")

func (s *Service) authorizeHomeroomReviewer(ctx context.Context, schoolID, academicYearID, studentID, actorUserID int64) error {
	if uid, ok, err := s.repo.HomeroomTeacherUserID(ctx, schoolID, academicYearID, studentID); err == nil && ok && uid == actorUserID {
		return nil
	}
	if ok, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagLeaveHomeroomReview); err == nil && ok {
		return nil
	}
	return httpx.ErrForbidden
}

func (s *Service) authorizeBKReviewer(ctx context.Context, schoolID, actorUserID int64) error {
	if ok, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagLeaveIssuance); err == nil && ok {
		return nil
	}
	return httpx.ErrForbidden
}

// ReviewHomeroom — POST /api/student-leave/{id}/review (tahap 1). approve ->
// pending_bk + notif pemegang flag leave_issuance; reject -> rejected +
// notif siswa+ortu.
func (s *Service) ReviewHomeroom(ctx context.Context, actorUserID, schoolID, requestID int64, decision, comment string) (RequestView, error) {
	if !validDecision(decision) {
		return RequestView{}, httpx.Validation("decision harus 'approve' atau 'reject'.")
	}
	detail, err := s.repo.GetRequestDetail(ctx, schoolID, requestID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return RequestView{}, httpx.ErrNotFound
		}
		return RequestView{}, err
	}
	if detail.Status != StatusPendingHomeroom {
		return RequestView{}, errNotPendingHomeroom
	}
	if err := s.authorizeHomeroomReviewer(ctx, schoolID, detail.AcademicYearID, detail.StudentID, actorUserID); err != nil {
		return RequestView{}, err
	}

	newStatus := StatusRejected
	if decision == DecisionApprove {
		newStatus = StatusPendingBK
	}
	comment = strings.TrimSpace(comment)
	rows, err := s.repo.UpdateHomeroomDecision(ctx, schoolID, requestID, newStatus, actorUserID, comment)
	if err != nil {
		return RequestView{}, err
	}
	if rows == 0 {
		return RequestView{}, errAlreadyDecidedRace
	}

	s.audit(ctx, schoolID, actorUserID, "studentleave.review", "student_leave_request", requestID,
		map[string]any{"status": StatusPendingHomeroom}, map[string]any{"status": newStatus, "decision": decision, "comment": comment})

	updated, err := s.repo.GetRequestDetail(ctx, schoolID, requestID)
	if err != nil {
		return RequestView{}, err
	}

	guardians, _ := s.students.GuardianUserIDs(ctx, schoolID, detail.StudentID)
	if decision == DecisionApprove {
		bkTargets, _ := s.duties.UserIDsWithFlag(ctx, schoolID, FlagLeaveIssuance)
		s.notify(ctx, schoolID, EventStudentLeaveForwarded, bkTargets, map[string]any{
			"student": detail.StudentName, "type": detail.Type,
		})
		s.emit(schoolID, requestID, dedupInt64(append([]int64{detail.SubmittedBy}, bkTargets...)...))
	} else {
		recipients := dedupInt64(append([]int64{detail.SubmittedBy}, guardians...)...)
		s.notify(ctx, schoolID, EventStudentLeaveDecided, recipients, map[string]any{
			"student": detail.StudentName, "type": detail.Type, "decision": "ditolak wali kelas",
		})
		s.emit(schoolID, requestID, recipients)
	}

	return updated.View(), nil
}

// -- POST /api/student-leave/{id}/issue (tahap BK) --

var errNotPendingBK = httpx.Validation("Pengajuan ini bukan/tidak lagi menunggu penerbitan BK.")

// verifyTokenChars/Length — 24 karakter alfanumerik (docs tugas).
const verifyTokenChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const verifyTokenLength = 24

func generateVerifyToken() (string, error) {
	b := make([]byte, verifyTokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, verifyTokenLength)
	for i, v := range b {
		out[i] = verifyTokenChars[int(v)%len(verifyTokenChars)]
	}
	return string(out), nil
}

func formatLetterNumber(year string, seq int32) string {
	return fmt.Sprintf("SI/%s/%04d", year, seq)
}

// IssueBK — POST /api/student-leave/{id}/issue (tahap 2, hanya pemegang flag
// leave_issuance). approve -> issued + nomor surat + verify_token + notif
// siswa+ortu+wali; reject -> rejected + notif siswa+ortu.
func (s *Service) IssueBK(ctx context.Context, actorUserID, schoolID, requestID int64, decision, comment string) (RequestView, error) {
	if !validDecision(decision) {
		return RequestView{}, httpx.Validation("decision harus 'approve' atau 'reject'.")
	}
	detail, err := s.repo.GetRequestDetail(ctx, schoolID, requestID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return RequestView{}, httpx.ErrNotFound
		}
		return RequestView{}, err
	}
	if detail.Status != StatusPendingBK {
		return RequestView{}, errNotPendingBK
	}
	if err := s.authorizeBKReviewer(ctx, schoolID, actorUserID); err != nil {
		return RequestView{}, err
	}

	comment = strings.TrimSpace(comment)
	newStatus := StatusRejected
	var letterNumber, verifyToken string
	if decision == DecisionApprove {
		newStatus = StatusIssued
		year := fmt.Sprintf("%d", s.clock.Now().Year())
		seq, err := s.repo.NextLetterSeq(ctx, schoolID, year)
		if err != nil {
			return RequestView{}, err
		}
		letterNumber = formatLetterNumber(year, seq)
		verifyToken, err = generateVerifyToken()
		if err != nil {
			return RequestView{}, err
		}
	}

	rows, err := s.repo.UpdateBKDecision(ctx, schoolID, requestID, newStatus, actorUserID, comment, letterNumber, verifyToken)
	if err != nil {
		return RequestView{}, err
	}
	if rows == 0 {
		return RequestView{}, errAlreadyDecidedRace
	}

	s.audit(ctx, schoolID, actorUserID, "studentleave.issue", "student_leave_request", requestID,
		map[string]any{"status": StatusPendingBK},
		map[string]any{"status": newStatus, "decision": decision, "comment": comment, "letter_number": letterNumber})

	updated, err := s.repo.GetRequestDetail(ctx, schoolID, requestID)
	if err != nil {
		return RequestView{}, err
	}

	guardians, _ := s.students.GuardianUserIDs(ctx, schoolID, detail.StudentID)
	recipients := dedupInt64(append([]int64{detail.SubmittedBy}, guardians...)...)
	decisionLabel := "diterbitkan"
	if decision == DecisionReject {
		decisionLabel = "ditolak BK"
	} else if homeroomUID, ok, herr := s.repo.HomeroomTeacherUserID(ctx, schoolID, detail.AcademicYearID, detail.StudentID); herr == nil && ok {
		recipients = dedupInt64(append(recipients, homeroomUID)...)
	}
	s.notify(ctx, schoolID, EventStudentLeaveDecided, recipients, map[string]any{
		"student": detail.StudentName, "type": detail.Type, "decision": decisionLabel,
	})
	s.emit(schoolID, requestID, recipients)

	return updated.View(), nil
}

// -- POST /api/student-leave/{id}/cancel --

func (s *Service) CancelRequest(ctx context.Context, actorUserID, schoolID, requestID int64) error {
	detail, err := s.repo.GetRequestDetail(ctx, schoolID, requestID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	if detail.SubmittedBy != actorUserID {
		return httpx.ErrForbidden
	}
	if !isPending(detail.Status) {
		return httpx.Validation("Hanya pengajuan yang masih menunggu keputusan yang bisa dibatalkan.")
	}
	rows, err := s.repo.CancelRequest(ctx, schoolID, requestID, actorUserID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.Validation("Pengajuan sudah tidak bisa dibatalkan (mungkin baru saja diputuskan).")
	}
	s.audit(ctx, schoolID, actorUserID, "studentleave.cancel", "student_leave_request", requestID,
		map[string]any{"status": detail.Status}, map[string]any{"status": StatusCanceled})
	s.emit(schoolID, requestID, []int64{actorUserID})
	return nil
}

// -- GET /api/student-leave/{id}/attachment & .../letter/pdf --

// authorizeView — pemilik/siswa-sendiri/orang-tua-anaknya/reviewer (wali
// kelas ATAU pemegang flag leave_homeroom_review/leave_issuance)/admin
// (student:manage) — pola yang sama dengan internal/leave.GetAttachment &
// internal/discipline.getLetterAuthorized.
func (s *Service) authorizeView(ctx context.Context, detail RequestDetail) error {
	role := reqctx.Role(ctx)
	actorID := reqctx.UserID(ctx)
	if detail.SubmittedBy == actorID {
		return nil
	}
	if s.identity.HasPermission(role, PermManageAll) {
		return nil
	}
	if err := s.students.CanViewStudent(ctx, actorID, role, detail.SchoolID, detail.StudentID); err == nil {
		return nil
	}
	if uid, ok, err := s.repo.HomeroomTeacherUserID(ctx, detail.SchoolID, detail.AcademicYearID, detail.StudentID); err == nil && ok && uid == actorID {
		return nil
	}
	if ok, err := s.duties.UserHasFlag(ctx, detail.SchoolID, actorID, FlagLeaveHomeroomReview); err == nil && ok {
		return nil
	}
	if ok, err := s.duties.UserHasFlag(ctx, detail.SchoolID, actorID, FlagLeaveIssuance); err == nil && ok {
		return nil
	}
	return httpx.ErrForbidden
}

// AttachmentInfo adalah metadata dipakai handler untuk stream file.
type AttachmentInfo struct {
	Path        string
	Name        string
	ContentType string
}

func (s *Service) GetAttachment(ctx context.Context, schoolID, requestID int64) (AttachmentInfo, error) {
	detail, err := s.repo.GetRequestDetail(ctx, schoolID, requestID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AttachmentInfo{}, httpx.ErrNotFound
		}
		return AttachmentInfo{}, err
	}
	if detail.Attachment == "" {
		return AttachmentInfo{}, httpx.ErrNotFound
	}
	if err := s.authorizeView(ctx, detail); err != nil {
		return AttachmentInfo{}, err
	}
	return AttachmentInfo{Path: detail.Attachment, Name: detail.AttachmentName, ContentType: detail.AttachmentMime}, nil
}

func (s *Service) OpenAttachment(path string) (io.ReadCloser, error) {
	return s.files.Open(path)
}

var errLetterNotIssued = httpx.Validation("Surat izin baru tersedia setelah diterbitkan BK.")

// leaveFooterNote membaca school_settings module "letters" ->
// leave_footer_note (Fase 14 Gelombang D, docs/12-sion-parity.md "template
// surat") — "" bila belum diset/gateway belum disuntik, DIBACA LIVE saat
// render (bukan bagian snapshot) supaya admin bisa mengubah catatan tanpa
// perlu menerbitkan ulang surat lama (pola sama internal/discipline.spFooterNote).
func (s *Service) leaveFooterNote(ctx context.Context, schoolID int64) string {
	if s.settings == nil {
		return ""
	}
	raw, found, err := s.settings.GetSetting(ctx, schoolID, "letters")
	if err != nil || !found {
		return ""
	}
	var v struct {
		LeaveFooterNote string `json:"leave_footer_note"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return v.LeaveFooterNote
}

// LetterPDF — GET /api/student-leave/{id}/letter/pdf (issued saja). host
// dipakai menyusun URL QR verifikasi (docs tugas: "host dari request").
func (s *Service) LetterPDF(ctx context.Context, schoolID, requestID int64, host string) ([]byte, string, error) {
	detail, err := s.repo.GetRequestDetail(ctx, schoolID, requestID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, "", httpx.ErrNotFound
		}
		return nil, "", err
	}
	if err := s.authorizeView(ctx, detail); err != nil {
		return nil, "", err
	}
	if detail.Status != StatusIssued {
		return nil, "", errLetterNotIssued
	}
	appName, err := s.branding.BrandingAppName(ctx, schoolID)
	if err != nil || appName == "" {
		appName = "NouSchool"
	}
	verifyURL := fmt.Sprintf("https://%s/verifikasi-surat?token=%s", host, detail.VerifyToken)
	data, err := BuildLeavePDF(detail, appName, verifyURL, s.leaveFooterNote(ctx, schoolID))
	if err != nil {
		return nil, "", err
	}
	return data, fmt.Sprintf("Surat Izin %s.pdf", sanitizeNumberForFilename(detail.LetterNumber)), nil
}

func sanitizeNumberForFilename(number string) string {
	return strings.ReplaceAll(number, "/", "-")
}

// -- GET /api/public/leave-verify?token= (PUBLIK) --

// PublicVerify — token tidak dikenal/salah -> {valid:false} TANPA detail
// (keputusan desain: dijawab 200 dgn payload minimal, BUKAN 404, supaya
// konsisten dengan konvensi envelope httpx.JSON yang dipakai SELURUH endpoint
// publik lain di codebase ini — mis. GET /api/public/context — lihat catatan
// di handler.go).
func (s *Service) PublicVerify(ctx context.Context, schoolID int64, token string) (VerifyView, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return VerifyView{Valid: false}, nil
	}
	rec, ok, err := s.repo.GetByVerifyToken(ctx, schoolID, token)
	if err != nil {
		return VerifyView{}, err
	}
	if !ok {
		return VerifyView{Valid: false}, nil
	}
	return VerifyView{
		Valid: true, LetterNumber: rec.LetterNumber, StudentName: rec.StudentName, ClassName: rec.ClassName,
		Type: rec.Type, DateStart: rec.DateStart.Format("2006-01-02"), DateEnd: rec.DateEnd.Format("2006-01-02"),
		IssuedAt: rec.IssuedAt.Format("2006-01-02"),
	}, nil
}
