package schedule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interfaces (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject lewat constructor di
// main.go). Semua signature sengaja hanya memakai tipe primitif supaya
// *identity.Service, *tenant.Service, dan *student.Service memenuhi interface
// ini secara struktural TANPA schedule perlu mengimpor package-package itu.

// IdentityGateway adalah kebutuhan modul schedule dari modul identity:
// gerbang permission (rooms qr_token hanya utk schedule:manage) dan audit log.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// AcademicYearLookup adalah kebutuhan modul schedule dari modul tenant.
type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// StudentClassLookup adalah kebutuhan modul schedule dari modul student:
// object-level check (siswa hanya boleh melihat jadwal rombelnya sendiri).
// Dipenuhi *student.Service secara struktural lewat method MyClassID.
type StudentClassLookup interface {
	MyClassID(ctx context.Context, schoolID, userID int64) (classID int64, ok bool, err error)
	// MyTeacherID: infer jadwal "milik saya" untuk role guru tanpa parameter.
	MyTeacherID(ctx context.Context, schoolID, userID int64) (teacherID int64, ok bool, err error)
}

// ErrNoActiveAcademicYear — sekolah belum mengaktifkan tahun ajaran.
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// Realtime adalah kebutuhan modul schedule dari modul realtime (Fase 12, bus
// event WebSocket read-only server->client) — consumer-side interface kecil
// dideklarasikan di sisi PEMAKAI (lihat CLAUDE.md). TIDAK dipenuhi
// *realtime.Hub secara langsung — dijembatani adapter kecil di cmd/server
// (realtimeadapter.go).
type Realtime interface {
	Publish(schoolID int64, eventType string, data map[string]any)
}

// Service berisi aturan bisnis modul schedule.
type Service struct {
	repo     scheduleRepository
	identity IdentityGateway
	years    AcademicYearLookup
	students StudentClassLookup
	clock    clock.Clock
	imports  *importStore
	realtime Realtime // opsional — nil = event realtime dilewati (lihat SetRealtime)
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup, students StudentClassLookup, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, years: years, students: students, clock: clk, imports: newImportStore()}
}

// SetRealtime menyuntikkan Realtime SETELAH konstruksi (opsional, disuntik
// main.go — pola yang sama dengan modul lain; nil aman/no-op).
func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// emitSchedule memancarkan "schedule" {} broadcast satu sekolah (docs tugas
// Fase 12: "slot create/update/delete/copy/import commit & periods
// replace") — dipanggil SETELAH operasi sukses, best-effort (nil-safe).
// Data SENGAJA kosong — klien selalu refetch endpoint schedule yang relevan
// (bus read-only).
func (s *Service) emitSchedule(schoolID int64) {
	if s.realtime == nil {
		return
	}
	s.realtime.Publish(schoolID, "schedule", map[string]any{})
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo scheduleRepository, identity IdentityGateway, years AcademicYearLookup, students StudentClassLookup, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, years: years, students: students, clock: clk, imports: newImportStore()}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

// resolveYear mengembalikan yearID apa adanya bila != 0, atau tahun ajaran
// AKTIF sekolah bila 0 (default endpoint yang tidak menyebut academic_year_id
// eksplisit — docs/04-schedule.md).
func (s *Service) resolveYear(ctx context.Context, schoolID, yearID int64) (int64, error) {
	if yearID != 0 {
		return yearID, nil
	}
	id, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoActiveAcademicYear
	}
	return id, nil
}

// Now — dipakai handler supaya waktu "sekarang" lewat clock (bisa dites).
func (s *Service) Now() time.Time { return s.clock.Now() }

// -- periods --

func periodViews(recs []PeriodRecord) []PeriodView {
	out := make([]PeriodView, 0, len(recs))
	for _, r := range recs {
		out = append(out, PeriodView{ID: r.ID, Number: r.Number, StartsAt: r.StartsAt, EndsAt: r.EndsAt, Label: r.Label})
	}
	return out
}

