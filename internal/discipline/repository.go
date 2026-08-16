package discipline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/discipline/disciplinedb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul discipline.
var ErrNotFound = errors.New("discipline: data tidak ditemukan")

// ErrConflict menandai pelanggaran unique constraint (nama jenis pelanggaran
// dipakai lagi, atau catatan dobel dalam sesi yang sama — lihat
// student_violations_session_unique di migrations/00014_discipline.sql).
var ErrConflict = errors.New("discipline: data sudah ada/bentrok")

// disciplineRepository adalah kontrak yang dibutuhkan Service dari repository
// — dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB — pola
// yang sama dengan internal/leave.
type disciplineRepository interface {
	CreateViolationType(ctx context.Context, schoolID int64, name string, points int, category string) (ViolationTypeRecord, error)
	UpdateViolationType(ctx context.Context, schoolID, id int64, name *string, points *int, category *string, active *bool) (ViolationTypeRecord, error)
	GetViolationTypeByID(ctx context.Context, schoolID, id int64) (ViolationTypeRecord, error)
	ListViolationTypes(ctx context.Context, schoolID int64) ([]ViolationTypeRecord, error)
	DeleteViolationType(ctx context.Context, schoolID, id int64) (int64, error)
	CountViolationsByType(ctx context.Context, schoolID, violationTypeID int64) (int64, error)

	CreateStudentViolation(ctx context.Context, in CreateViolationInput) (int64, error)
	GetRecordByID(ctx context.Context, schoolID, id int64) (RecordDetail, error)
	DeleteStudentViolation(ctx context.Context, schoolID, id int64) (int64, error)
	ListStudentViolations(ctx context.Context, schoolID, activeYearID int64, filter RecordFilter) ([]RecordDetail, int64, error)
	ListViolationsForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) ([]ViolationForYear, error)

	GetSPSettings(ctx context.Context, schoolID, academicYearID int64) (SPSettings, bool, error)
	UpsertSPSettings(ctx context.Context, schoolID, academicYearID int64, in SPSettings) (SPSettings, error)

	NextLetterNumber(ctx context.Context, schoolID int64, year string, level int) (string, error)
	CreateWarningLetter(ctx context.Context, in CreateLetterInput) (LetterRecord, error)
	GetWarningLetterByID(ctx context.Context, schoolID, id int64) (LetterRecord, error)
	ListWarningLettersForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) ([]LetterRecord, error)
	ListWarningLetters(ctx context.Context, schoolID int64, level int, limit, offset int) ([]LetterListRecord, int64, error)

	DisciplineSummary(ctx context.Context, schoolID, activeYearID int64, classID int64, from, to time.Time) ([]SummaryRecord, error)

	// -- validasi & lookup lintas modul (join read-only ke tabel bersama,
	// pola yang sama dengan internal/attendance/queries.sql) --
	StudentExists(ctx context.Context, schoolID, studentID int64) (name, nis string, ok bool, err error)
	AttendanceSessionExists(ctx context.Context, schoolID, sessionID int64) (bool, error)
	HomeroomTeacherUserID(ctx context.Context, schoolID, academicYearID, studentID int64) (userID int64, ok bool, err error)
	StudentClassName(ctx context.Context, schoolID, academicYearID, studentID int64) (string, error)
}

var _ disciplineRepository = (*Repository)(nil)

// Repository membungkus akses data modul discipline (sqlc hand-written + pgx
// — lihat catatan lingkungan build di disciplinedb/models.go).
type Repository struct {
	pool *pgxpool.Pool
	q    *disciplinedb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: disciplinedb.New(pool)}
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

