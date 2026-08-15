package schedule

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/schedule/scheduledb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul schedule.
var ErrNotFound = errors.New("schedule: data tidak ditemukan")

// ErrConflict menandai pelanggaran unique constraint (mis. nama ruangan
// dipakai lagi di sekolah yang sama).
var ErrConflict = errors.New("schedule: data sudah ada")

// isUniqueViolation melaporkan apakah err adalah pelanggaran unique
// constraint Postgres (kode 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// scheduleRepository adalah kontrak yang dibutuhkan Service dari repository —
// dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB — pola
// yang sama dengan internal/attendance dan internal/leave.
type scheduleRepository interface {
	// periods
	ListPeriods(ctx context.Context, schoolID int64) ([]PeriodRecord, error)
	ReplacePeriods(ctx context.Context, schoolID int64, periods []PeriodInput) ([]PeriodRecord, error)
	UsedPeriodNumbers(ctx context.Context, schoolID int64) (map[int]bool, error)

	// rooms
	CreateRoom(ctx context.Context, schoolID int64, name, qrToken string) (RoomRecord, error)
	UpdateRoomName(ctx context.Context, schoolID, id int64, name string) (RoomRecord, error)
	RegenerateRoomQR(ctx context.Context, schoolID, id int64, qrToken string) (RoomRecord, error)
	GetRoomByID(ctx context.Context, schoolID, id int64) (RoomRecord, error)
	ListRooms(ctx context.Context, schoolID int64) ([]RoomRecord, error)
	DeleteRoom(ctx context.Context, schoolID, id int64) error
	CountSlotsForRoom(ctx context.Context, schoolID, roomID int64) (int64, error)

	// referensi
	GetClassRef(ctx context.Context, schoolID, id int64) (ClassRefRow, error)
	GetSubjectRef(ctx context.Context, schoolID, id int64) (SubjectRef, error)
	GetTeacherRef(ctx context.Context, schoolID, id int64) (TeacherRef, error)
	GetRoomRef(ctx context.Context, schoolID, id int64) (RoomRef, error)
	LookupClassIDByName(ctx context.Context, schoolID, academicYearID int64, name string) (int64, bool, error)
	LookupSubjectByCode(ctx context.Context, schoolID int64, code string) (SubjectRef, bool, error)
	LookupTeacherByEmail(ctx context.Context, schoolID int64, email string) (TeacherRef, bool, error)
	LookupRoomIDByName(ctx context.Context, schoolID int64, name string) (int64, bool, error)

	// slots
	ListSlotsForYear(ctx context.Context, schoolID, academicYearID int64) ([]SlotRecord, error)
	GetSlotByID(ctx context.Context, schoolID, id int64) (SlotRecord, error)
	// CreateSlotChecked/UpdateSlotChecked mengecek bentrok DAN menulis slot
	// dalam SATU transaksi (docs/04-schedule.md "validasi di service dalam
	// satu transaksi") — bila conflicts tidak kosong, tidak ada yang ditulis.
	CreateSlotChecked(ctx context.Context, in SlotInput) (rec SlotRecord, conflicts []SlotRecord, err error)
	UpdateSlotChecked(ctx context.Context, schoolID, id int64, in SlotInput) (rec SlotRecord, conflicts []SlotRecord, err error)
	DeleteSlot(ctx context.Context, schoolID, id int64) error
	CreateSlotsBatch(ctx context.Context, ins []SlotInput) ([]SlotRecord, error)
}

var _ scheduleRepository = (*Repository)(nil)

// Repository membungkus akses data modul schedule (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *scheduledb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: scheduledb.New(pool)}
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

// -- konversi jam (pgtype.Time menyimpan mikrodetik sejak tengah malam) --

func timeOfMinutes(minutes int) pgtype.Time {
	return pgtype.Time{Microseconds: int64(minutes) * 60 * 1_000_000, Valid: true}
}

func minutesOfTime(t pgtype.Time) int {
	return int(t.Microseconds / 1_000_000 / 60)
}

