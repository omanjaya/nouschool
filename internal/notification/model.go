// Package notification mengurus notifikasi pluggable: inbox in-app (baseline,
// selalu ditulis), web push, WhatsApp, dan email — lewat pola outbox + channel
// provider (lihat docs/08-notification.md). Modul lain TIDAK memanggil
// WhatsApp/email langsung; mereka memanggil Service.Notify (consumer-side
// interface Notifier dideklarasikan di modul pemakai — lihat CLAUDE.md).
package notification

import (
	"context"
	"time"
)

// Channel kanonik (docs/08-notification.md). ChannelInApp adalah baseline:
// SELALU ditulis ke tabel notifications, TIDAK PERNAH lewat outbox.
const (
	ChannelInApp    = "in_app"
	ChannelWebPush  = "web_push"
	ChannelWhatsApp = "whatsapp"
	ChannelEmail    = "email"
)

func validChannel(c string) bool {
	switch c {
	case ChannelInApp, ChannelWebPush, ChannelWhatsApp, ChannelEmail:
		return true
	default:
		return false
	}
}

// Event kanonik yang terdaftar di registry (lihat templates.go). Modul lain
// mendefinisikan ULANG nilai string ini secara lokal (mis.
// attendance.EventStudentAbsent) karena mereka TIDAK boleh mengimpor package
// ini — hanya memanggil lewat consumer-side interface Notifier.
const (
	EventAttendanceStudentAbsent = "attendance.student_absent"
	EventLeaveSubmitted          = "leave.submitted"
	EventLeaveDecided            = "leave.decided"
	EventDisciplineRecorded      = "discipline.recorded"
	EventDisciplineSPIssued      = "discipline.sp_issued"
	EventStudentLeaveSubmitted   = "studentleave.submitted"
	EventStudentLeaveForwarded   = "studentleave.forwarded"
	EventStudentLeaveDecided     = "studentleave.decided"
	// Fase 14 Gelombang B2 (docs/12-sion-parity.md).
	EventExitPermitIssued    = "exitpermit.issued"
	EventExitPermitExited    = "exitpermit.exited"
	EventLateArrivalRecorded = "latearrival.recorded"
)

// Status kanonik notification_outbox.status (docs/08-notification.md).
const (
	OutboxPending = "pending"
	OutboxSent    = "sent"
	OutboxFailed  = "failed"
	OutboxDead    = "dead"
)

// Notification adalah input Service.Send — satu event, banyak penerima
// (kontak per channel diresolusi modul ini sendiri, lihat docs/08).
type Notification struct {
	SchoolID int64
	Event    string
	UserIDs  []int64
	Data     map[string]any
}

// RenderedMessage adalah pesan yang SUDAH dirender (title/body final saat
// Send dipanggil — worker tidak pernah merender ulang, lihat catatan desain
// di service.go) — dikirim ke Provider.Send.
type RenderedMessage struct {
	SchoolID int64
	UserID   int64
	Event    string
	Title    string
	Body     string
	Link     string
	Data     map[string]any
}

// Provider adalah satu-satunya kontrak channel pengirim (docs/08-notification.md
// "Channel provider"). Send harus idempotent-safe (worker bisa memanggil
// ulang setelah retry).
type Provider interface {
	Send(ctx context.Context, msg RenderedMessage) error
}

// Configurable adalah kapabilitas OPSIONAL sebuah Provider: melaporkan
// apakah konfigurasi platform-nya terisi (mis. SMTP_HOST, WA_GATEWAY_URL).
// Service.channelConfigured melakukan type assertion ke interface ini —
// Provider yang tidak mengimplementasikannya dianggap selalu configured
// (mis. web_push: VAPID selalu di-generate saat startup, lihat webpush.go).
type Configurable interface {
	Configured() bool
}

// ErrNoContact menandai penerima TIDAK punya kontak untuk channel ini (mis.
// whatsapp tanpa nomor telepon) — worker menandai baris outbox 'dead'
// LANGSUNG (attempts tetap 0, TANPA backoff) karena mengulang tidak akan
// mengubah hasil (lihat keputusan desain di worker.go).
var ErrNoContact = &contactError{}

type contactError struct{}

func (*contactError) Error() string {
	return "notification: penerima tidak punya kontak untuk channel ini"
}

// -- shape response API --

// NotificationItem adalah satu baris response GET /api/notifications.
type NotificationItem struct {
	ID        int64      `json:"id"`
	Event     string     `json:"event"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Link      string     `json:"link,omitempty"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// ListResult adalah shape response GET /api/notifications.
type ListResult struct {
	Items       []NotificationItem `json:"items"`
	UnreadCount int64              `json:"unread_count"`
}