func int4OrNilFromIntPtr(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func boolOrNilFromPtr(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}

func textOrNilFromPtr(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func dateOf(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func dateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

// -- domain records --

// ViolationTypeRecord adalah representasi domain satu baris violation_types.
type ViolationTypeRecord struct {
	ID       int64
	SchoolID int64
	Name     string
	Points   int
	Category string
	Active   bool
}

func violationTypeFromDB(v disciplinedb.ViolationType) ViolationTypeRecord {
	return ViolationTypeRecord{
		ID: v.ID, SchoolID: v.SchoolID, Name: v.Name, Points: int(v.Points),
		Category: v.Category, Active: v.Active,
	}
}

// CreateViolationInput adalah parameter Repository.CreateStudentViolation.
type CreateViolationInput struct {
	SchoolID            int64
	AcademicYearID      int64
	StudentID           int64
	ViolationTypeID     int64
	AttendanceSessionID int64 // 0 = tanpa sesi
	NotedBy             int64
	Note                string
	OccurredOn          time.Time
}

// RecordDetail adalah representasi domain lengkap satu catatan pelanggaran
// (join siswa+kelas+jenis+pencatat) — dipakai GetRecordByID & ListStudentViolations.
type RecordDetail struct {
	ID                  int64
	OccurredOn          time.Time
	Note                string
	AttendanceSessionID int64 // 0 = tanpa sesi
	CreatedAt           time.Time
	StudentID           int64
	StudentName         string
	StudentNIS          string
	ClassName           string
	ViolationTypeID     int64
	ViolationName       string
	ViolationPoints     int
	NotedBy             int64
	NotedByName         string
}

func (r RecordDetail) View() RecordView {
	var sessionID *int64
	if r.AttendanceSessionID != 0 {
		id := r.AttendanceSessionID
		sessionID = &id
	}
	return RecordView{
		ID: r.ID,
		Student: StudentRef{
			ID: r.StudentID, Name: r.StudentName, NIS: r.StudentNIS, ClassName: r.ClassName,
		},
		Violation:           ViolationRef{ID: r.ViolationTypeID, Name: r.ViolationName, Points: r.ViolationPoints},
		NotedByName:         r.NotedByName,
		Note:                r.Note,
		OccurredOn:          NewDate(r.OccurredOn),
		AttendanceSessionID: sessionID,
	}
}

// RecordFilter adalah parameter filter Repository.ListStudentViolations.
type RecordFilter struct {
	StudentID int64
	ClassID   int64
	From      time.Time
	To        time.Time
	Limit     int
	Offset    int
}

// ViolationForYear adalah satu baris pelanggaran siswa dalam SATU tahun
// ajaran (tanpa info siswa — sudah diketahui pemanggil) — dipakai
// Service.buildLetterSnapshot & Service.StudentDiscipline.
type ViolationForYear struct {
	ID                  int64
	OccurredOn          time.Time
	Note                string
	AttendanceSessionID int64
	CreatedAt           time.Time
	ViolationTypeID     int64
	ViolationName       string
	ViolationPoints     int
	NotedBy             int64
	NotedByName         string
}

// CreateLetterInput adalah parameter Repository.CreateWarningLetter.
type CreateLetterInput struct {
	SchoolID       int64
	AcademicYearID int64
	StudentID      int64
	Level          int
	Number         string
	PointsSnapshot int
	Snapshot       []byte
	CreatedBy      int64
}

// LetterRecord adalah representasi domain satu baris discipline_warning_letters.
type LetterRecord struct {
	ID             int64
	SchoolID       int64
	AcademicYearID int64
	StudentID      int64
	Level          int
	Number         string
	PointsSnapshot int
	Snapshot       []byte
	CreatedBy      int64
	CreatedAt      time.Time
}

func letterFromDB(l disciplinedb.DisciplineWarningLetter) LetterRecord {
	return LetterRecord{
		ID: l.ID, SchoolID: l.SchoolID, AcademicYearID: l.AcademicYearID, StudentID: l.StudentID,
		Level: int(l.Level), Number: l.Number, PointsSnapshot: int(l.PointsSnapshot),
		Snapshot: l.Snapshot, CreatedBy: int8ToInt64(l.CreatedBy), CreatedAt: l.CreatedAt.Time,
	}
}

// LetterListRecord adalah LetterRecord + identitas siswa (dipakai
// GET /api/discipline/letters, daftar lintas siswa).
type LetterListRecord struct {
	LetterRecord
	StudentName string
	StudentNIS  string
}

// SummaryRecord adalah satu baris hasil Repository.DisciplineSummary.
type SummaryRecord struct {
	StudentID   int64
	StudentName string
	StudentNIS  string
	ClassName   string
	TotalPoints int
	RecordCount int
	LastAt      time.Time // zero value = belum pernah ada catatan
}

// -- violation_types --

func (r *Repository) CreateViolationType(ctx context.Context, schoolID int64, name string, points int, category string) (ViolationTypeRecord, error) {
	row, err := r.q.CreateViolationType(ctx, disciplinedb.CreateViolationTypeParams{
		SchoolID: schoolID, Name: name, Points: int32(points), Category: category, Active: true,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return ViolationTypeRecord{}, ErrConflict
		}
		return ViolationTypeRecord{}, err
	}
	return violationTypeFromDB(row), nil
}

func (r *Repository) UpdateViolationType(ctx context.Context, schoolID, id int64, name *string, points *int, category *string, active *bool) (ViolationTypeRecord, error) {
	row, err := r.q.UpdateViolationType(ctx, disciplinedb.UpdateViolationTypeParams{
		Name: textOrNilFromPtr(name), Points: int4OrNilFromIntPtr(points), Category: textOrNilFromPtr(category),
		Active: boolOrNilFromPtr(active), ID: id, SchoolID: schoolID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return ViolationTypeRecord{}, ErrConflict
		}
		return ViolationTypeRecord{}, mapNoRows(err)
	}
	return violationTypeFromDB(row), nil
}

func (r *Repository) GetViolationTypeByID(ctx context.Context, schoolID, id int64) (ViolationTypeRecord, error) {
	row, err := r.q.GetViolationTypeByID(ctx, disciplinedb.GetViolationTypeByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return ViolationTypeRecord{}, mapNoRows(err)
	}
	return violationTypeFromDB(row), nil
}

func (r *Repository) ListViolationTypes(ctx context.Context, schoolID int64) ([]ViolationTypeRecord, error) {
	rows, err := r.q.ListViolationTypes(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]ViolationTypeRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, violationTypeFromDB(row))
	}
	return out, nil
}

func (r *Repository) DeleteViolationType(ctx context.Context, schoolID, id int64) (int64, error) {
	return r.q.DeleteViolationType(ctx, disciplinedb.DeleteViolationTypeParams{ID: id, SchoolID: schoolID})
}

func (r *Repository) CountViolationsByType(ctx context.Context, schoolID, violationTypeID int64) (int64, error) {
	return r.q.CountViolationsByType(ctx, disciplinedb.CountViolationsByTypeParams{SchoolID: schoolID, ViolationTypeID: violationTypeID})
}

// -- student_violations --

func (r *Repository) CreateStudentViolation(ctx context.Context, in CreateViolationInput) (int64, error) {
	row, err := r.q.CreateStudentViolation(ctx, disciplinedb.CreateStudentViolationParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, StudentID: in.StudentID,
		ViolationTypeID: in.ViolationTypeID, AttendanceSessionID: int8OrNil(in.AttendanceSessionID),
		NotedBy: in.NotedBy, Note: in.Note, OccurredOn: dateOf(in.OccurredOn),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrConflict
		}
		return 0, err
	}
	return row.ID, nil
}

