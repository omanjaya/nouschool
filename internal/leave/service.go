package leave

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
	"github.com/omanjaya/nouschool/internal/platform/storage"
)

// -- consumer-side interface (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject lewat constructor di
// main.go). Signature sengaja hanya memakai tipe primitif supaya
// *identity.Service memenuhi interface ini secara struktural TANPA leave
// perlu mengimpor package itu.

// IdentityGateway adalah kebutuhan modul leave dari modul identity: gerbang
// permission untuk endpoint dengan otorisasi campuran, dan audit log.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
	// UsersWithRole (fase 9, docs/08-notification.md) — resolve penerima
	// notifikasi step approval role-only (approver_user_id NULL: "siapa pun
	// yang sedang memegang role ini"). Dipenuhi *identity.Service secara
	// struktural.
	UsersWithRole(ctx context.Context, schoolID int64, role string) ([]int64, error)
}

// Notifier adalah kebutuhan modul leave dari modul notification (fase 9,
// docs/08-notification.md) — consumer-side interface kecil dideklarasikan di
// sisi PEMAKAI (lihat CLAUDE.md). Dipenuhi *notification.Service secara
// struktural lewat method Notify (signature primitif).
type Notifier interface {
	Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error
}

// Event kanonik — nilai HARUS sama persis dengan notification.EventLeaveSubmitted
// / notification.EventLeaveDecided (didefinisikan ulang di sini karena leave
// TIDAK boleh mengimpor notification, lihat CLAUDE.md).
const (
	EventLeaveSubmitted = "leave.submitted"
	EventLeaveDecided   = "leave.decided"
)

// maxAttachmentSize — batas ukuran lampiran (docs/07-leave.md scope Fase 4: pdf/jpg/png max 5MB).
const maxAttachmentSize = 5 << 20

var allowedAttachmentTypes = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
}

// ErrAttachmentTooLarge / ErrAttachmentType — pesan validasi lampiran.
var (
	errAttachmentTooLarge = httpx.Validation("Ukuran lampiran maksimal 5MB.")
	errAttachmentType     = httpx.Validation("Lampiran harus berformat PDF, JPG, atau PNG.")
)

// AttachmentUpload adalah lampiran mentah dari multipart form (opsional).
type AttachmentUpload struct {
	Filename string
	Content  []byte
}

// Service berisi aturan bisnis modul leave.
type Service struct {
	repo     leaveRepository
	identity IdentityGateway
	files    *storage.Store
	clock    clock.Clock
	notifier Notifier // opsional — nil = notifikasi dilewati (lihat SetNotifier)
}

func NewService(repo *Repository, identity IdentityGateway, files *storage.Store, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	if files == nil {
		files = storage.FromEnv()
	}
	return &Service{repo: repo, identity: identity, files: files, clock: clk}
}

// SetNotifier menyuntikkan Notifier SETELAH konstruksi (opsional, disuntik
// main.go — pola yang sama dengan internal/attendance.SetNotifier, supaya
// call site test yang sudah ada tidak perlu diubah; nil aman/no-op).
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// notifyStep mengirim notifikasi ke approver satu step: user spesifik
// (ApproverUserID != 0) atau SEMUA user yang sedang memegang ApproverRole di
// sekolah ini (role-only step, lihat docs/07-leave.md). Fire-and-forget
// (error diabaikan) — pola yang sama dengan s.audit.
func (s *Service) notifyStep(ctx context.Context, schoolID int64, approverRole string, approverUserID int64, event string, data map[string]any) {
	if s.notifier == nil {
		return
	}
	var recipients []int64
	if approverUserID != 0 {
		recipients = []int64{approverUserID}
	} else {
		ids, err := s.identity.UsersWithRole(ctx, schoolID, approverRole)
		if err != nil || len(ids) == 0 {
			return
		}
		recipients = ids
	}
	_ = s.notifier.Notify(ctx, schoolID, event, recipients, data)
}

