package substitution

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interface (lihat CLAUDE.md). Semua dipenuhi *identity.Service
// secara STRUKTURAL — substitution TIDAK mengimpor package identity.
// Validasi kepemilikan slot/keanggotaan guru dilakukan lewat join read-only
// LANGSUNG ke schedule_slots/teachers/memberships di repository.go (pola yang
// sama dengan internal/discipline "join langsung ke tabel modul lain untuk
// data read-only" — bukan consumer-side interface baru).

// IdentityGateway adalah kebutuhan modul substitution dari modul identity:
// gerbang permission (scope=all butuh schedule:manage) & audit log.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// Notifier adalah kebutuhan modul substitution dari modul notification.
type Notifier interface {
	Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error
}

// Event kanonik — nilai HARUS sama persis dengan
// notification.EventSubstitutionRequested/Decided (didefinisikan ulang di
// sini karena substitution TIDAK boleh mengimpor notification, lihat CLAUDE.md).
const (
	EventSubstitutionRequested = "substitution.requested"
	EventSubstitutionDecided   = "substitution.decided"
)

// Realtime adalah kebutuhan modul substitution dari modul realtime —
// consumer-side interface kecil. Publish event "schedule" saat accept (docs
// tugas: "Realtime schedule publish saat accept" — event yang SAMA dipakai
// modul schedule sendiri, supaya klien yang sudah mendengarkan "schedule"
// otomatis refetch, tanpa event baru).
type Realtime interface {
	Publish(schoolID int64, eventType string, data map[string]any)
}

// Service berisi aturan bisnis modul substitution.
type Service struct {
	repo     substitutionRepository
	identity IdentityGateway
	clock    clock.Clock
	notifier Notifier
	realtime Realtime
}

func NewService(repo *Repository, identity IdentityGateway, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, clock: clk}
}

func (s *Service) SetNotifier(n Notifier) { s.notifier = n }
func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja.
func newServiceForTest(repo substitutionRepository, identity IdentityGateway, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, clock: clk}
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

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