func (s *Service) ListPeriods(ctx context.Context, schoolID int64) ([]PeriodView, error) {
	recs, err := s.repo.ListPeriods(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	return periodViews(recs), nil
}

// ReplacePeriodInput adalah satu baris body PUT /api/periods.
type ReplacePeriodInput struct {
	Number   int
	StartsAt string
	EndsAt   string
	Label    string
}

type parsedPeriod struct {
	Number   int
	StartMin int
	EndMin   int
	Label    string
}

// ReplacePeriods mengganti SELURUH set periods sekolah (docs/04-schedule.md:
// replace-all transaksional). Validasi: number unik & berurutan mulai dari 1
// (tanpa lompat), starts_at < ends_at, dan tidak ada dua period yang
// tumpang-tindih jamnya. Menolak (409) bila ada nomor jam ke- yang sedang
// dipakai schedule_slots tapi hilang dari set baru.
func (s *Service) ReplacePeriods(ctx context.Context, actorUserID, schoolID int64, in []ReplacePeriodInput) ([]PeriodView, error) {
	if len(in) == 0 {
		return nil, httpx.Validation("Minimal satu jam pelajaran wajib diisi.")
	}

	seen := make(map[int]bool, len(in))
	items := make([]parsedPeriod, 0, len(in))
	for _, p := range in {
		if p.Number <= 0 {
			return nil, httpx.Validation("Nomor jam ke- harus lebih dari 0.")
		}
		if seen[p.Number] {
			return nil, httpx.Validation(fmt.Sprintf("Nomor jam ke-%d duplikat.", p.Number))
		}
		seen[p.Number] = true
		startMin, err := clockMinutes(p.StartsAt)
		if err != nil {
			return nil, httpx.Validation(fmt.Sprintf("Jam ke-%d: %s", p.Number, err.Error()))
		}
		endMin, err := clockMinutes(p.EndsAt)
		if err != nil {
			return nil, httpx.Validation(fmt.Sprintf("Jam ke-%d: %s", p.Number, err.Error()))
		}
		if startMin >= endMin {
			return nil, httpx.Validation(fmt.Sprintf("Jam ke-%d: jam mulai harus sebelum jam selesai.", p.Number))
		}
		items = append(items, parsedPeriod{Number: p.Number, StartMin: startMin, EndMin: endMin, Label: strings.TrimSpace(p.Label)})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Number < items[j].Number })
	for i, it := range items {
		if it.Number != i+1 {
			return nil, httpx.Validation("Nomor jam ke- harus unik dan berurutan mulai dari 1 tanpa lompat.")
		}
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].StartMin < items[j].EndMin && items[i].EndMin > items[j].StartMin {
				return nil, httpx.Validation(fmt.Sprintf("Jam ke-%d dan jam ke-%d tumpang tindih.", items[i].Number, items[j].Number))
			}
		}
	}

	used, err := s.repo.UsedPeriodNumbers(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	newNumbers := make(map[int]bool, len(items))
	for _, it := range items {
		newNumbers[it.Number] = true
	}
	var missing []string
	for n := range used {
		if !newNumbers[n] {
			missing = append(missing, strconv.Itoa(n))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, &httpx.Error{
			Status: http.StatusConflict, Code: "period_in_use",
			Message: fmt.Sprintf("Jam ke-%s masih dipakai jadwal yang ada — ubah/hapus jadwalnya dulu sebelum menghapus jam ini.", strings.Join(missing, ", ")),
		}
	}

	repoIn := make([]PeriodInput, 0, len(items))
	for _, it := range items {
		repoIn = append(repoIn, PeriodInput{Number: it.Number, StartsAt: minutesToClock(it.StartMin), EndsAt: minutesToClock(it.EndMin), Label: it.Label})
	}
	recs, err := s.repo.ReplacePeriods(ctx, schoolID, repoIn)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.periods_replace", "period", 0, nil, map[string]any{"count": len(recs)})
	s.emitSchedule(schoolID)
	return periodViews(recs), nil
}

// -- rooms --

func randomQRToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func roomView(rec RoomRecord, includeToken bool) RoomView {
	v := RoomView{ID: rec.ID, Name: rec.Name}
	if includeToken {
		t := rec.QRToken
		v.QRToken = &t
	}
	return v
}

// ListRooms — qr_token HANYA disertakan untuk requester dengan permission
// schedule:manage (docs/04-schedule.md).
func (s *Service) ListRooms(ctx context.Context, schoolID int64) ([]RoomView, error) {
	canManage := s.identity.HasPermission(reqctx.Role(ctx), PermScheduleManage)
	recs, err := s.repo.ListRooms(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]RoomView, 0, len(recs))
	for _, r := range recs {
		out = append(out, roomView(r, canManage))
	}
	return out, nil
}

func (s *Service) CreateRoom(ctx context.Context, actorUserID, schoolID int64, name string) (RoomView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return RoomView{}, httpx.Validation("Nama ruangan wajib diisi.")
	}
	token, err := randomQRToken()
	if err != nil {
		return RoomView{}, err
	}
	rec, err := s.repo.CreateRoom(ctx, schoolID, name, token)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return RoomView{}, httpx.Validation("Nama ruangan sudah dipakai.")
		}
		return RoomView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.room_create", "room", rec.ID, nil, map[string]any{"name": name})
	return roomView(rec, true), nil
}

func (s *Service) UpdateRoom(ctx context.Context, actorUserID, schoolID, id int64, name string) (RoomView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return RoomView{}, httpx.Validation("Nama ruangan wajib diisi.")
	}
	rec, err := s.repo.UpdateRoomName(ctx, schoolID, id, name)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return RoomView{}, httpx.Validation("Nama ruangan sudah dipakai.")
		}
		if errors.Is(err, ErrNotFound) {
			return RoomView{}, httpx.ErrNotFound
		}
		return RoomView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.room_update", "room", id, nil, map[string]any{"name": name})
	return roomView(rec, true), nil
}

