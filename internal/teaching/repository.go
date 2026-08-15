package teaching

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/teaching/teachingdb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul teaching.
var ErrNotFound = errors.New("teaching: data tidak ditemukan")

// teachingRepository adalah kontrak yang dibutuhkan Service dari repository —
// dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB — pola
// yang sama dengan internal/schedule & internal/attendance.
type teachingRepository interface {
	InsertJournal(ctx context.Context, in JournalInput) (JournalRecord, error)
	GetJournalByID(ctx context.Context, schoolID, id int64) (JournalRecord, error)
	GetJournalBySlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (JournalRecord, error)
	GetJournalDetail(ctx context.Context, schoolID, id int64) (JournalDetail, error)
	GetJournalDetailBySlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (JournalDetail, error)
	ListJournals(ctx context.Context, schoolID int64, teacherID int64, date time.Time, monthStart, monthEnd time.Time) ([]JournalDetail, error)
	UpdateJournalNote(ctx context.Context, schoolID, id int64, material, note string) (JournalRecord, error)
	EndJournal(ctx context.Context, schoolID, id int64, endedAt time.Time) (JournalRecord, error)

	// referensi lintas modul READ-ONLY (lihat catatan queries.sql).
	LookupRoomByToken(ctx context.Context, schoolID int64, token string) (RoomRef, error)
	LookupRoomByID(ctx context.Context, schoolID, id int64) (RoomRef, error)
	LookupClassRef(ctx context.Context, schoolID, id int64) (ClassRef, error)
	LookupSubjectRef(ctx context.Context, schoolID, id int64) (SubjectRef, error)
	LookupTeacherUserID(ctx context.Context, schoolID, teacherID int64) (int64, error)
	LookupTeacherRef(ctx context.Context, schoolID, teacherID int64) (TeacherRef, error)

	GetSettings(ctx context.Context, schoolID int64) (Settings, error)
}

var _ teachingRepository = (*Repository)(nil)

// Repository membungkus akses data modul teaching (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *teachingdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: teachingdb.New(pool)}
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

