package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/omanjaya/nouschool/internal/platform/clock"
)

// -- fakeRepo: implementasi notificationRepository in-memory (tanpa DB) —
// dipakai test di package ini saja, pola yang sama dengan
// internal/attendance/service_test.go fakeRepo. --

type fakeOutboxRow struct {
	OutboxRow
	nextRetryAt time.Time
	sentAt      time.Time
}

type fakeRepo struct {
	settings      map[int64]Settings
	notifications []NotificationRow
	notifSchool   map[int64]int64 // notif id -> school
	notifUser     map[int64]int64 // notif id -> user
	nextNotifID   int64

	outbox       map[int64]*fakeOutboxRow
	nextOutboxID int64

	contacts map[int64][2]string // userID -> [phone, email]

	subs map[string]struct {
		schoolID, userID       int64
		p256dh, auth, endpoint string
	}

	platformConfig map[string]string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		settings:       map[int64]Settings{},
		notifSchool:    map[int64]int64{},
		notifUser:      map[int64]int64{},
		outbox:         map[int64]*fakeOutboxRow{},
		contacts:       map[int64][2]string{},
		platformConfig: map[string]string{},
		subs: map[string]struct {
			schoolID, userID       int64
			p256dh, auth, endpoint string
		}{},
	}
}

func (f *fakeRepo) GetSettings(ctx context.Context, schoolID int64) (Settings, error) {
	if s, ok := f.settings[schoolID]; ok {
		return s, nil
	}
	return DefaultSettings(), nil
}

func (f *fakeRepo) InsertNotification(ctx context.Context, schoolID, userID int64, event, title, body, link string) (int64, error) {
	f.nextNotifID++
	id := f.nextNotifID
	f.notifications = append(f.notifications, NotificationRow{ID: id, Event: event, Title: title, Body: body, Link: link, CreatedAt: time.Now()})
	f.notifSchool[id] = schoolID
	f.notifUser[id] = userID
	return id, nil
}