// DeleteRoom menolak (409) bila ruangan masih dipakai schedule_slots manapun
// (docs/04-schedule.md).
func (s *Service) DeleteRoom(ctx context.Context, actorUserID, schoolID, id int64) error {
	if _, err := s.repo.GetRoomByID(ctx, schoolID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	count, err := s.repo.CountSlotsForRoom(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return &httpx.Error{
			Status: http.StatusConflict, Code: "room_in_use",
			Message: fmt.Sprintf("Ruangan ini masih dipakai %d jadwal — ubah/hapus jadwalnya dulu sebelum menghapus ruangan.", count),
		}
	}
	if err := s.repo.DeleteRoom(ctx, schoolID, id); err != nil {
		return err
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.room_delete", "room", id, nil, nil)
	return nil
}

func (s *Service) RegenerateRoomQR(ctx context.Context, actorUserID, schoolID, id int64) (RoomView, error) {
	token, err := randomQRToken()
	if err != nil {
		return RoomView{}, err
	}
	rec, err := s.repo.RegenerateRoomQR(ctx, schoolID, id, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return RoomView{}, httpx.ErrNotFound
		}
		return RoomView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.room_qr_regenerate", "room", id, nil, nil)
	return roomView(rec, true), nil
}

// RoomQRPNG menghasilkan gambar PNG QR (512x512) berisi string
// "nouschool:room:{qr_token}" (docs/04, dipakai fase 6 scan ruangan).
func (s *Service) RoomQRPNG(ctx context.Context, schoolID, id int64) ([]byte, error) {
	rec, err := s.repo.GetRoomByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.ErrNotFound
		}
		return nil, err
	}
	return generateRoomQRPNG(rec.QRToken)
}

// -- slots --

func slotView(rec SlotRecord) SlotView {
	v := SlotView{
		ID:          rec.ID,
		Class:       ClassRef{ID: rec.ClassID, Name: rec.ClassName},
		Subject:     SubjectRef{ID: rec.SubjectID, Code: rec.SubjectCode, Name: rec.SubjectName},
		Teacher:     TeacherRef{ID: rec.TeacherID, Name: rec.TeacherName},
		DayOfWeek:   rec.DayOfWeek,
		PeriodStart: rec.PeriodStart,
		PeriodEnd:   rec.PeriodEnd,
	}
	if rec.RoomID != 0 {
		v.Room = &RoomRef{ID: rec.RoomID, Name: rec.RoomName}
	}
	return v
}

func validateSlotShape(dayOfWeek, periodStart, periodEnd int) error {
	// 0 (Minggu) diizinkan sejak fase 6 (lihat komentar dayNames di model.go)
	// — sebelumnya hanya 1 (Senin) sampai 6 (Sabtu).
	if dayOfWeek < 0 || dayOfWeek > 6 {
		return httpx.Validation("Hari harus 0 (Minggu) sampai 6 (Sabtu).")
	}
	if periodStart <= 0 || periodEnd <= 0 {
		return httpx.Validation("Jam ke- harus lebih dari 0.")
	}
	if periodStart > periodEnd {
		return httpx.Validation("Jam mulai tidak boleh setelah jam selesai.")
	}
	return nil
}

func (s *Service) validateSlotRefs(ctx context.Context, schoolID, yearID, classID, subjectID, teacherID, roomID int64) error {
	cls, err := s.repo.GetClassRef(ctx, schoolID, classID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.Validation("Rombel tidak ditemukan.")
		}
		return err
	}
	if cls.AcademicYearID != yearID {
		return httpx.Validation("Rombel tidak terdaftar pada tahun ajaran ini.")
	}
	if _, err := s.repo.GetSubjectRef(ctx, schoolID, subjectID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.Validation("Mapel tidak ditemukan.")
		}
		return err
	}
	if _, err := s.repo.GetTeacherRef(ctx, schoolID, teacherID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.Validation("Guru tidak ditemukan.")
		}
		return err
	}
	if roomID != 0 {
		if _, err := s.repo.GetRoomRef(ctx, schoolID, roomID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return httpx.Validation("Ruangan tidak ditemukan.")
			}
			return err
		}
	}
	return nil
}

// SlotInputRequest adalah parameter POST/PATCH satu slot (bentuk sudah
// divalidasi handler, aturan bisnis di service — lihat CLAUDE.md).
type SlotInputRequest struct {
	ClassID        int64
	SubjectID      int64
	TeacherID      int64
	RoomID         int64 // 0 = tanpa ruang
	DayOfWeek      int
	PeriodStart    int
	PeriodEnd      int
	AcademicYearID int64 // 0 = tahun ajaran aktif (create) / tahun ajaran slot saat ini (update)
}