func recordDetailFromRow(id int64, occurredOn pgtype.Date, note string, attendanceSessionID pgtype.Int8, createdAt pgtype.Timestamptz,
	studentID int64, studentName, studentNIS, className string, violationTypeID int64, violationName string, violationPoints int32,
	notedBy int64, notedByName string) RecordDetail {
	return RecordDetail{
		ID: id, OccurredOn: dateToTime(occurredOn), Note: note, AttendanceSessionID: int8ToInt64(attendanceSessionID),
		CreatedAt: createdAt.Time, StudentID: studentID, StudentName: studentName, StudentNIS: studentNIS, ClassName: className,
		ViolationTypeID: violationTypeID, ViolationName: violationName, ViolationPoints: int(violationPoints),
		NotedBy: notedBy, NotedByName: notedByName,
	}
}

func (r *Repository) GetRecordByID(ctx context.Context, schoolID, id int64) (RecordDetail, error) {
	row, err := r.q.GetStudentViolationByID(ctx, disciplinedb.GetStudentViolationByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return RecordDetail{}, mapNoRows(err)
	}
	return recordDetailFromRow(row.ID, row.OccurredOn, row.Note, row.AttendanceSessionID, row.CreatedAt,
		row.StudentID, row.StudentName, row.StudentNis, row.ClassName, row.ViolationTypeID, row.ViolationName, row.ViolationPoints,
		row.NotedBy, row.NotedByName), nil
}

func (r *Repository) DeleteStudentViolation(ctx context.Context, schoolID, id int64) (int64, error) {
	return r.q.DeleteStudentViolation(ctx, disciplinedb.DeleteStudentViolationParams{ID: id, SchoolID: schoolID})
}