// -- domain records --

type PeriodRecord struct {
	ID       int64
	Number   int
	StartsAt string // "HH:MM"
	EndsAt   string
	Label    string
}

type PeriodInput struct {
	Number   int
	StartsAt string
	EndsAt   string
	Label    string
}

type RoomRecord struct {
	ID      int64
	Name    string
	QRToken string
}

type ClassRefRow struct {
	ID             int64
	Name           string
	AcademicYearID int64
}

// SubjectRef/TeacherRef/RoomRef dipakai DUA peran: shape response JSON
// (disematkan pada SlotView — lihat model.go) DAN hasil query referensi di
// bawah. Bentuknya kebetulan identik (ID+Code+Name / ID+Name), jadi
// didefinisikan SEKALI di model.go, bukan diduplikasi di sini.

// SlotRecord adalah representasi domain lengkap satu baris schedule_slots
// (join dengan nama kelas/mapel/guru/ruang, dipakai membangun SlotView).
type SlotRecord struct {
	ID             int64
	SchoolID       int64
	AcademicYearID int64
	ClassID        int64
	ClassName      string
	SubjectID      int64
	SubjectCode    string
	SubjectName    string
	TeacherID      int64
	TeacherName    string
	RoomID         int64 // 0 = tanpa ruang
	RoomName       string
	DayOfWeek      int
	PeriodStart    int
	PeriodEnd      int
}

// SlotInput adalah parameter tulis (create/update) satu slot.
type SlotInput struct {
	SchoolID       int64
	AcademicYearID int64
	ClassID        int64
	SubjectID      int64
	TeacherID      int64
	RoomID         int64 // 0 = tanpa ruang
	DayOfWeek      int
	PeriodStart    int
	PeriodEnd      int
}

func periodFromDB(p scheduledb.Period) PeriodRecord {
	return PeriodRecord{
		ID: p.ID, Number: int(p.Number), StartsAt: minutesToClock(minutesOfTime(p.StartsAt)),
		EndsAt: minutesToClock(minutesOfTime(p.EndsAt)), Label: p.Label.String,
	}
}

func roomFromDB(r scheduledb.Room) RoomRecord {
	return RoomRecord{ID: r.ID, Name: r.Name, QRToken: r.QrToken}
}

// -- periods --

func (r *Repository) ListPeriods(ctx context.Context, schoolID int64) ([]PeriodRecord, error) {
	rows, err := r.q.ListPeriods(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]PeriodRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, periodFromDB(row))
	}
	return out, nil
}

