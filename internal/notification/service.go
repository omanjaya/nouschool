package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Service berisi aturan bisnis modul notification: menulis inbox in-app +
// outbox (Send/Notify), dan memproses outbox (ProcessDueOutbox, dipanggil
// worker.go). Lihat docs/08-notification.md untuk arsitektur lengkap.
// FeatureGate adalah kebutuhan modul notification dari modul billing (fase
// 10, docs/09-billing.md "Channel whatsapp: notification service cek fitur
// whatsapp sebelum membuat outbox row wa") — consumer-side interface kecil
// dideklarasikan di sisi PEMAKAI (lihat CLAUDE.md). Dipenuhi *billing.Service
// secara struktural lewat method HasFeature (signature primitif).
type FeatureGate interface {
	HasFeature(ctx context.Context, schoolID int64, key string) (bool, error)
}

// FeatureWhatsApp — NILAI HARUS SAMA PERSIS dengan billing.FeatureWhatsApp
// (didefinisikan ulang di sini karena notification TIDAK boleh mengimpor
// billing untuk konstanta apa pun — lihat CLAUDE.md).
const FeatureWhatsApp = "whatsapp"

// Realtime adalah kebutuhan modul notification dari modul realtime (Fase 12,
// bus event WebSocket read-only server->client) — consumer-side interface
// kecil dideklarasikan di sisi PEMAKAI (lihat CLAUDE.md). TIDAK dipenuhi
// *realtime.Hub secara langsung — dijembatani adapter kecil di cmd/server
// (realtimeadapter.go).
type Realtime interface {
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

type Service struct {
	repo      notificationRepository
	clock     clock.Clock
	providers map[string]Provider // key: channel (web_push|whatsapp|email) — TIDAK termasuk in_app
	features  FeatureGate         // opsional — nil = gerbang fitur dilewati (lihat SetFeatureGate)
	realtime  Realtime            // opsional — nil = event realtime dilewati (lihat SetRealtime)

	vapidPublicKey string

	loggedMu   sync.Mutex
	loggedOnce map[string]bool // channel -> sudah pernah log "config kosong"
}

func NewService(repo *Repository, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, clock: clk, providers: map[string]Provider{}, loggedOnce: map[string]bool{}}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo notificationRepository, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, clock: clk, providers: map[string]Provider{}, loggedOnce: map[string]bool{}}
}

// RegisterProvider mendaftarkan provider untuk satu channel non-in_app
// (dipanggil main.go saat wiring — lihat cmd/server/main.go).
func (s *Service) RegisterProvider(channel string, p Provider) {
	s.providers[channel] = p
}

// SetFeatureGate menyuntikkan FeatureGate SETELAH konstruksi (opsional,
// disuntik main.go — pola yang sama dengan internal/attendance.SetNotifier;
// nil aman/no-op, channel whatsapp lolos tanpa gerbang fitur).
func (s *Service) SetFeatureGate(g FeatureGate) { s.features = g }

// SetRealtime menyuntikkan Realtime SETELAH konstruksi (opsional, disuntik
// main.go — pola yang sama dengan SetFeatureGate; nil aman/no-op).
func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// whatsappAllowed melaporkan apakah channel whatsapp boleh dipakai sekolah
// ini (fitur "whatsapp" aktif di langganannya, docs/09-billing.md "Channel
// gating"). featureGate nil ATAU error dianggap boleh (fail-open) — modul
// notification TIDAK menegakkan billing, hanya menghormatinya bila tersedia;
// enforcement sesungguhnya ada di billing.SubscriptionGuard/RequireFeature.
func (s *Service) whatsappAllowed(ctx context.Context, schoolID int64) bool {
	if s.features == nil {
		return true
	}
	ok, err := s.features.HasFeature(ctx, schoolID, FeatureWhatsApp)
	if err != nil {
		return true
	}
	return ok
}

func (s *Service) channelConfigured(channel string) bool {
	p, ok := s.providers[channel]
	if !ok {
		return false
	}
	if c, ok := p.(Configurable); ok {
		return c.Configured()
	}
	return true
}

func (s *Service) logMissingConfigOnce(channel string) {
	s.loggedMu.Lock()
	defer s.loggedMu.Unlock()
	if s.loggedOnce[channel] {
		return
	}
	s.loggedOnce[channel] = true
	slog.Debug("notification: channel dilewati, konfigurasi platform kosong", "channel", channel)
}

// -- Send / Notify --