func (r *Repository) ListStudentViolations(ctx context.Context, schoolID, activeYearID int64, filter RecordFilter) ([]RecordDetail, int64, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.q.ListStudentViolations(ctx, disciplinedb.ListStudentViolationsParams{
		AcademicYearID: activeYearID, SchoolID: schoolID,
		StudentID: int8OrNil(filter.StudentID), ClassID: int8OrNil(filter.ClassID),
		FromDate: dateOf(filter.From), ToDate: dateOf(filter.To),
		LimitRows: int32(limit), OffsetRows: int32(filter.Offset),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]RecordDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, recordDetailFromRow(row.ID, row.OccurredOn, row.Note, row.AttendanceSessionID, row.CreatedAt,
			row.StudentID, row.StudentName, row.StudentNis, row.ClassName, row.ViolationTypeID, row.ViolationName, row.ViolationPoints,
			row.NotedBy, row.NotedByName))
	}
	total, err := r.q.CountStudentViolations(ctx, disciplinedb.CountStudentViolationsParams{
		AcademicYearID: activeYearID, SchoolID: schoolID,
		StudentID: int8OrNil(filter.StudentID), ClassID: int8OrNil(filter.ClassID),
		FromDate: dateOf(filter.From), ToDate: dateOf(filter.To),
	})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *Repository) ListViolationsForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) ([]ViolationForYear, error) {
	rows, err := r.q.ListViolationsForStudentYear(ctx, disciplinedb.ListViolationsForStudentYearParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, StudentID: studentID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ViolationForYear, 0, len(rows))
	for _, row := range rows {
		out = append(out, ViolationForYear{
			ID: row.ID, OccurredOn: dateToTime(row.OccurredOn), Note: row.Note,
			AttendanceSessionID: int8ToInt64(row.AttendanceSessionID), CreatedAt: row.CreatedAt.Time,
			ViolationTypeID: row.ViolationTypeID, ViolationName: row.ViolationName, ViolationPoints: int(row.ViolationPoints),
			NotedBy: row.NotedBy, NotedByName: row.NotedByName,
		})
	}
	return out, nil
}

// -- discipline_sp_settings --

func (r *Repository) GetSPSettings(ctx context.Context, schoolID, academicYearID int64) (SPSettings, bool, error) {
	row, err := r.q.GetSPSettings(ctx, disciplinedb.GetSPSettingsParams{SchoolID: schoolID, AcademicYearID: academicYearID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SPSettings{}, false, nil
		}
		return SPSettings{}, false, err
	}
	return SPSettings{SP1: int(row.Sp1), SP2: int(row.Sp2), SP3: int(row.Sp3)}, true, nil
}

func (r *Repository) UpsertSPSettings(ctx context.Context, schoolID, academicYearID int64, in SPSettings) (SPSettings, error) {
	row, err := r.q.UpsertSPSettings(ctx, disciplinedb.UpsertSPSettingsParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, Sp1: int32(in.SP1), Sp2: int32(in.SP2), Sp3: int32(in.SP3),
	})
	if err != nil {
		return SPSettings{}, err
	}
	return SPSettings{SP1: int(row.Sp1), SP2: int(row.Sp2), SP3: int(row.Sp3)}, nil
}

// -- nomor surat --

// NextLetterNumber menghasilkan nomor "SP{level}/{tahun}/{seq}" (seq atomik
// per sekolah per tahun, lihat migrations/00014_discipline.sql).
func (r *Repository) NextLetterNumber(ctx context.Context, schoolID int64, year string, level int) (string, error) {
	seq, err := r.q.NextLetterSeq(ctx, disciplinedb.NextLetterSeqParams{SchoolID: schoolID, Year: year})
	if err != nil {
		return "", err
	}
	return formatLetterNumber(level, year, int(seq)), nil
}

// -- discipline_warning_letters --

func (r *Repository) CreateWarningLetter(ctx context.Context, in CreateLetterInput) (LetterRecord, error) {
	row, err := r.q.CreateWarningLetter(ctx, disciplinedb.CreateWarningLetterParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, StudentID: in.StudentID, Level: int32(in.Level),
		Number: in.Number, PointsSnapshot: int32(in.PointsSnapshot), Snapshot: in.Snapshot, CreatedBy: int8OrNil(in.CreatedBy),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return LetterRecord{}, ErrConflict
		}
		return LetterRecord{}, err
	}
	return letterFromDB(row), nil
}

func (r *Repository) GetWarningLetterByID(ctx context.Context, schoolID, id int64) (LetterRecord, error) {
	row, err := r.q.GetWarningLetterByID(ctx, disciplinedb.GetWarningLetterByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return LetterRecord{}, mapNoRows(err)
	}
	return letterFromDB(row), nil
}