// ReplacePeriods mengganti SELURUH set periods sekolah dalam satu transaksi
// (DELETE lalu INSERT ulang) — docs/04-schedule.md: replace-all lebih
// sederhana daripada CRUD satuan untuk data kecil begini. Validasi bisnis
// (unik/urut/overlap/dipakai slot) dilakukan di Service SEBELUM memanggil ini.
func (r *Repository) ReplacePeriods(ctx context.Context, schoolID int64, periods []PeriodInput) ([]PeriodRecord, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	if err := qtx.DeletePeriods(ctx, schoolID); err != nil {
		return nil, err
	}
	out := make([]PeriodRecord, 0, len(periods))
	for _, p := range periods {
		row, err := qtx.InsertPeriod(ctx, scheduledb.InsertPeriodParams{
			SchoolID: schoolID, Number: int32(p.Number),
			StartsAt: timeOfMinutes(mustClockMinutes(p.StartsAt)), EndsAt: timeOfMinutes(mustClockMinutes(p.EndsAt)),
			Label: textOrNil(p.Label),
		})
		if err != nil {
			return nil, err
		}
		out = append(out, periodFromDB(row))
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// mustClockMinutes — dipanggil setelah Service memvalidasi format "HH:MM",
// jadi error di sini seharusnya tidak pernah terjadi (defensive: 0 bila gagal
// parse, ditangkap uji unit Service.ReplacePeriods).
func mustClockMinutes(s string) int {
	m, err := clockMinutes(s)
	if err != nil {
		return 0
	}
	return m
}

// UsedPeriodNumbers mengembalikan himpunan nomor jam ke- yang SEDANG dipakai
// oleh schedule_slots manapun di sekolah ini (rentang period_start..period_end
// tiap slot) — dipakai Service.ReplacePeriods menolak penghapusan period yang
// masih dipakai (409).
func (r *Repository) UsedPeriodNumbers(ctx context.Context, schoolID int64) (map[int]bool, error) {
	rows, err := r.q.ListSlotPeriodRanges(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	used := make(map[int]bool)
	for _, row := range rows {
		for n := int(row.PeriodStart); n <= int(row.PeriodEnd); n++ {
			used[n] = true
		}
	}
	return used, nil
}

// -- rooms --

func (r *Repository) CreateRoom(ctx context.Context, schoolID int64, name, qrToken string) (RoomRecord, error) {
	row, err := r.q.CreateRoom(ctx, scheduledb.CreateRoomParams{SchoolID: schoolID, Name: name, QrToken: qrToken})
	if err != nil {
		if isUniqueViolation(err) {
			return RoomRecord{}, ErrConflict
		}
		return RoomRecord{}, err
	}
	return roomFromDB(row), nil
}

func (r *Repository) UpdateRoomName(ctx context.Context, schoolID, id int64, name string) (RoomRecord, error) {
	row, err := r.q.UpdateRoomName(ctx, scheduledb.UpdateRoomNameParams{ID: id, SchoolID: schoolID, Name: name})
	if err != nil {
		if isUniqueViolation(err) {
			return RoomRecord{}, ErrConflict
		}
		return RoomRecord{}, mapNoRows(err)
	}
	return roomFromDB(row), nil
}

func (r *Repository) RegenerateRoomQR(ctx context.Context, schoolID, id int64, qrToken string) (RoomRecord, error) {
	row, err := r.q.RegenerateRoomQRToken(ctx, scheduledb.RegenerateRoomQRTokenParams{ID: id, SchoolID: schoolID, QrToken: qrToken})
	if err != nil {
		return RoomRecord{}, mapNoRows(err)
	}
	return roomFromDB(row), nil
}

func (r *Repository) GetRoomByID(ctx context.Context, schoolID, id int64) (RoomRecord, error) {
	row, err := r.q.GetRoomByID(ctx, scheduledb.GetRoomByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return RoomRecord{}, mapNoRows(err)
	}
	return roomFromDB(row), nil
}

func (r *Repository) ListRooms(ctx context.Context, schoolID int64) ([]RoomRecord, error) {
	rows, err := r.q.ListRooms(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]RoomRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, roomFromDB(row))
	}
	return out, nil
}

func (r *Repository) DeleteRoom(ctx context.Context, schoolID, id int64) error {
	return r.q.DeleteRoom(ctx, scheduledb.DeleteRoomParams{ID: id, SchoolID: schoolID})
}

func (r *Repository) CountSlotsForRoom(ctx context.Context, schoolID, roomID int64) (int64, error) {
	return r.q.CountSlotsForRoom(ctx, scheduledb.CountSlotsForRoomParams{RoomID: int8OrNil(roomID), SchoolID: schoolID})
}

// -- referensi --

func (r *Repository) GetClassRef(ctx context.Context, schoolID, id int64) (ClassRefRow, error) {
	row, err := r.q.GetClassRef(ctx, scheduledb.GetClassRefParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return ClassRefRow{}, mapNoRows(err)
	}
	return ClassRefRow{ID: row.ID, Name: row.Name, AcademicYearID: row.AcademicYearID}, nil
}

func (r *Repository) GetSubjectRef(ctx context.Context, schoolID, id int64) (SubjectRef, error) {
	row, err := r.q.GetSubjectRef(ctx, scheduledb.GetSubjectRefParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return SubjectRef{}, mapNoRows(err)
	}
	return SubjectRef{ID: row.ID, Code: row.Code, Name: row.Name}, nil
}

func (r *Repository) GetTeacherRef(ctx context.Context, schoolID, id int64) (TeacherRef, error) {
	row, err := r.q.GetTeacherRef(ctx, scheduledb.GetTeacherRefParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return TeacherRef{}, mapNoRows(err)
	}
	return TeacherRef{ID: row.ID, Name: row.Name}, nil
}

func (r *Repository) GetRoomRef(ctx context.Context, schoolID, id int64) (RoomRef, error) {
	row, err := r.q.GetRoomRef(ctx, scheduledb.GetRoomRefParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return RoomRef{}, mapNoRows(err)
	}
	return RoomRef{ID: row.ID, Name: row.Name}, nil
}

func (r *Repository) LookupClassIDByName(ctx context.Context, schoolID, academicYearID int64, name string) (int64, bool, error) {
	id, err := r.q.LookupClassIDByName(ctx, scheduledb.LookupClassIDByNameParams{SchoolID: schoolID, AcademicYearID: academicYearID, Name: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

func (r *Repository) LookupSubjectByCode(ctx context.Context, schoolID int64, code string) (SubjectRef, bool, error) {
	row, err := r.q.LookupSubjectByCode(ctx, scheduledb.LookupSubjectByCodeParams{SchoolID: schoolID, Code: code})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubjectRef{}, false, nil
		}
		return SubjectRef{}, false, err
	}
	return SubjectRef{ID: row.ID, Code: row.Code, Name: row.Name}, true, nil
}

func (r *Repository) LookupTeacherByEmail(ctx context.Context, schoolID int64, email string) (TeacherRef, bool, error) {
	row, err := r.q.LookupTeacherByEmail(ctx, scheduledb.LookupTeacherByEmailParams{SchoolID: schoolID, Email: textOrNil(email)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TeacherRef{}, false, nil
		}
		return TeacherRef{}, false, err
	}
	return TeacherRef{ID: row.ID, Name: row.Name}, true, nil
}

func (r *Repository) LookupRoomIDByName(ctx context.Context, schoolID int64, name string) (int64, bool, error) {
	id, err := r.q.LookupRoomIDByName(ctx, scheduledb.LookupRoomIDByNameParams{SchoolID: schoolID, Name: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

// -- slots --

func slotFromDB(row slotJoinRow) SlotRecord {
	return SlotRecord{
		ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID,
		ClassID: row.ClassID, ClassName: row.ClassName,
		SubjectID: row.SubjectID, SubjectCode: row.SubjectCode, SubjectName: row.SubjectName,
		TeacherID: row.TeacherID, TeacherName: row.TeacherName,
		RoomID: int8ToInt64(row.RoomID), RoomName: row.RoomName,
		DayOfWeek: int(row.DayOfWeek), PeriodStart: int(row.PeriodStart), PeriodEnd: int(row.PeriodEnd),
	}
}

// slotJoinRow adapts baik scheduledb.ListSlotsForYearRow maupun
// scheduledb.GetSlotByIDRow (bentuknya identik, hasil sqlc generate terpisah
// per query) ke satu tipe supaya slotFromDB tidak duplikat.
type slotJoinRow struct {
	ID             int64
	SchoolID       int64
	AcademicYearID int64
	ClassID        int64
	ClassName      string
	SubjectID      int64
	SubjectCode    string
	SubjectName    string
	TeacherID      int64
	TeacherName    string
	RoomID         pgtype.Int8
	RoomName       string
	DayOfWeek      int32
	PeriodStart    int32
	PeriodEnd      int32
}

func (r *Repository) ListSlotsForYear(ctx context.Context, schoolID, academicYearID int64) ([]SlotRecord, error) {
	rows, err := r.q.ListSlotsForYear(ctx, scheduledb.ListSlotsForYearParams{SchoolID: schoolID, AcademicYearID: academicYearID})
	if err != nil {
		return nil, err
	}
	out := make([]SlotRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, slotFromDB(slotJoinRow{
			ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID,
			ClassID: row.ClassID, ClassName: row.ClassName,
			SubjectID: row.SubjectID, SubjectCode: row.SubjectCode, SubjectName: row.SubjectName,
			TeacherID: row.TeacherID, TeacherName: row.TeacherName,
			RoomID: row.RoomID, RoomName: row.RoomName,
			DayOfWeek: row.DayOfWeek, PeriodStart: row.PeriodStart, PeriodEnd: row.PeriodEnd,
		}))
	}
	return out, nil
}

func (r *Repository) GetSlotByID(ctx context.Context, schoolID, id int64) (SlotRecord, error) {
	row, err := r.q.GetSlotByID(ctx, scheduledb.GetSlotByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return SlotRecord{}, mapNoRows(err)
	}
	return slotFromDB(slotJoinRow{
		ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID,
		ClassID: row.ClassID, ClassName: row.ClassName,
		SubjectID: row.SubjectID, SubjectCode: row.SubjectCode, SubjectName: row.SubjectName,
		TeacherID: row.TeacherID, TeacherName: row.TeacherName,
		RoomID: row.RoomID, RoomName: row.RoomName,
		DayOfWeek: row.DayOfWeek, PeriodStart: row.PeriodStart, PeriodEnd: row.PeriodEnd,
	}), nil
}

func insertSlotParams(in SlotInput) scheduledb.InsertSlotParams {
	return scheduledb.InsertSlotParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, ClassID: in.ClassID,
		SubjectID: in.SubjectID, RoomID: int8OrNil(in.RoomID), TeacherID: in.TeacherID,
		DayOfWeek: int32(in.DayOfWeek), PeriodStart: int32(in.PeriodStart), PeriodEnd: int32(in.PeriodEnd),
	}
}

// slotsForYearTx memuat seluruh slot tahun ajaran tsb DI DALAM transaksi qtx
// (dipakai CreateSlotChecked/UpdateSlotChecked supaya cek bentrok & tulis
// konsisten dalam satu transaksi).
func slotsForYearTx(ctx context.Context, qtx *scheduledb.Queries, schoolID, academicYearID int64) ([]SlotRecord, error) {
	rows, err := qtx.ListSlotsForYear(ctx, scheduledb.ListSlotsForYearParams{SchoolID: schoolID, AcademicYearID: academicYearID})
	if err != nil {
		return nil, err
	}
	out := make([]SlotRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, slotFromDB(slotJoinRow{
			ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID,
			ClassID: row.ClassID, ClassName: row.ClassName,
			SubjectID: row.SubjectID, SubjectCode: row.SubjectCode, SubjectName: row.SubjectName,
			TeacherID: row.TeacherID, TeacherName: row.TeacherName,
			RoomID: row.RoomID, RoomName: row.RoomName,
			DayOfWeek: row.DayOfWeek, PeriodStart: row.PeriodStart, PeriodEnd: row.PeriodEnd,
		}))
	}
	return out, nil
}

func getSlotByIDTx(ctx context.Context, qtx *scheduledb.Queries, schoolID, id int64) (SlotRecord, error) {
	row, err := qtx.GetSlotByID(ctx, scheduledb.GetSlotByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return SlotRecord{}, mapNoRows(err)
	}
	return slotFromDB(slotJoinRow{
		ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID,
		ClassID: row.ClassID, ClassName: row.ClassName,
		SubjectID: row.SubjectID, SubjectCode: row.SubjectCode, SubjectName: row.SubjectName,
		TeacherID: row.TeacherID, TeacherName: row.TeacherName,
		RoomID: row.RoomID, RoomName: row.RoomName,
		DayOfWeek: row.DayOfWeek, PeriodStart: row.PeriodStart, PeriodEnd: row.PeriodEnd,
	}), nil
}

// CreateSlotChecked lihat dokumentasi interface di atas.
func (r *Repository) CreateSlotChecked(ctx context.Context, in SlotInput) (SlotRecord, []SlotRecord, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return SlotRecord{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	existing, err := slotsForYearTx(ctx, qtx, in.SchoolID, in.AcademicYearID)
	if err != nil {
		return SlotRecord{}, nil, err
	}
	conflicts := findConflicts(existing, 0, in.DayOfWeek, in.PeriodStart, in.PeriodEnd, in.TeacherID, in.ClassID, in.RoomID)
	if len(conflicts) > 0 {
		return SlotRecord{}, conflicts, nil
	}

	id, err := qtx.InsertSlot(ctx, insertSlotParams(in))
	if err != nil {
		return SlotRecord{}, nil, err
	}
	rec, err := getSlotByIDTx(ctx, qtx, in.SchoolID, id)
	if err != nil {
		return SlotRecord{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SlotRecord{}, nil, err
	}
	return rec, nil, nil
}

// UpdateSlotChecked lihat dokumentasi interface di atas.
func (r *Repository) UpdateSlotChecked(ctx context.Context, schoolID, id int64, in SlotInput) (SlotRecord, []SlotRecord, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return SlotRecord{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	if _, err := getSlotByIDTx(ctx, qtx, schoolID, id); err != nil {
		return SlotRecord{}, nil, err
	}

	existing, err := slotsForYearTx(ctx, qtx, schoolID, in.AcademicYearID)
	if err != nil {
		return SlotRecord{}, nil, err
	}
	conflicts := findConflicts(existing, id, in.DayOfWeek, in.PeriodStart, in.PeriodEnd, in.TeacherID, in.ClassID, in.RoomID)
	if len(conflicts) > 0 {
		return SlotRecord{}, conflicts, nil
	}

	if err := qtx.UpdateSlot(ctx, scheduledb.UpdateSlotParams{
		ID: id, SchoolID: schoolID, ClassID: in.ClassID, SubjectID: in.SubjectID, TeacherID: in.TeacherID,
		RoomID: int8OrNil(in.RoomID), DayOfWeek: int32(in.DayOfWeek), PeriodStart: int32(in.PeriodStart), PeriodEnd: int32(in.PeriodEnd),
	}); err != nil {
		return SlotRecord{}, nil, err
	}
	rec, err := getSlotByIDTx(ctx, qtx, schoolID, id)
	if err != nil {
		return SlotRecord{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SlotRecord{}, nil, err
	}
	return rec, nil, nil
}

func (r *Repository) DeleteSlot(ctx context.Context, schoolID, id int64) error {
	return r.q.DeleteSlot(ctx, scheduledb.DeleteSlotParams{ID: id, SchoolID: schoolID})
}

// CreateSlotsBatch menulis sekumpulan slot dalam satu transaksi (dipakai
// copy jadwal & commit import) — pemanggil (Service) SUDAH memvalidasi tidak
// ada bentrok di antara baris yang akan ditulis SEBELUM memanggil ini.
func (r *Repository) CreateSlotsBatch(ctx context.Context, ins []SlotInput) ([]SlotRecord, error) {
	if len(ins) == 0 {
		return nil, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	ids := make([]int64, 0, len(ins))
	schoolID := ins[0].SchoolID
	for _, in := range ins {
		id, err := qtx.InsertSlot(ctx, insertSlotParams(in))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	out := make([]SlotRecord, 0, len(ids))
	for _, id := range ids {
		row, err := qtx.GetSlotByID(ctx, scheduledb.GetSlotByIDParams{ID: id, SchoolID: schoolID})
		if err != nil {
			return nil, err
		}
		out = append(out, slotFromDB(slotJoinRow{
			ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID,
			ClassID: row.ClassID, ClassName: row.ClassName,
			SubjectID: row.SubjectID, SubjectCode: row.SubjectCode, SubjectName: row.SubjectName,
			TeacherID: row.TeacherID, TeacherName: row.TeacherName,
			RoomID: row.RoomID, RoomName: row.RoomName,
			DayOfWeek: row.DayOfWeek, PeriodStart: row.PeriodStart, PeriodEnd: row.PeriodEnd,
		}))
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