// CreateSlot membuat satu slot jadwal. Deteksi bentrok (guru/kelas/ruang
// beririsan hari+jam, TA sama) dilakukan REPOSITORY dalam satu transaksi
// (docs/04-schedule.md "validasi di service dalam satu transaksi" —
// implementasi konkretnya CreateSlotChecked, lihat repository.go).
func (s *Service) CreateSlot(ctx context.Context, actorUserID, schoolID int64, in SlotInputRequest) (SlotView, error) {
	if err := validateSlotShape(in.DayOfWeek, in.PeriodStart, in.PeriodEnd); err != nil {
		return SlotView{}, err
	}
	yearID, err := s.resolveYear(ctx, schoolID, in.AcademicYearID)
	if err != nil {
		return SlotView{}, err
	}
	if err := s.validateSlotRefs(ctx, schoolID, yearID, in.ClassID, in.SubjectID, in.TeacherID, in.RoomID); err != nil {
		return SlotView{}, err
	}

	rec, conflicts, err := s.repo.CreateSlotChecked(ctx, SlotInput{
		SchoolID: schoolID, AcademicYearID: yearID, ClassID: in.ClassID, SubjectID: in.SubjectID,
		TeacherID: in.TeacherID, RoomID: in.RoomID, DayOfWeek: in.DayOfWeek, PeriodStart: in.PeriodStart, PeriodEnd: in.PeriodEnd,
	})
	if err != nil {
		return SlotView{}, err
	}
	if len(conflicts) > 0 {
		return SlotView{}, httpx.Validation(conflictMessage(in.DayOfWeek, conflicts, in.TeacherID, in.ClassID, in.RoomID))
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.slot_create", "schedule_slot", rec.ID, nil, map[string]any{
		"class_id": rec.ClassID, "subject_id": rec.SubjectID, "teacher_id": rec.TeacherID,
		"day_of_week": rec.DayOfWeek, "period_start": rec.PeriodStart, "period_end": rec.PeriodEnd,
	})
	s.emitSchedule(schoolID)
	return slotView(rec), nil
}

func (s *Service) UpdateSlot(ctx context.Context, actorUserID, schoolID, id int64, in SlotInputRequest) (SlotView, error) {
	if err := validateSlotShape(in.DayOfWeek, in.PeriodStart, in.PeriodEnd); err != nil {
		return SlotView{}, err
	}
	current, err := s.repo.GetSlotByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SlotView{}, httpx.ErrNotFound
		}
		return SlotView{}, err
	}
	yearID := in.AcademicYearID
	if yearID == 0 {
		yearID = current.AcademicYearID
	}
	if err := s.validateSlotRefs(ctx, schoolID, yearID, in.ClassID, in.SubjectID, in.TeacherID, in.RoomID); err != nil {
		return SlotView{}, err
	}

	rec, conflicts, err := s.repo.UpdateSlotChecked(ctx, schoolID, id, SlotInput{
		SchoolID: schoolID, AcademicYearID: yearID, ClassID: in.ClassID, SubjectID: in.SubjectID,
		TeacherID: in.TeacherID, RoomID: in.RoomID, DayOfWeek: in.DayOfWeek, PeriodStart: in.PeriodStart, PeriodEnd: in.PeriodEnd,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SlotView{}, httpx.ErrNotFound
		}
		return SlotView{}, err
	}
	if len(conflicts) > 0 {
		return SlotView{}, httpx.Validation(conflictMessage(in.DayOfWeek, conflicts, in.TeacherID, in.ClassID, in.RoomID))
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.slot_update", "schedule_slot", id, nil, map[string]any{
		"class_id": rec.ClassID, "subject_id": rec.SubjectID, "teacher_id": rec.TeacherID,
		"day_of_week": rec.DayOfWeek, "period_start": rec.PeriodStart, "period_end": rec.PeriodEnd,
	})
	s.emitSchedule(schoolID)
	return slotView(rec), nil
}

func (s *Service) DeleteSlot(ctx context.Context, actorUserID, schoolID, id int64) error {
	old, err := s.repo.GetSlotByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	if err := s.repo.DeleteSlot(ctx, schoolID, id); err != nil {
		return err
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.slot_delete", "schedule_slot", id, map[string]any{
		"class_id": old.ClassID, "subject_id": old.SubjectID, "teacher_id": old.TeacherID,
		"day_of_week": old.DayOfWeek, "period_start": old.PeriodStart, "period_end": old.PeriodEnd,
	}, nil)
	s.emitSchedule(schoolID)
	return nil
}

// -- GET /api/schedule/slots --

// ListSlotsQuery adalah parameter GET /api/schedule/slots.
type ListSlotsQuery struct {
	ClassID        int64
	TeacherID      int64
	AcademicYearID int64
}