// notifyUser mengirim notifikasi ke SATU user (dipakai leave.decided ke
// pengaju). Fire-and-forget.
func (s *Service) notifyUser(ctx context.Context, schoolID, userID int64, event string, data map[string]any) {
	if s.notifier == nil || userID == 0 {
		return
	}
	_ = s.notifier.Notify(ctx, schoolID, event, []int64{userID}, data)
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo leaveRepository, identity IdentityGateway, files *storage.Store, clk clock.Clock) *Service {
	if files == nil {
		files = storage.New("")
	}
	return &Service{repo: repo, identity: identity, files: files, clock: clk}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

// -- tanggal lokal sekolah (sama seperti internal/attendance) --

func schoolToday(now time.Time, tz string) time.Time {
	local := clock.InZone(now, tz)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func parseDate(raw string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	return t, nil
}

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

// -- GET /api/leave/types --

func (s *Service) ListTypes(ctx context.Context, schoolID int64) ([]LeaveTypeView, error) {
	settings, err := s.repo.GetSettings(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]LeaveTypeView, 0, len(settings.Types))
	for _, t := range settings.Types {
		out = append(out, LeaveTypeView{Key: t.Key, Label: t.Label})
	}
	return out, nil
}

// -- isStepApprover: predikat TUNGGAL dipakai baik untuk filter self-approval
// saat submit maupun otorisasi antrian/keputusan saat decide (lihat
// docs/07-leave.md & catatan keputusan desain di SubmitRequest) --

func isStepApprover(approverUserID int64, approverRole string, actorUserID int64, actorRole string) bool {
	if approverUserID != 0 {
		return approverUserID == actorUserID
	}
	return approverRole == actorRole
}

// -- POST /api/leave/requests --

// SubmitInput adalah parameter Service.SubmitRequest (bentuk sudah divalidasi
// handler, aturan bisnis divalidasi di sini — lihat CLAUDE.md).
type SubmitInput struct {
	Type       string
	DateStart  string
	DateEnd    string
	Reason     string
	Attachment *AttachmentUpload
}

// detectAttachmentExt men-sniff content type SEBENARNYA dari isi file
// (bukan percaya begitu saja Content-Type dari klien) dan menolak tipe di
// luar pdf/jpg/png.
func detectAttachmentExt(content []byte) (string, string, error) {
	if len(content) > maxAttachmentSize {
		return "", "", errAttachmentTooLarge
	}
	mime := http.DetectContentType(content)
	// http.DetectContentType mengembalikan "image/jpeg", "image/png", atau
	// utk PDF "application/pdf" (didukung sejak Go stdlib mengenali magic byte %PDF).
	for prefix, ext := range allowedAttachmentTypes {
		if mime == prefix {
			return mime, ext, nil
		}
	}
	return "", "", errAttachmentType
}

// SubmitRequest membuat pengajuan izin baru + SNAPSHOT chain approval dari
// settings ke leave_approval_steps (docs/07-leave.md). Step yang approver-nya
// adalah pengaju sendiri di-skip (lihat isStepApprover); bila SEMUA step
// ter-skip, request otomatis approved + audit "auto_approved_self_chain".
func (s *Service) SubmitRequest(ctx context.Context, actorUserID, schoolID int64, in SubmitInput) (Request, error) {
	actorRole := reqctx.Role(ctx)

	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return Request{}, httpx.Validation("Alasan wajib diisi.")
	}

	dateStart, err := parseDate(in.DateStart)
	if err != nil {
		return Request{}, err
	}
	dateEnd, err := parseDate(in.DateEnd)
	if err != nil {
		return Request{}, err
	}
	if dateStart.After(dateEnd) {
		return Request{}, httpx.Validation("Tanggal mulai tidak boleh setelah tanggal selesai.")
	}
	today := schoolToday(s.clock.Now(), schoolTimezone(ctx))
	if dateStart.Before(today.AddDate(0, 0, -30)) {
		return Request{}, httpx.Validation("Tanggal mulai tidak boleh lebih dari 30 hari ke belakang.")
	}

	settings, err := s.repo.GetSettings(ctx, schoolID)
	if err != nil {
		return Request{}, err
	}
	typeOK := false
	for _, t := range settings.Types {
		if t.Key == in.Type {
			typeOK = true
			break
		}
	}
	if !typeOK {
		return Request{}, httpx.Validation("Tipe izin tidak dikenal: " + in.Type)
	}

	var attachment, attachmentName, attachmentMime string
	if in.Attachment != nil {
		mime, ext, err := detectAttachmentExt(in.Attachment.Content)
		if err != nil {
			return Request{}, err
		}
		filename, err := storage.RandomFilename(ext)
		if err != nil {
			return Request{}, err
		}
		relPath := fmt.Sprintf("%d/leave/%s", schoolID, filename)
		if err := s.files.Save(relPath, bytes.NewReader(in.Attachment.Content)); err != nil {
			return Request{}, err
		}
		attachment = relPath
		attachmentName = in.Attachment.Filename
		attachmentMime = mime
	}

	// Snapshot chain -> steps, skip step yang approver-nya pengaju sendiri
	// (lihat isStepApprover; predikat yang SAMA dipakai antrian & decide,
	// sehingga "siapa approver step ini" konsisten di seluruh siklus hidup
	// request — lihat catatan keputusan di laporan akhir tugas).
	steps := make([]StepPlan, 0, len(settings.Chain))
	order := int32(1)
	for _, c := range settings.Chain {
		if isStepApprover(c.UserID, c.Role, actorUserID, actorRole) {
			continue
		}
		steps = append(steps, StepPlan{StepOrder: order, ApproverRole: c.Role, ApproverUserID: c.UserID})
		order++
	}

	rec, stepRecs, err := s.repo.SubmitRequest(ctx, SubmitRequestParams{
		SchoolID: schoolID, TeacherID: actorUserID, Type: in.Type,
		DateStart: dateStart, DateEnd: dateEnd, Reason: reason,
		Attachment: attachment, AttachmentName: attachmentName, AttachmentMime: attachmentMime,
		Steps: steps, AutoApprove: len(steps) == 0,
	})
	if err != nil {
		return Request{}, err
	}

	if len(steps) == 0 {
		s.audit(ctx, schoolID, actorUserID, "leave.auto_approved_self_chain", "leave_request", rec.ID, nil,
			map[string]any{"status": StatusApproved})
	}
	s.audit(ctx, schoolID, actorUserID, "leave.submit", "leave_request", rec.ID, nil,
		map[string]any{"type": in.Type, "date_start": in.DateStart, "date_end": in.DateEnd, "status": rec.Status})

	// rec.TeacherName kosong (tidak di-join saat insert) — buildRequestView
	// memuat ulang lewat GetRequestByID untuk melengkapi nama pengaju.
	view, err := s.buildRequestView(ctx, schoolID, rec, stepRecs, settings)
	if err != nil {
		return Request{}, err
	}

	// Notify approver step AKTIF (step_order 1 di antara step yang tersisa
	// setelah filter self-approval) — fase 9, docs/08-notification.md
	// "leave.submitted -> approver step aktif". Request auto-approved (steps
	// kosong) TIDAK punya approver aktif -> tidak ada yang dinotifikasi.
	if len(steps) > 0 {
		s.notifyStep(ctx, schoolID, steps[0].ApproverRole, steps[0].ApproverUserID, EventLeaveSubmitted, map[string]any{
			"teacher": view.Teacher.Name, "type": view.TypeLabel,
			"date_start": view.DateStart.Format("2006-01-02"), "date_end": view.DateEnd.Format("2006-01-02"),
		})
	}

	return view, nil
}