// Send menulis inbox in-app SELALU (baseline, docs/08) untuk setiap
// penerima, lalu satu baris notification_outbox per (penerima x channel
// default event yang aktif di settings sekolah DAN dikonfigurasi di
// platform). Title/body dirender SEKALI di sini dan disimpan di payload —
// worker TIDAK pernah merender ulang (lihat RenderedMessage & worker.go).
func (s *Service) Send(ctx context.Context, n Notification) error {
	tmpl, ok := registry[n.Event]
	if !ok {
		return fmt.Errorf("notification: event tidak dikenal: %s", n.Event)
	}
	title, body, err := renderTemplate(tmpl, n.Data)
	if err != nil {
		return err
	}

	settings, err := s.repo.GetSettings(ctx, n.SchoolID)
	if err != nil {
		return err
	}
	enabled := channelsSet(settings.Channels)

	payload, err := json.Marshal(map[string]any{"title": title, "body": body, "link": tmpl.Link, "data": n.Data})
	if err != nil {
		return err
	}

	for _, userID := range n.UserIDs {
		if userID == 0 {
			continue
		}
		if _, err := s.repo.InsertNotification(ctx, n.SchoolID, userID, n.Event, title, body, tmpl.Link); err != nil {
			return err
		}
		// Realtime (Fase 12): inbox in-app baru saja ditulis -> beri tahu
		// klien penerima supaya badge & list notifikasi refetch instan
		// (docs tugas: "setelah menulis inbox in-app -> notification {} ke
		// user_ids penerima"). Data SENGAJA kosong — klien selalu refetch
		// GET /api/notifications (bus read-only, lihat internal/realtime).
		if s.realtime != nil {
			s.realtime.PublishTo(n.SchoolID, "notification", map[string]any{}, nil, []int64{userID})
		}
		for _, ch := range tmpl.Channels {
			if !enabled[ch] {
				continue
			}
			if !s.channelConfigured(ch) {
				s.logMissingConfigOnce(ch)
				continue
			}
			if ch == ChannelWhatsApp && !s.whatsappAllowed(ctx, n.SchoolID) {
				continue
			}
			if err := s.repo.InsertOutboxRow(ctx, n.SchoolID, userID, n.Event, ch, payload); err != nil {
				return err
			}
		}
	}
	return nil
}

// Notify adalah wrapper primitif-friendly dari Send — signature ini SENGAJA
// dipakai supaya modul lain bisa mendeklarasikan consumer-side interface
// kecil (Notifier) tanpa mengimpor package ini, sesuai pola "antar-modul
// lewat interface kecil" di CLAUDE.md.
func (s *Service) Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error {
	return s.Send(ctx, Notification{SchoolID: schoolID, Event: event, UserIDs: userIDs, Data: data})
}

// -- GET /api/notifications --

func (s *Service) ListNotifications(ctx context.Context, schoolID, userID int64, page int) (ListResult, error) {
	if page < 1 {
		page = 1
	}
	const pageSize = 20
	rows, err := s.repo.ListNotifications(ctx, schoolID, userID, pageSize, int32((page-1)*pageSize))
	if err != nil {
		return ListResult{}, err
	}
	unread, err := s.repo.CountUnreadNotifications(ctx, schoolID, userID)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]NotificationItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, NotificationItem{
			ID: row.ID, Event: row.Event, Title: row.Title, Body: row.Body, Link: row.Link,
			ReadAt: row.ReadAt, CreatedAt: row.CreatedAt,
		})
	}
	return ListResult{Items: items, UnreadCount: unread}, nil
}

// -- POST /api/notifications/read --

func (s *Service) MarkRead(ctx context.Context, schoolID, userID int64, ids []int64, all bool) error {
	if all {
		_, err := s.repo.MarkAllNotificationsRead(ctx, schoolID, userID)
		return err
	}
	if len(ids) == 0 {
		return httpx.Validation("ids atau all wajib diisi.")
	}
	_, err := s.repo.MarkNotificationsReadByIDs(ctx, schoolID, userID, ids)
	return err
}

// -- push subscriptions --

func (s *Service) PublicKey() string {
	return s.vapidPublicKey
}

func (s *Service) Subscribe(ctx context.Context, schoolID, userID int64, endpoint, p256dh, auth string) error {
	if endpoint == "" || p256dh == "" || auth == "" {
		return httpx.Validation("endpoint, p256dh, dan auth wajib diisi.")
	}
	return s.repo.UpsertPushSubscription(ctx, schoolID, userID, endpoint, p256dh, auth)
}

func (s *Service) Unsubscribe(ctx context.Context, schoolID, userID int64, endpoint string) error {
	if endpoint == "" {
		return httpx.Validation("endpoint wajib diisi.")
	}
	_, err := s.repo.DeletePushSubscriptionByUserEndpoint(ctx, schoolID, userID, endpoint)
	return err
}

// SetVAPIDPublicKey diisi saat wiring (main.go, setelah generate/muat
// pasangan kunci — lihat webpush.go) supaya GET /api/push/public-key bisa
// menjawab tanpa Service perlu tahu detail VAPID.
func (s *Service) SetVAPIDPublicKey(key string) { s.vapidPublicKey = key }

// -- worker: lihat worker.go untuk loop-nya, backoff.go untuk jadwal retry --