func (r *Repository) ListWarningLettersForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) ([]LetterRecord, error) {
	rows, err := r.q.ListWarningLettersForStudentYear(ctx, disciplinedb.ListWarningLettersForStudentYearParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, StudentID: studentID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]LetterRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, letterFromDB(row))
	}
	return out, nil
}

func (r *Repository) ListWarningLetters(ctx context.Context, schoolID int64, level int, limit, offset int) ([]LetterListRecord, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	var levelFilter pgtype.Int4
	if level != 0 {
		levelFilter = pgtype.Int4{Int32: int32(level), Valid: true}
	}
	rows, err := r.q.ListWarningLetters(ctx, disciplinedb.ListWarningLettersParams{
		SchoolID: schoolID, Level: levelFilter, LimitRows: int32(limit), OffsetRows: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]LetterListRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, LetterListRecord{
			LetterRecord: LetterRecord{
				ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID, StudentID: row.StudentID,
				Level: int(row.Level), Number: row.Number, PointsSnapshot: int(row.PointsSnapshot),
				Snapshot: row.Snapshot, CreatedBy: int8ToInt64(row.CreatedBy), CreatedAt: row.CreatedAt.Time,
			},
			StudentName: row.StudentName, StudentNIS: row.StudentNis,
		})
	}
	total, err := r.q.CountWarningLetters(ctx, disciplinedb.CountWarningLettersParams{SchoolID: schoolID, Level: levelFilter})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// -- rekap --

func (r *Repository) DisciplineSummary(ctx context.Context, schoolID, activeYearID int64, classID int64, from, to time.Time) ([]SummaryRecord, error) {
	rows, err := r.q.DisciplineSummary(ctx, disciplinedb.DisciplineSummaryParams{
		AcademicYearID: activeYearID, SchoolID: schoolID, ClassID: int8OrNil(classID),
		FromDate: dateOf(from), ToDate: dateOf(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]SummaryRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, SummaryRecord{
			StudentID: row.StudentID, StudentName: row.StudentName, StudentNIS: row.StudentNis, ClassName: row.ClassName,
			TotalPoints: int(row.TotalPoints), RecordCount: int(row.RecordCount), LastAt: aggDateToTime(row.LastAt),
		})
	}
	return out, nil
}

// aggDateToTime menangani hasil agregat MAX(date) yang oleh sqlc di-generate
// sebagai interface{} (bukan pgtype.Date) — bentuk runtime-nya tetap
// pgtype.Date atau time.Time tergantung driver.
func aggDateToTime(v interface{}) time.Time {
	switch d := v.(type) {
	case pgtype.Date:
		return d.Time
	case time.Time:
		return d
	default:
		return time.Time{}
	}
}

// -- validasi & lookup lintas modul --

func (r *Repository) StudentExists(ctx context.Context, schoolID, studentID int64) (string, string, bool, error) {
	row, err := r.q.GetStudentBasic(ctx, disciplinedb.GetStudentBasicParams{ID: studentID, SchoolID: schoolID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return row.Name, row.Nis, true, nil
}

func (r *Repository) AttendanceSessionExists(ctx context.Context, schoolID, sessionID int64) (bool, error) {
	_, err := r.q.AttendanceSessionExists(ctx, disciplinedb.AttendanceSessionExistsParams{ID: sessionID, SchoolID: schoolID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) HomeroomTeacherUserID(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, bool, error) {
	id, err := r.q.GetHomeroomTeacherUserID(ctx, disciplinedb.GetHomeroomTeacherUserIDParams{
		StudentID: studentID, SchoolID: schoolID, AcademicYearID: academicYearID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

// StudentClassName mengembalikan "" (tanpa error) bila siswa tidak terdaftar
// di kelas manapun pada TA tersebut (pola sama HomeroomTeacherUserID).
func (r *Repository) StudentClassName(ctx context.Context, schoolID, academicYearID, studentID int64) (string, error) {
	name, err := r.q.GetStudentClassName(ctx, disciplinedb.GetStudentClassNameParams{
		StudentID: studentID, SchoolID: schoolID, AcademicYearID: academicYearID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return name, nil
}

// formatLetterNumber menyusun nomor surat "SP{level}/{tahun}/{seq}"
// (docs/12-sion-parity.md) — seq zero-padded 4 digit, pola yang sama dengan
// billing.nextInvoiceNumber ("INV/YYYY/NNNN").
func formatLetterNumber(level int, year string, seq int) string {
	return fmt.Sprintf("SP%d/%s/%04d", level, year, seq)
}