func dateOf(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func tsOrNil(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// -- domain records --

// JournalRecord adalah representasi mentah satu baris teaching_journals
// (tanpa join nama — dipakai alur tulis: insert/update/end).
type JournalRecord struct {
	ID             int64
	SchoolID       int64
	TeacherID      int64
	ScheduleSlotID int64 // 0 = unscheduled
	ClassID        int64
	SubjectID      int64 // 0 = tidak diisi
	RoomID         int64 // 0 = tidak diisi
	Date           time.Time
	StartedAt      time.Time
	EndedAt        time.Time // zero = belum diakhiri
	Material       string
	Note           string
	Flags          []string
}

// JournalInput adalah parameter INSERT satu jurnal.
type JournalInput struct {
	SchoolID       int64
	TeacherID      int64
	ScheduleSlotID int64 // 0 = unscheduled (NULL)
	ClassID        int64
	SubjectID      int64 // 0 = NULL
	RoomID         int64 // 0 = NULL
	Date           time.Time
	StartedAt      time.Time
	Flags          []string
}

// JournalDetail adalah JournalRecord + nama-nama hasil join (teacher/class/
// subject/room) — dipakai membangun JournalView (docs/06-teaching.md).
type JournalDetail struct {
	JournalRecord
	TeacherUserID int64
	TeacherName   string
	ClassName     string
	SubjectCode   string // "" bila subject_id NULL
	SubjectName   string
	RoomName      string // "" bila room_id NULL
}

func journalFromDB(j teachingdb.TeachingJournal) JournalRecord {
	return JournalRecord{
		ID: j.ID, SchoolID: j.SchoolID, TeacherID: j.TeacherID,
		ScheduleSlotID: int8ToInt64(j.ScheduleSlotID), ClassID: j.ClassID,
		SubjectID: int8ToInt64(j.SubjectID), RoomID: int8ToInt64(j.RoomID),
		Date: j.Date.Time, StartedAt: j.StartedAt.Time, EndedAt: j.EndedAt.Time,
		Material: j.Material.String, Note: j.Note.String, Flags: append([]string{}, j.Flags...),
	}
}

// -- journals --

func (r *Repository) InsertJournal(ctx context.Context, in JournalInput) (JournalRecord, error) {
	flags := in.Flags
	if flags == nil {
		flags = []string{}
	}
	row, err := r.q.InsertJournal(ctx, teachingdb.InsertJournalParams{
		SchoolID: in.SchoolID, TeacherID: in.TeacherID,
		ScheduleSlotID: int8OrNil(in.ScheduleSlotID), ClassID: in.ClassID,
		SubjectID: int8OrNil(in.SubjectID), RoomID: int8OrNil(in.RoomID),
		Date: dateOf(in.Date), StartedAt: tsOrNil(in.StartedAt), Flags: flags,
	})
	if err != nil {
		return JournalRecord{}, err
	}
	return journalFromDB(row), nil
}

func (r *Repository) GetJournalByID(ctx context.Context, schoolID, id int64) (JournalRecord, error) {
	row, err := r.q.GetJournalByID(ctx, teachingdb.GetJournalByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return JournalRecord{}, mapNoRows(err)
	}
	return journalFromDB(row), nil
}

func (r *Repository) GetJournalBySlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (JournalRecord, error) {
	row, err := r.q.GetJournalBySlotDate(ctx, teachingdb.GetJournalBySlotDateParams{
		SchoolID: schoolID, ScheduleSlotID: int8OrNil(slotID), Date: dateOf(date),
	})
	if err != nil {
		return JournalRecord{}, mapNoRows(err)
	}
	return journalFromDB(row), nil
}

func detailFromJoinRow(id, schoolID, teacherID int64, scheduleSlotID pgtype.Int8, classID int64, subjectID, roomID pgtype.Int8,
	date pgtype.Date, startedAt, endedAt pgtype.Timestamptz, material, note pgtype.Text, flags []string,
	teacherUserID int64, teacherName, className string, subjectCode, subjectName, roomName pgtype.Text) JournalDetail {
	return JournalDetail{
		JournalRecord: JournalRecord{
			ID: id, SchoolID: schoolID, TeacherID: teacherID, ScheduleSlotID: int8ToInt64(scheduleSlotID),
			ClassID: classID, SubjectID: int8ToInt64(subjectID), RoomID: int8ToInt64(roomID),
			Date: date.Time, StartedAt: startedAt.Time, EndedAt: endedAt.Time,
			Material: material.String, Note: note.String, Flags: append([]string{}, flags...),
		},
		TeacherUserID: teacherUserID, TeacherName: teacherName, ClassName: className,
		SubjectCode: subjectCode.String, SubjectName: subjectName.String, RoomName: roomName.String,
	}
}

func (r *Repository) GetJournalDetail(ctx context.Context, schoolID, id int64) (JournalDetail, error) {
	row, err := r.q.GetJournalRow(ctx, teachingdb.GetJournalRowParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return JournalDetail{}, mapNoRows(err)
	}
	return detailFromJoinRow(row.ID, row.SchoolID, row.TeacherID, row.ScheduleSlotID, row.ClassID, row.SubjectID, row.RoomID,
		row.Date, row.StartedAt, row.EndedAt, row.Material, row.Note, row.Flags,
		row.TeacherUserID, row.TeacherName, row.ClassName, row.SubjectCode, row.SubjectName, row.RoomName), nil
}

// GetJournalDetailBySlotDate dipakai alur scan idempoten (kembalikan journal
// existing lengkap dengan nama-nama, bukan hanya JournalRecord mentah).
func (r *Repository) GetJournalDetailBySlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (JournalDetail, error) {
	rec, err := r.GetJournalBySlotDate(ctx, schoolID, slotID, date)
	if err != nil {
		return JournalDetail{}, err
	}
	return r.GetJournalDetail(ctx, schoolID, rec.ID)
}

func (r *Repository) ListJournals(ctx context.Context, schoolID int64, teacherID int64, date time.Time, monthStart, monthEnd time.Time) ([]JournalDetail, error) {
	rows, err := r.q.ListJournalsRow(ctx, teachingdb.ListJournalsRowParams{
		SchoolID: schoolID, TeacherID: int8OrNil(teacherID), Date: dateOf(date),
		MonthStart: dateOf(monthStart), MonthEnd: dateOf(monthEnd),
	})
	if err != nil {
		return nil, err
	}
	out := make([]JournalDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromJoinRow(row.ID, row.SchoolID, row.TeacherID, row.ScheduleSlotID, row.ClassID, row.SubjectID, row.RoomID,
			row.Date, row.StartedAt, row.EndedAt, row.Material, row.Note, row.Flags,
			row.TeacherUserID, row.TeacherName, row.ClassName, row.SubjectCode, row.SubjectName, row.RoomName))
	}
	return out, nil
}

func (r *Repository) UpdateJournalNote(ctx context.Context, schoolID, id int64, material, note string) (JournalRecord, error) {
	row, err := r.q.UpdateJournalNote(ctx, teachingdb.UpdateJournalNoteParams{
		ID: id, SchoolID: schoolID, Material: textOrNil(material), Note: textOrNil(note),
	})
	if err != nil {
		return JournalRecord{}, mapNoRows(err)
	}
	return journalFromDB(row), nil
}

func (r *Repository) EndJournal(ctx context.Context, schoolID, id int64, endedAt time.Time) (JournalRecord, error) {
	row, err := r.q.EndJournal(ctx, teachingdb.EndJournalParams{ID: id, SchoolID: schoolID, EndedAt: tsOrNil(endedAt)})
	if err != nil {
		return JournalRecord{}, mapNoRows(err)
	}
	return journalFromDB(row), nil
}

// -- referensi lintas modul (READ-ONLY) --

func (r *Repository) LookupRoomByToken(ctx context.Context, schoolID int64, token string) (RoomRef, error) {
	row, err := r.q.LookupRoomByToken(ctx, teachingdb.LookupRoomByTokenParams{SchoolID: schoolID, QrToken: token})
	if err != nil {
		return RoomRef{}, mapNoRows(err)
	}
	return RoomRef{ID: row.ID, Name: row.Name}, nil
}

func (r *Repository) LookupRoomByID(ctx context.Context, schoolID, id int64) (RoomRef, error) {
	row, err := r.q.LookupRoomByID(ctx, teachingdb.LookupRoomByIDParams{SchoolID: schoolID, ID: id})
	if err != nil {
		return RoomRef{}, mapNoRows(err)
	}
	return RoomRef{ID: row.ID, Name: row.Name}, nil
}

func (r *Repository) LookupClassRef(ctx context.Context, schoolID, id int64) (ClassRef, error) {
	row, err := r.q.LookupClassRef(ctx, teachingdb.LookupClassRefParams{SchoolID: schoolID, ID: id})
	if err != nil {
		return ClassRef{}, mapNoRows(err)
	}
	return ClassRef{ID: row.ID, Name: row.Name}, nil
}

func (r *Repository) LookupSubjectRef(ctx context.Context, schoolID, id int64) (SubjectRef, error) {
	row, err := r.q.LookupSubjectRef(ctx, teachingdb.LookupSubjectRefParams{SchoolID: schoolID, ID: id})
	if err != nil {
		return SubjectRef{}, mapNoRows(err)
	}
	return SubjectRef{ID: row.ID, Code: row.Code, Name: row.Name}, nil
}

func (r *Repository) LookupTeacherUserID(ctx context.Context, schoolID, teacherID int64) (int64, error) {
	id, err := r.q.LookupTeacherUserID(ctx, teachingdb.LookupTeacherUserIDParams{ID: teacherID, SchoolID: schoolID})
	if err != nil {
		return 0, mapNoRows(err)
	}
	return id, nil
}

func (r *Repository) LookupTeacherRef(ctx context.Context, schoolID, teacherID int64) (TeacherRef, error) {
	row, err := r.q.LookupTeacherRef(ctx, teachingdb.LookupTeacherRefParams{ID: teacherID, SchoolID: schoolID})
	if err != nil {
		return TeacherRef{}, mapNoRows(err)
	}
	return TeacherRef{ID: row.ID, Name: row.Name}, nil
}

// -- settings --

// GetSettings membaca school_settings module='teaching' langsung (pola yang
// sama dipakai internal/attendance/repository.go) dan mengembalikan default
// bila sekolah belum pernah menyimpan pengaturannya.
func (r *Repository) GetSettings(ctx context.Context, schoolID int64) (Settings, error) {
	raw, err := r.q.GetTeachingSettingsRaw(ctx, schoolID)
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
