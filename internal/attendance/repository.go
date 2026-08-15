package attendance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/attendance/attendancedb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul attendance.
var ErrNotFound = errors.New("attendance: data tidak ditemukan")

// ErrConflict menandai pelanggaran unique constraint (sqlc.arg partial unique
// (class_id,date) WHERE type='daily') — dipakai Service untuk create-or-get
// sesi idempoten (lihat docs/05-attendance.md).
var ErrConflict = errors.New("attendance: sesi sudah ada")

// attendanceRepository adalah kontrak yang dibutuhkan Service dari repository
// — dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB (lihat
// service_test.go), sama seperti pola internal/student.
type attendanceRepository interface {
	ListClassesForDate(ctx context.Context, schoolID, academicYearID int64, date time.Time) ([]ClassAttendanceRow, error)
	GetClassMeta(ctx context.Context, schoolID, classID int64) (ClassMeta, error)
	ListClassStudents(ctx context.Context, schoolID, classID int64) ([]RosterStudent, error)

	CreateSession(ctx context.Context, in CreateSessionInput) (SessionRecord, error)
	GetSessionByClassDate(ctx context.Context, schoolID, classID int64, date time.Time) (SessionRecord, error)
	CreateSubjectSession(ctx context.Context, in CreateSubjectSessionInput) (SessionRecord, error)
	GetSessionBySlotDate(ctx context.Context, schoolID, scheduleSlotID int64, date time.Time) (SessionRecord, error)
	GetSessionByID(ctx context.Context, schoolID, id int64) (SessionRecord, error)
	GetSessionDetail(ctx context.Context, schoolID, id int64) (SessionDetailRow, error)
	FinalizeSession(ctx context.Context, schoolID, id int64) (SessionRecord, error)

	ListSessionRecords(ctx context.Context, sessionID int64) ([]RecordRow, error)
	BulkUpsertRecords(ctx context.Context, schoolID, sessionID, actorUserID int64, method string, items []RecordInput) error
	CountEnrolledForClass(ctx context.Context, classID int64) (int64, error)
	CountRecordsForSession(ctx context.Context, sessionID int64) (int64, error)

	SummaryByDate(ctx context.Context, schoolID, academicYearID int64, date time.Time, classID int64) ([]SummaryRow, error)
	StudentHistory(ctx context.Context, schoolID, studentID int64, from, to time.Time) ([]HistoryRow, error)
	StudentCounts(ctx context.Context, schoolID, studentID int64, from, to time.Time) (Counts, error)

	GetSettings(ctx context.Context, schoolID int64) (Settings, error)
}

var _ attendanceRepository = (*Repository)(nil)

// Repository membungkus akses data modul attendance (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *attendancedb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: attendancedb.New(pool)}
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

// isUniqueViolation melaporkan apakah err adalah pelanggaran unique
// constraint Postgres (kode 23505) — dipakai CreateSession untuk deteksi
// sesi yang sudah ada (create-or-get idempoten).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func textOrNil(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func dateOf(t time.Time) pgtype.Date {
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

// -- domain records --

type ClassMeta struct {
	ID             int64
	Name           string
	AcademicYearID int64
}

type SessionRecord struct {
	ID             int64
	SchoolID       int64
	AcademicYearID int64
	ClassID        int64
	Date           time.Time
	Type           string
	ScheduleSlotID int64 // 0 = NULL (sesi daily)
	OpenedBy       int64
	Status         string
	CreatedAt      time.Time
}

func sessionFromDB(s attendancedb.AttendanceSession) SessionRecord {
	return SessionRecord{
		ID: s.ID, SchoolID: s.SchoolID, AcademicYearID: s.AcademicYearID, ClassID: s.ClassID,
		Date: s.Date.Time, Type: s.Type, ScheduleSlotID: int8ToInt64(s.ScheduleSlotID), OpenedBy: s.OpenedBy, Status: s.Status, CreatedAt: s.CreatedAt.Time,
	}
}

type SessionDetailRow struct {
	SessionRecord
	ClassName    string
	OpenedByName string
}

type RosterStudent struct {
	ID   int64
	Name string
	NIS  string
}

type RecordRow struct {
	StudentID int64
	Status    string
	Note      string
	Method    string
	MarkedAt  time.Time
}

type ClassAttendanceRow struct {
	ClassID       int64
	Name          string
	StudentCount  int64
	SessionID     int64 // 0 = belum ada sesi untuk tanggal ini
	SessionStatus string
	MarkedCount   int64
}

type RecordInput struct {
	StudentID int64
	Status    string
	Note      string
}

type SummaryRow struct {
	ClassID       int64
	ClassName     string
	Total         int64
	Hadir         int64
	Terlambat     int64
	Izin          int64
	Sakit         int64
	Alpa          int64
	SessionStatus string
}

type Counts struct {
	Hadir     int64
	Terlambat int64
	Izin      int64
	Sakit     int64
	Alpa      int64
}