// -- shape building --

func daysInclusive(start, end time.Time) int {
	return int(end.Sub(start).Hours()/24) + 1
}

func typeLabel(settings Settings, key string) string {
	for _, t := range settings.Types {
		if t.Key == key {
			return t.Label
		}
	}
	return key
}

func (s *Service) buildRequestView(ctx context.Context, schoolID int64, rec RequestRecord, stepRecs []StepRecord, settings Settings) (Request, error) {
	if rec.TeacherName == "" {
		full, err := s.repo.GetRequestByID(ctx, schoolID, rec.ID)
		if err != nil {
			return Request{}, err
		}
		rec = full
	}
	return requestView(rec, stepRecs, settings), nil
}

func requestView(rec RequestRecord, stepRecs []StepRecord, settings Settings) Request {
	var attachmentURL *string
	if rec.Attachment != "" {
		u := fmt.Sprintf("/api/files/leave/%d/attachment", rec.ID)
		attachmentURL = &u
	}
	steps := make([]StepView, 0, len(stepRecs))
	for _, st := range stepRecs {
		sv := StepView{ID: st.ID, StepOrder: int(st.StepOrder), ApproverRole: st.ApproverRole}
		if st.Decision != "" {
			d := st.Decision
			sv.Decision = &d
			sv.DecidedAt = st.DecidedAt
			if st.DecidedByName != "" {
				n := st.DecidedByName
				sv.ApproverName = &n
			}
		}
		if st.Comment != "" {
			c := st.Comment
			sv.Comment = &c
		}
		steps = append(steps, sv)
	}
	return Request{
		ID: rec.ID, Type: rec.Type, TypeLabel: typeLabel(settings, rec.Type),
		DateStart: NewDate(rec.DateStart), DateEnd: NewDate(rec.DateEnd), Days: daysInclusive(rec.DateStart, rec.DateEnd),
		Reason: rec.Reason, Status: rec.Status, CreatedAt: rec.CreatedAt,
		Teacher:       TeacherView{ID: rec.TeacherID, Name: rec.TeacherName},
		AttachmentURL: attachmentURL, Steps: steps,
	}
}