func schoolToday(now time.Time, tz string) time.Time {
	local := clock.InZone(now, tz)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func rowView(r Row) RequestView {
	return RequestView{
		ID: r.ID,
		Slot: SlotRef{
			ID: r.ScheduleSlotID, ClassName: r.ClassName, SubjectName: r.SubjectName,
			DayOfWeek: r.DayOfWeek, PeriodStart: r.PeriodStart, PeriodEnd: r.PeriodEnd,
		},
		Date:        r.Date.Format("2006-01-02"),
		RequestedBy: UserRef{ID: r.RequestedBy, Name: r.RequestedByName},
		Substitute:  UserRef{ID: r.SubstituteUserID, Name: r.SubstituteName},
		Reason:      r.Reason, Status: r.Status, DecidedAt: r.DecidedAt, CreatedAt: r.CreatedAt,
	}
}

// -- POST /api/substitutions --

// RequestInput adalah parameter Service.Request.
type RequestInput struct {
	ScheduleSlotID   int64
	Date             string
	SubstituteUserID int64
	Reason           string
}

// Request — POST /api/substitutions: guru pemilik slot mengajukan pengganti
// utk tanggal >= hari ini (docs tugas Fase 14 Gelombang D). admin_sekolah
// boleh mengajukan atas nama slot mana pun (kelonggaran administratif —
// TIDAK dilarang tugas, konsisten dengan pola modul lain "admin bebas").
func (s *Service) Request(ctx context.Context, actorUserID, schoolID int64, in RequestInput) (RequestView, error) {
	if in.ScheduleSlotID == 0 {
		return RequestView{}, httpx.Validation("schedule_slot_id wajib diisi.")
	}
	date, err := time.Parse("2006-01-02", strings.TrimSpace(in.Date))
	if err != nil {
		return RequestView{}, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	today := schoolToday(s.clock.Now(), schoolTimezone(ctx))
	if date.Before(today) {
		return RequestView{}, httpx.Validation("Tanggal pengganti tidak boleh sebelum hari ini.")
	}
	if in.SubstituteUserID == 0 {
		return RequestView{}, httpx.Validation("substitute_user_id wajib diisi.")
	}
	if in.SubstituteUserID == actorUserID {
		return RequestView{}, httpx.Validation("Tidak bisa mengajukan diri sendiri sebagai pengganti.")
	}

	slot, err := s.repo.GetSlotBasic(ctx, schoolID, in.ScheduleSlotID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return RequestView{}, httpx.Validation("Slot jadwal tidak ditemukan.")
		}
		return RequestView{}, err
	}
	if int(date.Weekday()) != slot.DayOfWeek {
		return RequestView{}, httpx.Validation("Tanggal yang dipilih tidak jatuh pada hari slot ini dijadwalkan.")
	}

	if reqctx.Role(ctx) != RoleAdminSekolah {
		ownerUserID, oerr := s.repo.GetTeacherUserID(ctx, schoolID, slot.TeacherID)
		if oerr != nil {
			return RequestView{}, oerr
		}
		if ownerUserID != actorUserID {
			return RequestView{}, httpx.ErrForbidden
		}
	}

	activeGuru, err := s.repo.IsActiveGuru(ctx, schoolID, in.SubstituteUserID)
	if err != nil {
		return RequestView{}, err
	}
	if !activeGuru {
		return RequestView{}, httpx.Validation("Pengganti harus guru aktif di sekolah ini.")
	}

	rec, err := s.repo.Create(ctx, CreateInput{
		SchoolID: schoolID, ScheduleSlotID: in.ScheduleSlotID, Date: date,
		RequestedBy: actorUserID, SubstituteUserID: in.SubstituteUserID, Reason: strings.TrimSpace(in.Reason),
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return RequestView{}, httpx.Conflict("Sudah ada permintaan pengganti aktif untuk slot & tanggal ini.")
		}
		return RequestView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "substitution.request", "teacher_substitution_request", rec.ID, nil,
		map[string]any{"schedule_slot_id": in.ScheduleSlotID, "date": in.Date, "substitute_user_id": in.SubstituteUserID})

	row, err := s.repo.GetRow(ctx, schoolID, rec.ID)
	if err != nil {
		return RequestView{}, err
	}
	s.notify(ctx, schoolID, EventSubstitutionRequested, []int64{in.SubstituteUserID}, map[string]any{
		"teacher": row.RequestedByName, "class": row.ClassName, "subject": row.SubjectName, "date": row.Date.Format("2006-01-02"),
	})
	return rowView(row), nil
}

// -- GET /api/substitutions?scope=mine|for-me|all&date= --

// ListQuery adalah parameter Service.List.
type ListQuery struct {
	Scope string
	Date  string
}

func (s *Service) List(ctx context.Context, actorUserID, schoolID int64, in ListQuery) ([]RequestView, error) {
	var requestedBy, substituteUserID int64
	switch in.Scope {
	case "", "mine":
		requestedBy = actorUserID
	case "for-me":
		substituteUserID = actorUserID
	case "all":
		if !s.identity.HasPermission(reqctx.Role(ctx), PermScheduleManage) {
			return nil, httpx.ErrForbidden
		}
	default:
		return nil, httpx.Validation("scope harus mine, for-me, atau all.")
	}

	rows, err := s.repo.ListRows(ctx, schoolID, requestedBy, substituteUserID, strings.TrimSpace(in.Date))
	if err != nil {
		return nil, err
	}
	out := make([]RequestView, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowView(r))
	}
	return out, nil
}

// -- POST /api/substitutions/{id}/accept|reject|cancel --