type HistoryRow struct {
	Date   time.Time
	Status string
	Note   string
}

// -- classes / roster --

func (r *Repository) ListClassesForDate(ctx context.Context, schoolID, academicYearID int64, date time.Time) ([]ClassAttendanceRow, error) {
	rows, err := r.q.ListClassesForDate(ctx, attendancedb.ListClassesForDateParams{
		Date: dateOf(date), SchoolID: schoolID, AcademicYearID: academicYearID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ClassAttendanceRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ClassAttendanceRow{
			ClassID: row.ID, Name: row.Name, StudentCount: row.StudentCount,
			SessionID: row.SessionID.Int64, SessionStatus: row.SessionStatus.String, MarkedCount: row.MarkedCount,
		})
	}
	return out, nil
}

func (r *Repository) GetClassMeta(ctx context.Context, schoolID, classID int64) (ClassMeta, error) {
	row, err := r.q.GetClassMeta(ctx, attendancedb.GetClassMetaParams{ID: classID, SchoolID: schoolID})
	if err != nil {
		return ClassMeta{}, mapNoRows(err)
	}
	return ClassMeta{ID: row.ID, Name: row.Name, AcademicYearID: row.AcademicYearID}, nil
}

func (r *Repository) ListClassStudents(ctx context.Context, schoolID, classID int64) ([]RosterStudent, error) {
	rows, err := r.q.ListClassStudents(ctx, attendancedb.ListClassStudentsParams{ClassID: classID, SchoolID: schoolID})
	if err != nil {
		return nil, err
	}
	out := make([]RosterStudent, 0, len(rows))
	for _, row := range rows {
		out = append(out, RosterStudent{ID: row.ID, Name: row.Name, NIS: row.Nis})
	}
	return out, nil
}

// -- sessions --

type CreateSessionInput struct {
	SchoolID       int64
	AcademicYearID int64
	ClassID        int64
	Date           time.Time
	OpenedBy       int64
}

// CreateSession mengembalikan ErrConflict (bukan error pgx mentah) bila sesi
// daily untuk (class_id,date) itu sudah ada — pemanggil (Service) lalu
// memanggil GetSessionByClassDate untuk mengambilnya (create-or-get idempoten).
func (r *Repository) CreateSession(ctx context.Context, in CreateSessionInput) (SessionRecord, error) {
	row, err := r.q.CreateSession(ctx, attendancedb.CreateSessionParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, ClassID: in.ClassID,
		Date: dateOf(in.Date), OpenedBy: in.OpenedBy,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return SessionRecord{}, ErrConflict
		}
		return SessionRecord{}, err
	}
	return sessionFromDB(row), nil
}

// CreateSubjectSessionInput adalah parameter INSERT sesi type='subject'
// (fase 6 — docs/05 & docs/06). Sama seperti CreateSession, mengembalikan
// ErrConflict bila sesi utk (schedule_slot_id,date) itu sudah ada (partial
// unique index attendance_sessions_subject_unique) — pemanggil (Service)
// lalu memanggil GetSessionBySlotDate (create-or-get idempoten).
type CreateSubjectSessionInput struct {
	SchoolID       int64
	AcademicYearID int64
	ClassID        int64
	ScheduleSlotID int64
	Date           time.Time
	OpenedBy       int64
}

func (r *Repository) CreateSubjectSession(ctx context.Context, in CreateSubjectSessionInput) (SessionRecord, error) {
	row, err := r.q.CreateSubjectSession(ctx, attendancedb.CreateSubjectSessionParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, ClassID: in.ClassID,
		Date: dateOf(in.Date), ScheduleSlotID: int8OrNil(in.ScheduleSlotID), OpenedBy: in.OpenedBy,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return SessionRecord{}, ErrConflict
		}
		return SessionRecord{}, err
	}
	return sessionFromDB(row), nil
}

func (r *Repository) GetSessionBySlotDate(ctx context.Context, schoolID, scheduleSlotID int64, date time.Time) (SessionRecord, error) {
	row, err := r.q.GetSessionBySlotDate(ctx, attendancedb.GetSessionBySlotDateParams{
		SchoolID: schoolID, ScheduleSlotID: int8OrNil(scheduleSlotID), Date: dateOf(date),
	})
	if err != nil {
		return SessionRecord{}, mapNoRows(err)
	}
	return sessionFromDB(row), nil
}

func (r *Repository) GetSessionByClassDate(ctx context.Context, schoolID, classID int64, date time.Time) (SessionRecord, error) {
	row, err := r.q.GetSessionByClassDate(ctx, attendancedb.GetSessionByClassDateParams{
		SchoolID: schoolID, ClassID: classID, Date: dateOf(date),
	})
	if err != nil {
		return SessionRecord{}, mapNoRows(err)
	}
	return sessionFromDB(row), nil
}