// ListSlots — object-level: siswa hanya boleh query class_id rombelnya
// sendiri (docs/04-schedule.md); guru/orang_tua/admin/kepsek bebas baca
// (jadwal sekolah itu publik internal).
func (s *Service) ListSlots(ctx context.Context, schoolID int64, in ListSlotsQuery) ([]SlotView, error) {
	role := reqctx.Role(ctx)
	if role == RoleSiswa {
		myClassID, ok, err := s.students.MyClassID(ctx, schoolID, reqctx.UserID(ctx))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.ErrForbidden
		}
		if in.ClassID != 0 && in.ClassID != myClassID {
			return nil, httpx.ErrForbidden
		}
		in.ClassID = myClassID
		in.TeacherID = 0
	}
	if role == RoleGuru && in.ClassID == 0 && in.TeacherID == 0 {
		// Guru tanpa parameter = jadwal miliknya sendiri.
		tid, ok, err := s.students.MyTeacherID(ctx, schoolID, reqctx.UserID(ctx))
		if err != nil {
			return nil, err
		}
		if ok {
			in.TeacherID = tid
		}
	}
	if in.ClassID == 0 && in.TeacherID == 0 {
		return nil, httpx.Validation("class_id atau teacher_id wajib diisi.")
	}

	yearID, err := s.resolveYear(ctx, schoolID, in.AcademicYearID)
	if err != nil {
		return nil, err
	}
	all, err := s.repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return nil, err
	}
	out := make([]SlotView, 0)
	for _, sl := range all {
		if in.ClassID != 0 && sl.ClassID != in.ClassID {
			continue
		}
		if in.TeacherID != 0 && sl.TeacherID != in.TeacherID {
			continue
		}
		out = append(out, slotView(sl))
	}
	return out, nil
}

// -- POST /api/schedule/copy --

// CopyInput adalah parameter POST /api/schedule/copy.
type CopyInput struct {
	FromClassID int64
	ToClassID   int64
}

// CopySchedule menyalin seluruh slot kelas sumber ke kelas tujuan
// (subject/day/period ikut, teacher ikut, room DIKOSONGKAN — docs/04). Tolak
// bila kelas tujuan sudah punya slot. Slot yang bentrok (mis. guru sudah
// mengajar kelas lain di jam yang sama) di-SKIP dan dilaporkan, bukan
// menggagalkan seluruh operasi.
func (s *Service) CopySchedule(ctx context.Context, actorUserID, schoolID int64, in CopyInput) (CopyResult, error) {
	if in.FromClassID == 0 || in.ToClassID == 0 {
		return CopyResult{}, httpx.Validation("from_class_id dan to_class_id wajib diisi.")
	}
	if in.FromClassID == in.ToClassID {
		return CopyResult{}, httpx.Validation("Rombel sumber dan tujuan tidak boleh sama.")
	}

	fromCls, err := s.repo.GetClassRef(ctx, schoolID, in.FromClassID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return CopyResult{}, httpx.Validation("Rombel sumber tidak ditemukan.")
		}
		return CopyResult{}, err
	}
	toCls, err := s.repo.GetClassRef(ctx, schoolID, in.ToClassID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return CopyResult{}, httpx.Validation("Rombel tujuan tidak ditemukan.")
		}
		return CopyResult{}, err
	}

	destExisting, err := s.repo.ListSlotsForYear(ctx, schoolID, toCls.AcademicYearID)
	if err != nil {
		return CopyResult{}, err
	}
	for _, sl := range destExisting {
		if sl.ClassID == in.ToClassID {
			return CopyResult{}, httpx.Validation("Rombel tujuan sudah punya jadwal — kosongkan dulu sebelum menyalin.")
		}
	}

	sourceAll, err := s.repo.ListSlotsForYear(ctx, schoolID, fromCls.AcademicYearID)
	if err != nil {
		return CopyResult{}, err
	}
	var source []SlotRecord
	for _, sl := range sourceAll {
		if sl.ClassID == in.FromClassID {
			source = append(source, sl)
		}
	}
	if len(source) == 0 {
		return CopyResult{Copied: 0, Skipped: []CopySkipItem{}}, nil
	}

	working := append([]SlotRecord{}, destExisting...)
	// syntheticID: lihat catatan yang sama di import.go markImportConflicts —
	// TIDAK PERNAH 0 supaya tidak "cocok" dengan excludeSlotID=0 di bawah.
	syntheticID := int64(-1)
	var toInsert []SlotInput
	var skipped []CopySkipItem
	for _, src := range source {
		conflicts := findConflicts(working, 0, src.DayOfWeek, src.PeriodStart, src.PeriodEnd, src.TeacherID, in.ToClassID, 0)
		if len(conflicts) > 0 {
			skipped = append(skipped, CopySkipItem{
				Subject: src.SubjectName,
				Reason:  conflictMessage(src.DayOfWeek, conflicts, src.TeacherID, in.ToClassID, 0),
			})
			continue
		}
		toInsert = append(toInsert, SlotInput{
			SchoolID: schoolID, AcademicYearID: toCls.AcademicYearID, ClassID: in.ToClassID,
			SubjectID: src.SubjectID, TeacherID: src.TeacherID, RoomID: 0,
			DayOfWeek: src.DayOfWeek, PeriodStart: src.PeriodStart, PeriodEnd: src.PeriodEnd,
		})
		working = append(working, SlotRecord{
			ID: syntheticID, ClassID: in.ToClassID, ClassName: toCls.Name,
			SubjectID: src.SubjectID, SubjectName: src.SubjectName,
			TeacherID: src.TeacherID, TeacherName: src.TeacherName,
			DayOfWeek: src.DayOfWeek, PeriodStart: src.PeriodStart, PeriodEnd: src.PeriodEnd,
		})
		syntheticID--
	}

	created, err := s.repo.CreateSlotsBatch(ctx, toInsert)
	if err != nil {
		return CopyResult{}, err
	}
	if skipped == nil {
		skipped = []CopySkipItem{}
	}
	s.audit(ctx, schoolID, actorUserID, "schedule.copy", "class", in.ToClassID, nil,
		map[string]any{"from_class_id": in.FromClassID, "copied": len(created), "skipped": len(skipped)})
	s.emitSchedule(schoolID)
	return CopyResult{Copied: len(created), Skipped: skipped}, nil
}