func (s *Service) decide(ctx context.Context, actorUserID, schoolID, id int64, to string, requireRole func(Row) error, notifyEvent string) (RequestView, error) {
	row, err := s.repo.GetRow(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return RequestView{}, httpx.ErrNotFound
		}
		return RequestView{}, err
	}
	if err := requireRole(row); err != nil {
		return RequestView{}, err
	}
	if row.Status != StatusPending {
		return RequestView{}, httpx.Validation("Permintaan ini sudah tidak pending (status saat ini: " + row.Status + ").")
	}

	if _, err := s.repo.Transition(ctx, schoolID, id, StatusPending, to, s.clock.Now()); err != nil {
		if errors.Is(err, ErrStateChanged) {
			return RequestView{}, httpx.Conflict("Status permintaan sudah berubah, muat ulang halaman.")
		}
		return RequestView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "substitution."+to, "teacher_substitution_request", id,
		map[string]any{"status": StatusPending}, map[string]any{"status": to})

	updated, err := s.repo.GetRow(ctx, schoolID, id)
	if err != nil {
		return RequestView{}, err
	}
	if notifyEvent != "" {
		decision := to
		s.notify(ctx, schoolID, notifyEvent, []int64{updated.RequestedBy}, map[string]any{
			"substitute": updated.SubstituteName, "class": updated.ClassName, "subject": updated.SubjectName,
			"date": updated.Date.Format("2006-01-02"), "decision": decisionLabel(decision),
		})
	}
	return rowView(updated), nil
}

func decisionLabel(status string) string {
	switch status {
	case StatusAccepted:
		return "diterima"
	case StatusRejected:
		return "ditolak"
	case StatusCanceled:
		return "dibatalkan"
	default:
		return status
	}
}

// Accept — POST /api/substitutions/{id}/accept: HANYA pengganti yang diminta.
func (s *Service) Accept(ctx context.Context, actorUserID, schoolID, id int64) (RequestView, error) {
	view, err := s.decide(ctx, actorUserID, schoolID, id, StatusAccepted, func(row Row) error {
		if row.SubstituteUserID != actorUserID {
			return httpx.ErrForbidden
		}
		return nil
	}, EventSubstitutionDecided)
	if err != nil {
		return RequestView{}, err
	}
	if s.realtime != nil {
		// Event "schedule" yang SAMA dipakai modul schedule sendiri (docs
		// tugas: "Realtime schedule publish saat accept") — klien yang sudah
		// mendengarkan slot/jadwal otomatis refetch, tanpa event baru.
		s.realtime.Publish(schoolID, "schedule", map[string]any{})
	}
	return view, nil
}

// Reject — POST /api/substitutions/{id}/reject: HANYA pengganti yang diminta.
func (s *Service) Reject(ctx context.Context, actorUserID, schoolID, id int64) (RequestView, error) {
	return s.decide(ctx, actorUserID, schoolID, id, StatusRejected, func(row Row) error {
		if row.SubstituteUserID != actorUserID {
			return httpx.ErrForbidden
		}
		return nil
	}, EventSubstitutionDecided)
}

// Cancel — POST /api/substitutions/{id}/cancel: HANYA pengaju, selama masih
// pending (docs tugas).
func (s *Service) Cancel(ctx context.Context, actorUserID, schoolID, id int64) (RequestView, error) {
	return s.decide(ctx, actorUserID, schoolID, id, StatusCanceled, func(row Row) error {
		if row.RequestedBy != actorUserID && reqctx.Role(ctx) != RoleAdminSekolah {
			return httpx.ErrForbidden
		}
		return nil
	}, "")
}

// -- interface publik (dipakai modul teaching lewat consumer-side interface
// SubstitutionLookup — docs tugas Fase 14 Gelombang D) --

// SubstituteName mengembalikan nama pengganti ACCEPTED utk (slotID,date),
// atau ok=false bila tidak ada (dipakai monitoring/TV: "teacher_name tampil
// {pengganti} (pengganti)").
func (s *Service) SubstituteName(ctx context.Context, schoolID, slotID int64, date string) (string, bool, error) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", false, nil
	}
	_, name, ok, err := s.repo.ActiveSubstituteForSlotDate(ctx, schoolID, slotID, d)
	if err != nil || !ok {
		return "", false, err
	}
	return name, true, nil
}

// IsSubstituteToday mengembalikan daftar schedule_slot_id yang teacherUserID
// adalah pengganti ACCEPTED pada tanggal date (dipakai teaching.Scan:
// scanner BUKAN guru slot TAPI pengganti accepted -> diizinkan buka jurnal).
func (s *Service) IsSubstituteToday(ctx context.Context, schoolID, teacherUserID int64, date string) ([]int64, error) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, nil
	}
	return s.repo.SlotIDsAcceptedSubstituteForDate(ctx, schoolID, teacherUserID, d)
}