// -- GET /api/leave/requests --

// ListRequests — scope "mine" (default) mengembalikan milik sendiri; scope
// "all" butuh perm leave:manage (docs/07-leave.md scope Fase 4).
func (s *Service) ListRequests(ctx context.Context, schoolID, actorUserID int64, scope, status string) ([]Request, error) {
	role := reqctx.Role(ctx)
	var teacherFilter int64
	switch scope {
	case "", "mine":
		teacherFilter = actorUserID
	case "all":
		if !s.identity.HasPermission(role, PermLeaveManage) {
			return nil, httpx.ErrForbidden
		}
	default:
		return nil, httpx.Validation("scope harus 'mine' atau 'all'.")
	}
	if status != "" && !validStatus(status) {
		return nil, httpx.Validation("status tidak dikenal: " + status)
	}

	recs, err := s.repo.ListRequests(ctx, schoolID, teacherFilter, status)
	if err != nil {
		return nil, err
	}
	return s.buildRequestViews(ctx, schoolID, recs)
}

func validStatus(st string) bool {
	switch st {
	case StatusPending, StatusApproved, StatusRejected, StatusCanceled:
		return true
	default:
		return false
	}
}

// buildRequestViews memuat settings SEKALI + steps utk semua request SEKALI
// (batch, hindari N+1) lalu merakit shape Request per baris.
func (s *Service) buildRequestViews(ctx context.Context, schoolID int64, recs []RequestRecord) ([]Request, error) {
	settings, err := s.repo.GetSettings(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(recs))
	for _, r := range recs {
		ids = append(ids, r.ID)
	}
	steps, err := s.repo.ListStepsForRequests(ctx, ids)
	if err != nil {
		return nil, err
	}
	stepsByRequest := make(map[int64][]StepRecord, len(recs))
	for _, st := range steps {
		stepsByRequest[st.LeaveRequestID] = append(stepsByRequest[st.LeaveRequestID], st)
	}
	out := make([]Request, 0, len(recs))
	for _, r := range recs {
		out = append(out, requestView(r, stepsByRequest[r.ID], settings))
	}
	return out, nil
}

// -- POST /api/leave/requests/{id}/cancel --

func (s *Service) CancelRequest(ctx context.Context, actorUserID, schoolID, requestID int64) error {
	rec, err := s.repo.GetRequestByID(ctx, schoolID, requestID)
	if err != nil {
		return err
	}
	if rec.TeacherID != actorUserID {
		return httpx.ErrForbidden
	}
	if rec.Status != StatusPending {
		return httpx.Validation("Hanya pengajuan berstatus pending yang bisa dibatalkan.")
	}
	rows, err := s.repo.CancelRequest(ctx, schoolID, requestID, actorUserID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.Validation("Pengajuan sudah tidak pending (mungkin baru saja diputuskan).")
	}
	s.audit(ctx, schoolID, actorUserID, "leave.cancel", "leave_request", requestID, map[string]any{"status": StatusPending},
		map[string]any{"status": StatusCanceled})
	return nil
}

// -- GET /api/leave/approvals --

func (s *Service) ListApprovals(ctx context.Context, schoolID, actorUserID int64) ([]QueueItem, error) {
	role := reqctx.Role(ctx)
	steps, err := s.repo.ListApprovalQueue(ctx, schoolID, actorUserID, role)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return []QueueItem{}, nil
	}
	ids := make([]int64, 0, len(steps))
	seen := make(map[int64]bool, len(steps))
	for _, st := range steps {
		if !seen[st.LeaveRequestID] {
			seen[st.LeaveRequestID] = true
			ids = append(ids, st.LeaveRequestID)
		}
	}
	recs := make([]RequestRecord, 0, len(ids))
	for _, id := range ids {
		rec, err := s.repo.GetRequestByID(ctx, schoolID, id)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rec)
	}
	views, err := s.buildRequestViews(ctx, schoolID, recs)
	if err != nil {
		return nil, err
	}
	viewByID := make(map[int64]Request, len(views))
	for _, v := range views {
		viewByID[v.ID] = v
	}
	out := make([]QueueItem, 0, len(steps))
	for _, st := range steps {
		out = append(out, QueueItem{StepID: st.ID, Request: viewByID[st.LeaveRequestID]})
	}
	return out, nil
}