// -- query kunci (docs/04: dipakai fase 6/7, expose juga sebagai endpoint) --

// sortPeriodsByStart mengurutkan berdasarkan waktu mulai (bukan cuma
// `number`, walau seharusnya selalu selaras — ReplacePeriods menegakkan itu).
func sortPeriodsByStart(periods []PeriodRecord) {
	sort.Slice(periods, func(i, j int) bool {
		si, _ := clockMinutes(periods[i].StartsAt)
		sj, _ := clockMinutes(periods[j].StartsAt)
		return si < sj
	})
}

// currentPeriod mencari period yang mencakup waktu lokal sekolah `at` —
// dikembalikan juga daftar period terurut (dipakai CurrentPeriod menghitung
// next_starts_at tanpa query ulang).
func (s *Service) currentPeriod(ctx context.Context, schoolID int64, at time.Time) (*PeriodRecord, []PeriodRecord, error) {
	periods, err := s.repo.ListPeriods(ctx, schoolID)
	if err != nil {
		return nil, nil, err
	}
	sortPeriodsByStart(periods)
	local := clock.InZone(at, schoolTimezone(ctx))
	nowMin := local.Hour()*60 + local.Minute()
	for i := range periods {
		startMin, _ := clockMinutes(periods[i].StartsAt)
		endMin, _ := clockMinutes(periods[i].EndsAt)
		if nowMin >= startMin && nowMin <= endMin {
			return &periods[i], periods, nil
		}
	}
	return nil, periods, nil
}

// CurrentPeriod — GET /api/schedule/current-period: jam ke berapa sekarang
// (waktu lokal sekolah), atau jam berikutnya yang akan mulai (docs/04).
func (s *Service) CurrentPeriod(ctx context.Context, schoolID int64, at time.Time) (CurrentPeriodView, error) {
	cur, periods, err := s.currentPeriod(ctx, schoolID, at)
	if err != nil {
		return CurrentPeriodView{}, err
	}
	if cur != nil {
		return CurrentPeriodView{Period: &CurrentPeriodInfo{Number: cur.Number, StartsAt: cur.StartsAt, EndsAt: cur.EndsAt}}, nil
	}
	local := clock.InZone(at, schoolTimezone(ctx))
	nowMin := local.Hour()*60 + local.Minute()
	for _, p := range periods {
		startMin, _ := clockMinutes(p.StartsAt)
		if startMin > nowMin {
			v := p.StartsAt
			return CurrentPeriodView{NextStartsAt: &v}, nil
		}
	}
	return CurrentPeriodView{}, nil
}

// TodayQuery adalah parameter GET /api/schedule/today.
type TodayQuery struct {
	ClassID   int64
	TeacherID int64
}

// SlotsToday — slot hari ini (waktu lokal sekolah) urut jam, dengan penanda
// is_now (docs/04: dipakai jurnal mengajar & TV fase 6/7). Object-level sama
// dengan ListSlots.
func (s *Service) SlotsToday(ctx context.Context, schoolID int64, in TodayQuery, at time.Time) ([]TodaySlotView, error) {
	role := reqctx.Role(ctx)
	if role == RoleSiswa {
		myClassID, ok, err := s.students.MyClassID(ctx, schoolID, reqctx.UserID(ctx))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.ErrForbidden
		}
		if in.ClassID != 0 && in.ClassID != myClassID {
			return nil, httpx.ErrForbidden
		}
		in.ClassID = myClassID
		in.TeacherID = 0
	}
	if role == RoleGuru && in.ClassID == 0 && in.TeacherID == 0 {
		// Guru tanpa parameter = jadwal miliknya sendiri.
		tid, ok, err := s.students.MyTeacherID(ctx, schoolID, reqctx.UserID(ctx))
		if err != nil {
			return nil, err
		}
		if ok {
			in.TeacherID = tid
		}
	}
	if in.ClassID == 0 && in.TeacherID == 0 {
		return nil, httpx.Validation("class_id atau teacher_id wajib diisi.")
	}

	yearID, err := s.resolveYear(ctx, schoolID, 0)
	if err != nil {
		return nil, err
	}
	all, err := s.repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return nil, err
	}

	local := clock.InZone(at, schoolTimezone(ctx))
	dow := int(local.Weekday())
	cur, _, err := s.currentPeriod(ctx, schoolID, at)
	if err != nil {
		return nil, err
	}

	out := make([]TodaySlotView, 0)
	for _, sl := range all {
		if sl.DayOfWeek != dow {
			continue
		}
		if in.ClassID != 0 && sl.ClassID != in.ClassID {
			continue
		}
		if in.TeacherID != 0 && sl.TeacherID != in.TeacherID {
			continue
		}
		isNow := cur != nil && sl.PeriodStart <= cur.Number && cur.Number <= sl.PeriodEnd
		out = append(out, TodaySlotView{SlotView: slotView(sl), IsNow: isNow})
	}
	return out, nil
}