func (f *fakeRepo) ListNotifications(ctx context.Context, schoolID, userID int64, limit, offset int32) ([]NotificationRow, error) {
	var out []NotificationRow
	for _, n := range f.notifications {
		if f.notifSchool[n.ID] == schoolID && f.notifUser[n.ID] == userID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeRepo) CountUnreadNotifications(ctx context.Context, schoolID, userID int64) (int64, error) {
	var n int64
	for _, row := range f.notifications {
		if f.notifSchool[row.ID] == schoolID && f.notifUser[row.ID] == userID && row.ReadAt == nil {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) MarkNotificationsReadByIDs(ctx context.Context, schoolID, userID int64, ids []int64) (int64, error) {
	set := map[int64]bool{}
	for _, id := range ids {
		set[id] = true
	}
	var n int64
	for i, row := range f.notifications {
		if set[row.ID] && f.notifSchool[row.ID] == schoolID && f.notifUser[row.ID] == userID && row.ReadAt == nil {
			t := time.Now()
			f.notifications[i].ReadAt = &t
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) MarkAllNotificationsRead(ctx context.Context, schoolID, userID int64) (int64, error) {
	var n int64
	for i, row := range f.notifications {
		if f.notifSchool[row.ID] == schoolID && f.notifUser[row.ID] == userID && row.ReadAt == nil {
			t := time.Now()
			f.notifications[i].ReadAt = &t
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) InsertOutboxRow(ctx context.Context, schoolID, userID int64, event, channel string, payload []byte) error {
	f.nextOutboxID++
	f.outbox[f.nextOutboxID] = &fakeOutboxRow{OutboxRow: OutboxRow{
		ID: f.nextOutboxID, SchoolID: schoolID, Event: event, UserID: userID, Channel: channel, Payload: payload, Status: OutboxPending,
	}}
	return nil
}

func (f *fakeRepo) ListDueOutbox(ctx context.Context, limit int32) ([]OutboxRow, error) {
	var out []OutboxRow
	for _, row := range f.outbox {
		if row.Status != OutboxPending && row.Status != OutboxFailed {
			continue
		}
		if !row.nextRetryAt.IsZero() && row.nextRetryAt.After(time.Now()) {
			continue
		}
		out = append(out, row.OutboxRow)
		if int32(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeRepo) MarkOutboxSent(ctx context.Context, id int64, sentAt time.Time) error {
	row := f.outbox[id]
	row.Status = OutboxSent
	row.sentAt = sentAt
	return nil
}

func (f *fakeRepo) MarkOutboxRetry(ctx context.Context, id int64, attempts int32, nextRetryAt time.Time) error {
	row := f.outbox[id]
	row.Status = OutboxFailed
	row.Attempts = attempts
	row.nextRetryAt = nextRetryAt
	return nil
}

func (f *fakeRepo) MarkOutboxDead(ctx context.Context, id int64, attempts int32) error {
	row := f.outbox[id]
	row.Status = OutboxDead
	row.Attempts = attempts
	return nil
}

func (f *fakeRepo) CountOutboxByChannel(ctx context.Context, schoolID int64, event, channel string) (int64, error) {
	var n int64
	for _, row := range f.outbox {
		if row.SchoolID == schoolID && row.Event == event && row.Channel == channel {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) GetUserContact(ctx context.Context, userID int64) (string, string, error) {
	c := f.contacts[userID]
	return c[0], c[1], nil
}

func (f *fakeRepo) UpsertPushSubscription(ctx context.Context, schoolID, userID int64, endpoint, p256dh, auth string) error {
	f.subs[endpoint] = struct {
		schoolID, userID       int64
		p256dh, auth, endpoint string
	}{schoolID, userID, p256dh, auth, endpoint}
	return nil
}

func (f *fakeRepo) ListPushSubscriptionsForUser(ctx context.Context, schoolID, userID int64) ([]PushSubscriptionRow, error) {
	var out []PushSubscriptionRow
	for _, s := range f.subs {
		if s.schoolID == schoolID && s.userID == userID {
			out = append(out, PushSubscriptionRow{Endpoint: s.endpoint, P256dh: s.p256dh, Auth: s.auth})
		}
	}
	return out, nil
}

func (f *fakeRepo) DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	delete(f.subs, endpoint)
	return nil
}

func (f *fakeRepo) DeletePushSubscriptionByUserEndpoint(ctx context.Context, schoolID, userID int64, endpoint string) (int64, error) {
	s, ok := f.subs[endpoint]
	if !ok || s.schoolID != schoolID || s.userID != userID {
		return 0, nil
	}
	delete(f.subs, endpoint)
	return 1, nil
}

func (f *fakeRepo) GetPlatformConfig(ctx context.Context, key string) (string, bool, error) {
	v, ok := f.platformConfig[key]
	return v, ok, nil
}

func (f *fakeRepo) SetPlatformConfig(ctx context.Context, key, value string) error {
	f.platformConfig[key] = value
	return nil
}

// -- fakeProvider: Provider palsu dipakai test worker --

type fakeProvider struct {
	configured bool
	sendFunc   func(ctx context.Context, msg RenderedMessage) error
	calls      []RenderedMessage
}

func (p *fakeProvider) Send(ctx context.Context, msg RenderedMessage) error {
	p.calls = append(p.calls, msg)
	if p.sendFunc != nil {
		return p.sendFunc(ctx, msg)
	}
	return nil
}

func (p *fakeProvider) Configured() bool { return p.configured }

// -- renderTemplate (fungsi murni) --

func TestRenderTemplate(t *testing.T) {
	tmpl := registry[EventAttendanceStudentAbsent]
	title, body, err := renderTemplate(tmpl, map[string]any{"student": "Budi Santoso", "date": "2026-08-16", "status": "sakit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "Absensi Budi Santoso" {
		t.Fatalf("unexpected title: %q", title)
	}
	if body != "Budi Santoso tercatat sakit pada 2026-08-16." {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestRenderTemplate_LeaveEvents(t *testing.T) {
	subTmpl := registry[EventLeaveSubmitted]
	_, body, err := renderTemplate(subTmpl, map[string]any{"teacher": "Rendi", "type": "Izin", "date_start": "2026-08-17", "date_end": "2026-08-17"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "Rendi mengajukan Izin") {
		t.Fatalf("unexpected body: %q", body)
	}

	decTmpl := registry[EventLeaveDecided]
	_, body2, err := renderTemplate(decTmpl, map[string]any{"type": "Izin", "date_start": "2026-08-17", "date_end": "2026-08-17", "decision": "disetujui"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body2, "telah disetujui") {
		t.Fatalf("unexpected body: %q", body2)
	}
}

// -- backoff (fungsi murni) --

func TestNextBackoff(t *testing.T) {
	cases := []struct {
		attempts  int
		wantDelay time.Duration
		wantDead  bool
	}{
		{1, 1 * time.Minute, false},
		{2, 5 * time.Minute, false},
		{3, 30 * time.Minute, false},
		{4, 2 * time.Hour, false},
		{5, 0, true},
		{6, 0, true},
	}
	for _, c := range cases {
		delay, dead := nextBackoff(c.attempts)
		if dead != c.wantDead {
			t.Fatalf("attempts=%d: expected dead=%v, got %v", c.attempts, c.wantDead, dead)
		}
		if !dead && delay != c.wantDelay {
			t.Fatalf("attempts=%d: expected delay=%v, got %v", c.attempts, c.wantDelay, delay)
		}
	}
}

// -- Send: baseline in_app SELALU ditulis + resolusi channel (settings sekolah x config platform) --

func TestSendWritesInAppAlwaysAndOutboxPerConfiguredChannel(t *testing.T) {
	repo := newFakeRepo()
	// Sekolah mengaktifkan in_app + web_push + whatsapp (default hanya
	// in_app+web_push — whatsapp ditambahkan eksplisit di sini supaya test
	// ini murni menguji sisi "config platform kosong", bukan tertutup oleh
	// gerbang settings sekolah).
	repo.settings[1] = Settings{Channels: []string{ChannelInApp, ChannelWebPush, ChannelWhatsApp}}
	svc := newServiceForTest(repo, clock.Fixed{T: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)})

	wa := &fakeProvider{configured: true}
	svc.RegisterProvider(ChannelWhatsApp, wa)
	// web_push TIDAK didaftarkan sama sekali -> channelConfigured() harus false.

	err := svc.Notify(context.Background(), 1, EventAttendanceStudentAbsent, []int64{50}, map[string]any{
		"student": "Budi", "date": "2026-08-16", "status": "sakit",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// in_app selalu ditulis.
	items, err := repo.ListNotifications(context.Background(), 1, 50, 10, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("expected 1 notifikasi in-app, got %d (err=%v)", len(items), err)
	}
	if items[0].Title != "Absensi Budi" {
		t.Fatalf("unexpected title: %q", items[0].Title)
	}

	// whatsapp dikonfigurasi & default channel event ini -> outbox row dibuat.
	waCount, _ := repo.CountOutboxByChannel(context.Background(), 1, EventAttendanceStudentAbsent, ChannelWhatsApp)
	if waCount != 1 {
		t.Fatalf("expected 1 outbox row whatsapp, got %d", waCount)
	}
	// web_push default channel event ini TAPI provider TIDAK terdaftar (config kosong) -> TIDAK ada outbox row.
	pushCount, _ := repo.CountOutboxByChannel(context.Background(), 1, EventAttendanceStudentAbsent, ChannelWebPush)
	if pushCount != 0 {
		t.Fatalf("expected 0 outbox row web_push (config kosong), got %d", pushCount)
	}
}

func TestSendSkipsChannelNotEnabledInSchoolSettings(t *testing.T) {
	repo := newFakeRepo()
	repo.settings[1] = Settings{Channels: []string{ChannelInApp}} // sekolah HANYA mengaktifkan in_app
	svc := newServiceForTest(repo, clock.Fixed{T: time.Now()})
	svc.RegisterProvider(ChannelWhatsApp, &fakeProvider{configured: true})
	svc.RegisterProvider(ChannelWebPush, &fakeProvider{configured: true})

	if err := svc.Notify(context.Background(), 1, EventAttendanceStudentAbsent, []int64{50}, map[string]any{
		"student": "Budi", "date": "2026-08-16", "status": "sakit",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waCount, _ := repo.CountOutboxByChannel(context.Background(), 1, EventAttendanceStudentAbsent, ChannelWhatsApp)
	pushCount, _ := repo.CountOutboxByChannel(context.Background(), 1, EventAttendanceStudentAbsent, ChannelWebPush)
	if waCount != 0 || pushCount != 0 {
		t.Fatalf("expected 0 outbox row (settings sekolah hanya in_app), got wa=%d push=%d", waCount, pushCount)
	}
}

func TestSendUnknownEventErrors(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceForTest(repo, clock.Fixed{T: time.Now()})
	err := svc.Notify(context.Background(), 1, "event.tidak.dikenal", []int64{1}, nil)
	if err == nil {
		t.Fatal("expected error untuk event tidak dikenal")
	}
}

// -- worker: sent/failed/dead --

func TestProcessDueOutbox_Sent(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceForTest(repo, clock.Fixed{T: time.Now()})
	svc.RegisterProvider(ChannelWhatsApp, &fakeProvider{configured: true})

	if err := repo.InsertOutboxRow(context.Background(), 1, 50, EventAttendanceStudentAbsent, ChannelWhatsApp,
		mustPayload(t, "Judul", "Isi", "")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n, err := svc.ProcessDueOutbox(context.Background(), 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 baris diproses, got %d", n)
	}
	if repo.outbox[1].Status != OutboxSent {
		t.Fatalf("expected status sent, got %s", repo.outbox[1].Status)
	}
}

func TestProcessDueOutbox_FailedThenDead(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceForTest(repo, clock.Fixed{T: time.Now()})
	boom := errors.New("gateway down")
	svc.RegisterProvider(ChannelWhatsApp, &fakeProvider{configured: true, sendFunc: func(ctx context.Context, msg RenderedMessage) error { return boom }})

	if err := repo.InsertOutboxRow(context.Background(), 1, 50, EventAttendanceStudentAbsent, ChannelWhatsApp, mustPayload(t, "Judul", "Isi", "")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Percobaan #1..#4 -> failed dgn backoff, TETAP due (fake repo tidak
	// menghormati next_retry_at kecuali dites eksplisit, jadi kita panggil
	// ProcessDueOutbox berulang & reset next_retry_at manual antar panggilan
	// supaya baris tetap due).
	for i := 1; i <= 4; i++ {
		if _, err := svc.ProcessDueOutbox(context.Background(), 50); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		row := repo.outbox[1]
		if row.Status != OutboxFailed {
			t.Fatalf("percobaan #%d: expected status failed, got %s", i, row.Status)
		}
		if row.Attempts != int32(i) {
			t.Fatalf("percobaan #%d: expected attempts=%d, got %d", i, i, row.Attempts)
		}
		row.nextRetryAt = time.Time{} // paksa due lagi utk percobaan berikutnya di test ini
	}

	// Percobaan #5 -> dead.
	if _, err := svc.ProcessDueOutbox(context.Background(), 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	row := repo.outbox[1]
	if row.Status != OutboxDead {
		t.Fatalf("expected status dead setelah 5 percobaan, got %s (attempts=%d)", row.Status, row.Attempts)
	}
}

func TestProcessDueOutbox_NoContactMarksDeadImmediatelyAttemptsZero(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceForTest(repo, clock.Fixed{T: time.Now()})
	svc.RegisterProvider(ChannelWhatsApp, &fakeProvider{configured: true, sendFunc: func(ctx context.Context, msg RenderedMessage) error { return ErrNoContact }})

	if err := repo.InsertOutboxRow(context.Background(), 1, 50, EventAttendanceStudentAbsent, ChannelWhatsApp, mustPayload(t, "Judul", "Isi", "")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := svc.ProcessDueOutbox(context.Background(), 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	row := repo.outbox[1]
	if row.Status != OutboxDead {
		t.Fatalf("expected status dead LANGSUNG (tanpa kontak), got %s", row.Status)
	}
	if row.Attempts != 0 {
		t.Fatalf("expected attempts TETAP 0 (keputusan desain), got %d", row.Attempts)
	}
}

func mustPayload(t *testing.T, title, body, link string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"title": title, "body": body, "link": link, "data": map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return b
}

// -- WebPushProvider: subscription 404/410 dihapus --

type fakePushSender struct {
	status int
}

func (f *fakePushSender) Send(message []byte, sub *webpush.Subscription, options *webpush.Options) (*http.Response, error) {
	return &http.Response{StatusCode: f.status, Status: http.StatusText(f.status), Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestWebPushProvider_DeletesGoneSubscription(t *testing.T) {
	repo := newFakeRepo()
	if err := repo.UpsertPushSubscription(context.Background(), 1, 50, "https://push.example/ep1", "p256dh", "auth"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	provider := NewWebPushProvider(repo, "pub", "priv", "mailto:admin@nouschool.id")
	provider.sender = &fakePushSender{status: http.StatusGone} // 410

	err := provider.Send(context.Background(), RenderedMessage{SchoolID: 1, UserID: 50, Title: "T", Body: "B"})
	if err == nil {
		t.Fatal("expected error (tidak ada subscription yang berhasil dikirimi)")
	}

	subs, err := repo.ListPushSubscriptionsForUser(context.Background(), 1, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected subscription terhapus setelah 410, tersisa %d", len(subs))
	}
}

func TestWebPushProvider_NoSubscriptionReturnsError(t *testing.T) {
	repo := newFakeRepo()
	provider := NewWebPushProvider(repo, "pub", "priv", "mailto:admin@nouschool.id")
	err := provider.Send(context.Background(), RenderedMessage{SchoolID: 1, UserID: 999, Title: "T", Body: "B"})
	if err == nil {
		t.Fatal("expected error (belum ada subscription)")
	}
}

// -- WhatsAppProvider / EmailProvider: tanpa kontak -> ErrNoContact --

func TestWhatsAppProvider_NoPhoneReturnsErrNoContact(t *testing.T) {
	repo := newFakeRepo()
	provider := NewWhatsAppProvider(repo, "http://gateway.example", "token")
	err := provider.Send(context.Background(), RenderedMessage{SchoolID: 1, UserID: 50, Title: "T", Body: "B"})
	if !errors.Is(err, ErrNoContact) {
		t.Fatalf("expected ErrNoContact, got: %v", err)
	}
}

func TestEmailProvider_NoEmailReturnsErrNoContact(t *testing.T) {
	repo := newFakeRepo()
	provider := NewEmailProvider(repo, "smtp.example.com", "587", "", "", "no-reply@nouschool.id")
	err := provider.Send(context.Background(), RenderedMessage{SchoolID: 1, UserID: 50, Title: "T", Body: "B"})
	if !errors.Is(err, ErrNoContact) {
		t.Fatalf("expected ErrNoContact, got: %v", err)
	}
}

func TestWhatsAppEmailConfigured(t *testing.T) {
	wa := NewWhatsAppProvider(nil, "", "")
	if wa.Configured() {
		t.Fatal("expected NOT configured (WA_GATEWAY_URL kosong)")
	}
	wa2 := NewWhatsAppProvider(nil, "http://gateway.example", "")
	if !wa2.Configured() {
		t.Fatal("expected configured (WA_GATEWAY_URL terisi)")
	}

	email := NewEmailProvider(nil, "", "", "", "", "")
	if email.Configured() {
		t.Fatal("expected NOT configured (SMTP_HOST kosong)")
	}
	email2 := NewEmailProvider(nil, "smtp.example.com", "587", "", "", "")
	if !email2.Configured() {
		t.Fatal("expected configured (SMTP_HOST terisi)")
	}
}