// -- POST /api/leave/approvals/{step_id}/decide --

var errStepAlreadyDecided = httpx.Validation("Step ini sudah diputuskan sebelumnya.")
var errStepNotActive = httpx.Validation("Menunggu keputusan step sebelumnya terlebih dahulu.")
var errRequestNotPending = httpx.Validation("Pengajuan ini sudah tidak pending.")

// DecideStep menegakkan aturan engine (docs/07-leave.md): step diputuskan
// berurutan, approver tidak boleh memutuskan request miliknya sendiri, dan
// status request adalah derivasi dari step-step-nya.
func (s *Service) DecideStep(ctx context.Context, actorUserID, schoolID, stepID int64, decision, comment string) (Request, error) {
	if decision != DecisionApproved && decision != DecisionRejected {
		return Request{}, httpx.Validation("decision harus 'approved' atau 'rejected'.")
	}
	actorRole := reqctx.Role(ctx)

	step, err := s.repo.GetStepByID(ctx, schoolID, stepID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Request{}, httpx.ErrNotFound
		}
		return Request{}, err
	}
	req, err := s.repo.GetRequestByID(ctx, schoolID, step.LeaveRequestID)
	if err != nil {
		return Request{}, err
	}
	if req.TeacherID == actorUserID {
		return Request{}, httpx.ErrForbidden
	}
	if req.Status != StatusPending {
		return Request{}, errRequestNotPending
	}
	if step.Decision != "" {
		return Request{}, errStepAlreadyDecided
	}
	if !isStepApprover(step.ApproverUserID, step.ApproverRole, actorUserID, actorRole) {
		return Request{}, httpx.ErrForbidden
	}

	allSteps, err := s.repo.ListStepsForRequest(ctx, step.LeaveRequestID)
	if err != nil {
		return Request{}, err
	}
	maxOrder := int32(0)
	for _, st := range allSteps {
		if st.Decision == "" && st.StepOrder < step.StepOrder {
			return Request{}, errStepNotActive
		}
		if st.StepOrder > maxOrder {
			maxOrder = st.StepOrder
		}
	}

	newReqStatus := ""
	switch decision {
	case DecisionRejected:
		newReqStatus = StatusRejected
	case DecisionApproved:
		if step.StepOrder == maxOrder {
			newReqStatus = StatusApproved
		}
	}

	_, updatedReq, err := s.repo.DecideStep(ctx, DecideStepInput{
		SchoolID: schoolID, StepID: stepID, DeciderID: actorUserID, Decision: decision, Comment: strings.TrimSpace(comment),
		RequestID: step.LeaveRequestID, NewReqStatus: newReqStatus,
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return Request{}, httpx.Validation("Keputusan gagal disimpan (kemungkinan sudah diputuskan pihak lain). Muat ulang.")
		}
		return Request{}, err
	}

	s.audit(ctx, schoolID, actorUserID, "leave.decide", "leave_approval_step", stepID,
		map[string]any{"decision": nil}, map[string]any{"decision": decision, "request_status": updatedReq.Status})

	settings, err := s.repo.GetSettings(ctx, schoolID)
	if err != nil {
		return Request{}, err
	}
	steps, err := s.repo.ListStepsForRequest(ctx, step.LeaveRequestID)
	if err != nil {
		return Request{}, err
	}
	view, err := s.buildRequestView(ctx, schoolID, updatedReq, steps, settings)
	if err != nil {
		return Request{}, err
	}

	// Notify pengaju SELALU (fase 9, docs/08-notification.md "leave.decided
	// -> pengaju"), lalu approver step berikutnya BILA masih ada (chain
	// belum selesai — updatedReq masih pending berarti step approved TAPI
	// bukan step_order terakhir).
	decisionLabel := "disetujui"
	if decision == DecisionRejected {
		decisionLabel = "ditolak"
	}
	notifyData := map[string]any{
		"type": view.TypeLabel, "date_start": view.DateStart.Format("2006-01-02"), "date_end": view.DateEnd.Format("2006-01-02"),
	}
	s.notifyUser(ctx, schoolID, req.TeacherID, EventLeaveDecided, map[string]any{
		"type": notifyData["type"], "date_start": notifyData["date_start"], "date_end": notifyData["date_end"], "decision": decisionLabel,
	})
	if updatedReq.Status == StatusPending {
		for _, st := range steps {
			if st.Decision == "" {
				s.notifyStep(ctx, schoolID, st.ApproverRole, st.ApproverUserID, EventLeaveSubmitted, map[string]any{
					"teacher": view.Teacher.Name, "type": notifyData["type"], "date_start": notifyData["date_start"], "date_end": notifyData["date_end"],
				})
				break
			}
		}
	}

	return view, nil
}