// SlotsForDayOfWeek — SEMUA slot jadwal (tahun ajaran aktif) pada
// day_of_week tertentu, lintas kelas & guru, TANPA filter object-level
// (dipakai modul teaching fase 6 lewat consumer-side interface untuk
// monitoring lintas guru — otorisasi teaching:monitor sudah ditegakkan
// pemanggil, lihat docs/06-teaching.md "Status mengajar"). Berbeda dari
// SlotsToday: tidak butuh class_id/teacher_id, dan tidak terikat "hari ini"
// (dipakai juga untuk tanggal lampau saat GET /api/teaching/status?date=).
func (s *Service) SlotsForDayOfWeek(ctx context.Context, schoolID int64, dayOfWeek int) ([]SlotView, error) {
	yearID, err := s.resolveYear(ctx, schoolID, 0)
	if err != nil {
		if errors.Is(err, ErrNoActiveAcademicYear) {
			return []SlotView{}, nil
		}
		return nil, err
	}
	all, err := s.repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return nil, err
	}
	out := make([]SlotView, 0)
	for _, sl := range all {
		if sl.DayOfWeek == dayOfWeek {
			out = append(out, slotView(sl))
		}
	}
	return out, nil
}

// SlotByID mengembalikan satu slot apa adanya (tanpa filter tanggal/guru) —
// dipakai modul teaching (fase 6, lewat consumer-side interface) menderivasi
// status "done" jurnal individual dari period_end slot itu (docs/06-teaching.md).
// ok=false bila slot tidak ditemukan di sekolah ini.
func (s *Service) SlotByID(ctx context.Context, schoolID, slotID int64) (SlotView, bool, error) {
	rec, err := s.repo.GetSlotByID(ctx, schoolID, slotID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SlotView{}, false, nil
		}
		return SlotView{}, false, err
	}
	return slotView(rec), true, nil
}

// SlotOwnership mengembalikan (class_id, teacher_id) satu slot — primitif
// murni supaya modul lain (attendance fase 6, lewat consumer-side interface)
// bisa memvalidasi kepemilikan slot ("guru hanya boleh buka sesi absen utk
// slotnya sendiri") TANPA mengimpor tipe schedule. ok=false bila slot tidak
// ditemukan di sekolah ini.
func (s *Service) SlotOwnership(ctx context.Context, schoolID, slotID int64) (classID, teacherID int64, ok bool, err error) {
	rec, err := s.repo.GetSlotByID(ctx, schoolID, slotID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, 0, false, nil
		}
		return 0, 0, false, err
	}
	return rec.ClassID, rec.TeacherID, true, nil
}

// SlotNow — slot guru (teacherID) yang sedang berlangsung pada waktu `at`
// (docs/04: dipakai scan QR kelas fase 6). Mengembalikan (nil, nil) bila guru
// tidak sedang mengajar (di luar jam pelajaran, atau tidak ada slot cocok).
func (s *Service) SlotNow(ctx context.Context, schoolID, teacherID int64, at time.Time) (*SlotView, error) {
	cur, _, err := s.currentPeriod(ctx, schoolID, at)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return nil, nil
	}
	yearID, err := s.resolveYear(ctx, schoolID, 0)
	if err != nil {
		if errors.Is(err, ErrNoActiveAcademicYear) {
			return nil, nil
		}
		return nil, err
	}
	all, err := s.repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return nil, err
	}
	local := clock.InZone(at, schoolTimezone(ctx))
	dow := int(local.Weekday())
	for _, sl := range all {
		if sl.TeacherID == teacherID && sl.DayOfWeek == dow && sl.PeriodStart <= cur.Number && cur.Number <= sl.PeriodEnd {
			v := slotView(sl)
			return &v, nil
		}
	}
	return nil, nil
}

// -- Fase 14 Gelombang B2 (docs/12-sion-parity.md): dipakai internal/exitpermit
// & internal/latearrival lewat consumer-side interface (dijembatani adapter
// cmd/server, lihat b2adapter.go — SlotView.Teacher.ID adalah profil guru,
// bukan user_id; pemetaan ke user_id dilakukan adapter via
// student.Service.MyTeacherID yang SUDAH ADA, tanpa method baru di modul
// student).

