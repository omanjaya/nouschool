package leave

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/leave/leavedb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul leave.
var ErrNotFound = errors.New("leave: data tidak ditemukan")

// ErrConflict menandai race optimistic (mis. step sudah diputuskan pihak
// lain, atau request sudah tidak pending lagi) — dipetakan Service ke pesan
// yang aman ditampilkan ke pengguna.
var ErrConflict = errors.New("leave: data sudah berubah, muat ulang")

// leaveRepository adalah kontrak yang dibutuhkan Service dari repository —
// dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB — pola
// yang sama dengan internal/attendance.
type leaveRepository interface {
	GetSettings(ctx context.Context, schoolID int64) (Settings, error)

	SubmitRequest(ctx context.Context, in SubmitRequestParams) (RequestRecord, []StepRecord, error)
	GetRequestByID(ctx context.Context, schoolID, id int64) (RequestRecord, error)
	ListRequests(ctx context.Context, schoolID int64, teacherID int64, status string) ([]RequestRecord, error)
	ListStepsForRequest(ctx context.Context, requestID int64) ([]StepRecord, error)
	ListStepsForRequests(ctx context.Context, requestIDs []int64) ([]StepRecord, error)

	CancelRequest(ctx context.Context, schoolID, id, teacherID int64) (int64, error)

	GetStepByID(ctx context.Context, schoolID, stepID int64) (StepRecord, error)
	ListApprovalQueue(ctx context.Context, schoolID, userID int64, role string) ([]StepRecord, error)
	DecideStep(ctx context.Context, in DecideStepInput) (StepRecord, RequestRecord, error)

	Summary(ctx context.Context, schoolID int64, from, to time.Time) ([]SummaryRowRaw, error)
	ApprovedOn(ctx context.Context, schoolID, teacherUserID int64, date time.Time) (bool, error)
}

var _ leaveRepository = (*Repository)(nil)

// Repository membungkus akses data modul leave (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *leavedb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: leavedb.New(pool)}
}

func mapNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func textOrNil(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func dateOf(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func int8OrNil(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func int8ToInt64(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func textToPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func timestamptzToPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

// -- domain records --

// RequestRecord adalah representasi domain satu baris leave_requests (join
// dengan nama guru pengaju).
type RequestRecord struct {
	ID             int64
	SchoolID       int64
	TeacherID      int64
	TeacherName    string
	Type           string
	DateStart      time.Time
	DateEnd        time.Time
	Reason         string
	Attachment     string
	AttachmentName string
	AttachmentMime string
	Status         string
	CreatedAt      time.Time
}

type StepRecord struct {
	ID             int64
	SchoolID       int64
	LeaveRequestID int64
	StepOrder      int32
	ApproverRole   string
	ApproverUserID int64 // 0 = role-only (siapa pun dengan role ini)
	DecidedBy      int64 // 0 = belum diputuskan
	DecidedByName  string
	Decision       string // "" = belum diputuskan
	DecidedAt      *time.Time
	Comment        string
}

type SummaryRowRaw struct {
	TeacherID   int64
	TeacherName string
	Type        string
	Days        int64
}

func requestFromRow(id, schoolID, teacherID int64, teacherName, typ string, dateStart, dateEnd pgtype.Date,
	reason string, attachment, attachmentName, attachmentMime pgtype.Text, status string, createdAt pgtype.Timestamptz) RequestRecord {
	return RequestRecord{
		ID: id, SchoolID: schoolID, TeacherID: teacherID, TeacherName: teacherName, Type: typ,
		DateStart: dateStart.Time, DateEnd: dateEnd.Time, Reason: reason,
		Attachment: attachment.String, AttachmentName: attachmentName.String, AttachmentMime: attachmentMime.String,
		Status: status, CreatedAt: createdAt.Time,
	}
}

func stepFromDB(s leavedb.LeaveApprovalStep, decidedByName string) StepRecord {
	return StepRecord{
		ID: s.ID, SchoolID: s.SchoolID, LeaveRequestID: s.LeaveRequestID, StepOrder: s.StepOrder,
		ApproverRole: s.ApproverRole, ApproverUserID: int8ToInt64(s.ApproverUserID),
		DecidedBy: int8ToInt64(s.DecidedBy), DecidedByName: decidedByName,
		Decision: s.Decision.String, DecidedAt: timestamptzToPtr(s.DecidedAt), Comment: s.Comment.String,
	}
}

// -- settings --

// GetSettings membaca school_settings module='leave' langsung (tabel & pola
// yang sama dipakai tenant.SettingsService — lihat catatan di queries.sql)
// dan mengembalikan default bila sekolah belum pernah menyimpan pengaturannya.
func (r *Repository) GetSettings(ctx context.Context, schoolID int64) (Settings, error) {
	raw, err := r.q.GetLeaveSettingsRaw(ctx, schoolID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DefaultSettings(), nil
		}
		return Settings{}, err
	}
	out := DefaultSettings()
	if err := json.Unmarshal(raw, &out); err != nil {
		return Settings{}, err
	}
	return out, nil
}

// -- submit --

// StepPlan adalah satu step yang akan ditulis saat submit (setelah filter
// self-approval — lihat Service.SubmitRequest).
type StepPlan struct {
	StepOrder      int32
	ApproverRole   string
	ApproverUserID int64
}

type SubmitRequestParams struct {
	SchoolID       int64
	TeacherID      int64
	Type           string
	DateStart      time.Time
	DateEnd        time.Time
	Reason         string
	Attachment     string
	AttachmentName string
	AttachmentMime string
	Steps          []StepPlan
	AutoApprove    bool // true bila Steps kosong (seluruh chain ter-skip self-approval)
}

// SubmitRequest menulis leave_requests + snapshot leave_approval_steps dalam
// satu transaksi (docs/07-leave.md: "steps di-SNAPSHOT saat pengajuan").
func (r *Repository) SubmitRequest(ctx context.Context, in SubmitRequestParams) (RequestRecord, []StepRecord, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RequestRecord{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	status := StatusPending
	if in.AutoApprove {
		status = StatusApproved
	}

	req, err := qtx.CreateLeaveRequest(ctx, leavedb.CreateLeaveRequestParams{
		SchoolID: in.SchoolID, TeacherID: in.TeacherID, Type: in.Type,
		DateStart: dateOf(in.DateStart), DateEnd: dateOf(in.DateEnd), Reason: in.Reason,
		Attachment: textOrNil(in.Attachment), AttachmentName: textOrNil(in.AttachmentName), AttachmentMime: textOrNil(in.AttachmentMime),
		Status: status,
	})
	if err != nil {
		return RequestRecord{}, nil, err
	}

	steps := make([]StepRecord, 0, len(in.Steps))
	for _, sp := range in.Steps {
		row, err := qtx.CreateLeaveApprovalStep(ctx, leavedb.CreateLeaveApprovalStepParams{
			SchoolID: in.SchoolID, LeaveRequestID: req.ID, StepOrder: sp.StepOrder,
			ApproverRole: sp.ApproverRole, ApproverUserID: int8OrNil(sp.ApproverUserID),
		})
		if err != nil {
			return RequestRecord{}, nil, err
		}
		steps = append(steps, stepFromDB(row, ""))
	}

	if err := tx.Commit(ctx); err != nil {
		return RequestRecord{}, nil, err
	}

	return requestFromRow(req.ID, req.SchoolID, req.TeacherID, "", req.Type, req.DateStart, req.DateEnd,
		req.Reason, req.Attachment, req.AttachmentName, req.AttachmentMime, req.Status, req.CreatedAt), steps, nil
}

// -- read --

func (r *Repository) GetRequestByID(ctx context.Context, schoolID, id int64) (RequestRecord, error) {
	row, err := r.q.GetLeaveRequestByID(ctx, leavedb.GetLeaveRequestByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return RequestRecord{}, mapNoRows(err)
	}
	return requestFromRow(row.ID, row.SchoolID, row.TeacherID, row.TeacherName, row.Type, row.DateStart, row.DateEnd,
		row.Reason, row.Attachment, row.AttachmentName, row.AttachmentMime, row.Status, row.CreatedAt), nil
}

func (r *Repository) ListRequests(ctx context.Context, schoolID int64, teacherID int64, status string) ([]RequestRecord, error) {
	rows, err := r.q.ListLeaveRequests(ctx, leavedb.ListLeaveRequestsParams{
		SchoolID: schoolID, TeacherID: int8OrNil(teacherID), Status: textOrNil(status),
	})
	if err != nil {
		return nil, err
	}
	out := make([]RequestRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, requestFromRow(row.ID, row.SchoolID, row.TeacherID, row.TeacherName, row.Type, row.DateStart, row.DateEnd,
			row.Reason, row.Attachment, row.AttachmentName, row.AttachmentMime, row.Status, row.CreatedAt))
	}
	return out, nil
}

func (r *Repository) ListStepsForRequest(ctx context.Context, requestID int64) ([]StepRecord, error) {
	rows, err := r.q.ListLeaveStepsForRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	out := make([]StepRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, stepFromDB(leavedb.LeaveApprovalStep{
			ID: row.ID, SchoolID: row.SchoolID, LeaveRequestID: row.LeaveRequestID, StepOrder: row.StepOrder,
			ApproverRole: row.ApproverRole, ApproverUserID: row.ApproverUserID, DecidedBy: row.DecidedBy,
			Decision: row.Decision, DecidedAt: row.DecidedAt, Comment: row.Comment,
		}, row.DecidedByName.String))
	}
	return out, nil
}

func (r *Repository) ListStepsForRequests(ctx context.Context, requestIDs []int64) ([]StepRecord, error) {
	if len(requestIDs) == 0 {
		return nil, nil
	}
	rows, err := r.q.ListLeaveStepsByRequestIDs(ctx, requestIDs)
	if err != nil {
		return nil, err
	}
	out := make([]StepRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, stepFromDB(leavedb.LeaveApprovalStep{
			ID: row.ID, SchoolID: row.SchoolID, LeaveRequestID: row.LeaveRequestID, StepOrder: row.StepOrder,
			ApproverRole: row.ApproverRole, ApproverUserID: row.ApproverUserID, DecidedBy: row.DecidedBy,
			Decision: row.Decision, DecidedAt: row.DecidedAt, Comment: row.Comment,
		}, row.DecidedByName.String))
	}
	return out, nil
}

// -- cancel --

// CancelRequest membatalkan request bila milik teacherID DAN masih pending
// (guard di WHERE, bukan cek terpisah — race-safe). Mengembalikan jumlah
// baris terpengaruh: 0 berarti tidak ditemukan/bukan milik/tidak pending
// (Service yang menerjemahkan ke pesan spesifik).
func (r *Repository) CancelRequest(ctx context.Context, schoolID, id, teacherID int64) (int64, error) {
	return r.q.CancelLeaveRequestGuarded(ctx, leavedb.CancelLeaveRequestGuardedParams{
		ID: id, SchoolID: schoolID, TeacherID: teacherID,
	})
}

// -- approval --

func (r *Repository) GetStepByID(ctx context.Context, schoolID, stepID int64) (StepRecord, error) {
	row, err := r.q.GetLeaveStepByID(ctx, leavedb.GetLeaveStepByIDParams{ID: stepID, SchoolID: schoolID})
	if err != nil {
		return StepRecord{}, mapNoRows(err)
	}
	return stepFromDB(row, ""), nil
}

func (r *Repository) ListApprovalQueue(ctx context.Context, schoolID, userID int64, role string) ([]StepRecord, error) {
	rows, err := r.q.ListLeaveApprovalQueue(ctx, leavedb.ListLeaveApprovalQueueParams{
		SchoolID: schoolID, UserID: userID, Role: role,
	})
	if err != nil {
		return nil, err
	}
	out := make([]StepRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, stepFromDB(row, ""))
	}
	return out, nil
}