func (r *Repository) GetSessionByID(ctx context.Context, schoolID, id int64) (SessionRecord, error) {
	row, err := r.q.GetSessionByID(ctx, attendancedb.GetSessionByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return SessionRecord{}, mapNoRows(err)
	}
	return sessionFromDB(row), nil
}

func (r *Repository) GetSessionDetail(ctx context.Context, schoolID, id int64) (SessionDetailRow, error) {
	row, err := r.q.GetSessionDetail(ctx, attendancedb.GetSessionDetailParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return SessionDetailRow{}, mapNoRows(err)
	}
	return SessionDetailRow{
		SessionRecord: SessionRecord{
			ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID, ClassID: row.ClassID,
			Date: row.Date.Time, Type: row.Type, OpenedBy: row.OpenedBy, Status: row.Status, CreatedAt: row.CreatedAt.Time,
		},
		ClassName: row.ClassName, OpenedByName: row.OpenedByName,
	}, nil
}

func (r *Repository) FinalizeSession(ctx context.Context, schoolID, id int64) (SessionRecord, error) {
	row, err := r.q.FinalizeSession(ctx, attendancedb.FinalizeSessionParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return SessionRecord{}, mapNoRows(err)
	}
	return sessionFromDB(row), nil
}

// -- records --

func (r *Repository) ListSessionRecords(ctx context.Context, sessionID int64) ([]RecordRow, error) {
	rows, err := r.q.ListSessionRecords(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]RecordRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, RecordRow{
			StudentID: row.StudentID, Status: row.Status, Note: row.Note.String,
			Method: row.Method, MarkedAt: row.MarkedAt.Time,
		})
	}
	return out, nil
}

// BulkUpsertRecords menulis sekumpulan record dalam satu transaksi (satu
// method & marked_by untuk seluruh batch — input manual bulk save,
// docs/05-attendance.md).
func (r *Repository) BulkUpsertRecords(ctx context.Context, schoolID, sessionID, actorUserID int64, method string, items []RecordInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	for _, item := range items {
		if _, err := qtx.UpsertRecord(ctx, attendancedb.UpsertRecordParams{
			SchoolID: schoolID, SessionID: sessionID, StudentID: item.StudentID,
			Status: item.Status, Method: method, MarkedBy: actorUserID, Note: textOrNil(item.Note),
		}); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) CountEnrolledForClass(ctx context.Context, classID int64) (int64, error) {
	return r.q.CountEnrolledForClass(ctx, classID)
}

func (r *Repository) CountRecordsForSession(ctx context.Context, sessionID int64) (int64, error) {
	return r.q.CountRecordsForSession(ctx, sessionID)
}

// -- rekap & riwayat --

func (r *Repository) SummaryByDate(ctx context.Context, schoolID, academicYearID int64, date time.Time, classID int64) ([]SummaryRow, error) {
	rows, err := r.q.SummaryByDate(ctx, attendancedb.SummaryByDateParams{
		Date: dateOf(date), SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: int8OrNil(classID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]SummaryRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, SummaryRow{
			ClassID: row.ClassID, ClassName: row.ClassName, Total: row.Total,
			Hadir: row.Hadir, Terlambat: row.Terlambat, Izin: row.Izin, Sakit: row.Sakit, Alpa: row.Alpa,
			SessionStatus: row.SessionStatus,
		})
	}
	return out, nil
}

func (r *Repository) StudentHistory(ctx context.Context, schoolID, studentID int64, from, to time.Time) ([]HistoryRow, error) {
	rows, err := r.q.StudentAttendanceHistory(ctx, attendancedb.StudentAttendanceHistoryParams{
		StudentID: studentID, SchoolID: schoolID, FromDate: dateOf(from), ToDate: dateOf(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]HistoryRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, HistoryRow{Date: row.Date.Time, Status: row.Status, Note: row.Note.String})
	}
	return out, nil
}

func (r *Repository) StudentCounts(ctx context.Context, schoolID, studentID int64, from, to time.Time) (Counts, error) {
	row, err := r.q.StudentAttendanceCounts(ctx, attendancedb.StudentAttendanceCountsParams{
		StudentID: studentID, SchoolID: schoolID, FromDate: dateOf(from), ToDate: dateOf(to),
	})
	if err != nil {
		return Counts{}, err
	}
	return Counts{Hadir: row.Hadir, Terlambat: row.Terlambat, Izin: row.Izin, Sakit: row.Sakit, Alpa: row.Alpa}, nil
}

// -- settings --

// GetSettings membaca school_settings module='attendance' langsung (tabel &
// pola yang sama dipakai tenant.SettingsService — lihat catatan di
// queries.sql) dan mengembalikan default bila sekolah belum pernah menyimpan
// pengaturannya.
func (r *Repository) GetSettings(ctx context.Context, schoolID int64) (Settings, error) {
	raw, err := r.q.GetAttendanceSettingsRaw(ctx, schoolID)
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