// ClassSlotNowOrNext mengembalikan slot KELAS classID yang SEDANG berjalan
// (period berjalan, waktu lokal sekolah `at`) — atau BILA TIDAK ADA, slot
// PALING AWAL berikutnya hari itu (period_start > sekarang) — dipakai
// exitpermit memvalidasi "guru pengajar jam berjalan/berikutnya" (tahap 2
// rantai dispensasi keluar). nil bila TA tidak aktif, kelas tanpa jadwal
// hari ini, atau seluruh slot hari itu sudah lewat.
func (s *Service) ClassSlotNowOrNext(ctx context.Context, schoolID, classID int64, at time.Time) (*SlotView, error) {
	cur, periods, err := s.currentPeriod(ctx, schoolID, at)
	if err != nil {
		return nil, err
	}
	yearID, err := s.resolveYear(ctx, schoolID, 0)
	if err != nil {
		if errors.Is(err, ErrNoActiveAcademicYear) {
			return nil, nil
		}
		return nil, err
	}
	all, err := s.repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return nil, err
	}
	local := clock.InZone(at, schoolTimezone(ctx))
	dow := int(local.Weekday())
	todays := make([]SlotRecord, 0)
	for _, sl := range all {
		if sl.DayOfWeek == dow && sl.ClassID == classID {
			todays = append(todays, sl)
		}
	}
	if cur != nil {
		for _, sl := range todays {
			if sl.PeriodStart <= cur.Number && cur.Number <= sl.PeriodEnd {
				v := slotView(sl)
				return &v, nil
			}
		}
	}
	periodStartMin := make(map[int]int, len(periods))
	for _, p := range periods {
		if m, err := clockMinutes(p.StartsAt); err == nil {
			periodStartMin[p.Number] = m
		}
	}
	nowMin := local.Hour()*60 + local.Minute()
	bestIdx := -1
	bestMin := 0
	for i, sl := range todays {
		startMin, ok := periodStartMin[sl.PeriodStart]
		if !ok || startMin <= nowMin {
			continue
		}
		if bestIdx == -1 || startMin < bestMin {
			bestIdx, bestMin = i, startMin
		}
	}
	if bestIdx == -1 {
		return nil, nil
	}
	v := slotView(todays[bestIdx])
	return &v, nil
}

// TeachesClassToday melaporkan apakah guru profil teacherID mengajar kelas
// classID pada SETIDAKNYA SATU slot hari ini (waktu lokal sekolah `at`),
// TANPA syarat jam berjalan/berikutnya — dipakai internal/latearrival tahap
// akhir ("guru kelas", cukup ada slot hari itu, beda dari
// ClassSlotNowOrNext dipakai exitpermit).
func (s *Service) TeachesClassToday(ctx context.Context, schoolID, classID, teacherID int64, at time.Time) (bool, error) {
	yearID, err := s.resolveYear(ctx, schoolID, 0)
	if err != nil {
		if errors.Is(err, ErrNoActiveAcademicYear) {
			return false, nil
		}
		return false, err
	}
	all, err := s.repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return false, err
	}
	local := clock.InZone(at, schoolTimezone(ctx))
	dow := int(local.Weekday())
	for _, sl := range all {
		if sl.DayOfWeek == dow && sl.ClassID == classID && sl.TeacherID == teacherID {
			return true, nil
		}
	}
	return false, nil
}

// LastPeriodEndToday mengembalikan waktu UTC akhir period TERAKHIR (max
// EndsAt di antara SELURUH period sekolah) pada tanggal lokal sekolah `at` —
// dipakai exitpermit menghitung gate_expires_at otomatis saat izin issued
// (docs tugas: "kedaluwarsa otomatis di akhir jam izin"). ok=false bila
// sekolah belum punya period sama sekali (pemanggil fallback +6 jam).
func (s *Service) LastPeriodEndToday(ctx context.Context, schoolID int64, at time.Time) (time.Time, bool, error) {
	periods, err := s.repo.ListPeriods(ctx, schoolID)
	if err != nil {
		return time.Time{}, false, err
	}
	if len(periods) == 0 {
		return time.Time{}, false, nil
	}
	maxEndMin := -1
	for _, p := range periods {
		m, err := clockMinutes(p.EndsAt)
		if err != nil {
			continue
		}
		if m > maxEndMin {
			maxEndMin = m
		}
	}
	if maxEndMin < 0 {
		return time.Time{}, false, nil
	}
	tz := schoolTimezone(ctx)
	loc, lerr := time.LoadLocation(tz)
	if lerr != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	local := clock.InZone(at, tz)
	endLocal := time.Date(local.Year(), local.Month(), local.Day(), maxEndMin/60, maxEndMin%60, 0, 0, loc)
	return endLocal.UTC(), true, nil
}