// DecideStepInput adalah parameter Repository.DecideStep.
type DecideStepInput struct {
	SchoolID     int64
	StepID       int64
	DeciderID    int64
	Decision     string
	Comment      string
	RequestID    int64
	NewReqStatus string // "" = request TIDAK berubah status (masih pending, menunggu step berikutnya)
}

// DecideStep menulis keputusan step, dan (bila NewReqStatus != "") status
// request, dalam satu transaksi. Guard "decision IS NULL" pada step dan
// "status = 'pending'" pada request mencegah race (dua approver memutuskan
// step yang sama bersamaan, atau memutuskan setelah request sudah
// final/dibatalkan) — bila guard gagal, mengembalikan ErrConflict.
func (r *Repository) DecideStep(ctx context.Context, in DecideStepInput) (StepRecord, RequestRecord, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return StepRecord{}, RequestRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	rows, err := qtx.DecideLeaveStepGuarded(ctx, leavedb.DecideLeaveStepGuardedParams{
		Decision: in.Decision, DecidedBy: in.DeciderID, Comment: textOrNil(in.Comment),
		ID: in.StepID, SchoolID: in.SchoolID,
	})
	if err != nil {
		return StepRecord{}, RequestRecord{}, err
	}
	if rows == 0 {
		return StepRecord{}, RequestRecord{}, ErrConflict
	}

	if in.NewReqStatus != "" {
		rows, err := qtx.UpdateLeaveRequestStatusGuarded(ctx, leavedb.UpdateLeaveRequestStatusGuardedParams{
			Status: in.NewReqStatus, ID: in.RequestID, SchoolID: in.SchoolID,
		})
		if err != nil {
			return StepRecord{}, RequestRecord{}, err
		}
		if rows == 0 {
			return StepRecord{}, RequestRecord{}, ErrConflict
		}
	}

	stepRow, err := qtx.GetLeaveStepByID(ctx, leavedb.GetLeaveStepByIDParams{ID: in.StepID, SchoolID: in.SchoolID})
	if err != nil {
		return StepRecord{}, RequestRecord{}, err
	}
	reqRow, err := qtx.GetLeaveRequestByID(ctx, leavedb.GetLeaveRequestByIDParams{ID: in.RequestID, SchoolID: in.SchoolID})
	if err != nil {
		return StepRecord{}, RequestRecord{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return StepRecord{}, RequestRecord{}, err
	}

	return stepFromDB(stepRow, ""), requestFromRow(reqRow.ID, reqRow.SchoolID, reqRow.TeacherID, reqRow.TeacherName,
		reqRow.Type, reqRow.DateStart, reqRow.DateEnd, reqRow.Reason, reqRow.Attachment, reqRow.AttachmentName,
		reqRow.AttachmentMime, reqRow.Status, reqRow.CreatedAt), nil
}

// -- rekap & interface publik fase 6 --

func (r *Repository) Summary(ctx context.Context, schoolID int64, from, to time.Time) ([]SummaryRowRaw, error) {
	rows, err := r.q.LeaveSummaryByRange(ctx, leavedb.LeaveSummaryByRangeParams{
		SchoolID: schoolID, FromDate: dateOf(from), ToDate: dateOf(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]SummaryRowRaw, 0, len(rows))
	for _, row := range rows {
		out = append(out, SummaryRowRaw{TeacherID: row.TeacherID, TeacherName: row.TeacherName, Type: row.Type, Days: row.Days})
	}
	return out, nil
}

func (r *Repository) ApprovedOn(ctx context.Context, schoolID, teacherUserID int64, date time.Time) (bool, error) {
	return r.q.LeaveApprovedOn(ctx, leavedb.LeaveApprovedOnParams{
		SchoolID: schoolID, TeacherID: teacherUserID, OnDate: dateOf(date),
	})
}