// -- GET /api/leave/summary --

// Summary — perm leave:manage ATAU role kepala_sekolah (docs/07-leave.md).
func (s *Service) Summary(ctx context.Context, schoolID int64, fromStr, toStr string) ([]SummaryRow, error) {
	role := reqctx.Role(ctx)
	if !s.identity.HasPermission(role, PermLeaveManage) && role != RoleKepalaSekolah {
		return nil, httpx.ErrForbidden
	}
	from, err := parseDate(fromStr)
	if err != nil {
		return nil, err
	}
	to, err := parseDate(toStr)
	if err != nil {
		return nil, err
	}
	if from.After(to) {
		return nil, httpx.Validation("'from' tidak boleh setelah 'to'.")
	}

	rows, err := s.repo.Summary(ctx, schoolID, from, to)
	if err != nil {
		return nil, err
	}
	order := make([]int64, 0)
	byTeacher := make(map[int64]*SummaryRow)
	for _, row := range rows {
		sr, ok := byTeacher[row.TeacherID]
		if !ok {
			sr = &SummaryRow{TeacherID: row.TeacherID, Name: row.TeacherName, PerType: map[string]int{}}
			byTeacher[row.TeacherID] = sr
			order = append(order, row.TeacherID)
		}
		sr.PerType[row.Type] += int(row.Days)
		sr.TotalDays += int(row.Days)
	}
	out := make([]SummaryRow, 0, len(order))
	for _, id := range order {
		out = append(out, *byTeacher[id])
	}
	return out, nil
}

// -- GET /api/files/leave/{id}/attachment --

// AttachmentInfo adalah metadata yang dibutuhkan handler untuk stream file.
type AttachmentInfo struct {
	Path        string
	Name        string
	ContentType string
}

// GetAttachment menegakkan otorisasi (pengaju, approver DI CHAIN request ini
// — baik step aktif maupun tidak, atau leave:manage) lalu mengembalikan
// metadata file untuk di-stream handler.
func (s *Service) GetAttachment(ctx context.Context, actorUserID, schoolID, requestID int64) (AttachmentInfo, error) {
	role := reqctx.Role(ctx)
	rec, err := s.repo.GetRequestByID(ctx, schoolID, requestID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AttachmentInfo{}, httpx.ErrNotFound
		}
		return AttachmentInfo{}, err
	}
	if rec.Attachment == "" {
		return AttachmentInfo{}, httpx.ErrNotFound
	}

	allowed := rec.TeacherID == actorUserID || s.identity.HasPermission(role, PermLeaveManage)
	if !allowed {
		steps, err := s.repo.ListStepsForRequest(ctx, requestID)
		if err != nil {
			return AttachmentInfo{}, err
		}
		for _, st := range steps {
			if isStepApprover(st.ApproverUserID, st.ApproverRole, actorUserID, role) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return AttachmentInfo{}, httpx.ErrForbidden
	}

	return AttachmentInfo{Path: rec.Attachment, Name: rec.AttachmentName, ContentType: rec.AttachmentMime}, nil
}

// Open membuka file lampiran untuk di-stream handler (pemanggil wajib Close).
func (s *Service) OpenAttachment(path string) (io.ReadCloser, error) {
	return s.files.Open(path)
}

// -- interface publik fase 6: ApprovedOn --

// ApprovedOn melaporkan apakah guru (teacherUserID) punya izin approved pada
// tanggal date (YYYY-MM-DD) di sekolah schoolID — dipakai modul teaching
// (fase 6) lewat consumer-side interface LeaveChecker (docs/07-leave.md).
func (s *Service) ApprovedOn(ctx context.Context, schoolID, teacherUserID int64, date string) (bool, error) {
	d, err := parseDate(date)
	if err != nil {
		return false, err
	}
	return s.repo.ApprovedOn(ctx, schoolID, teacherUserID, d)
}
