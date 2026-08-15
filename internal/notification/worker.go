package notification

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"
)

// outboxPayload adalah bentuk payload jsonb notification_outbox (ditulis
// Service.Send — lihat catatan "title/body dirender SEKALI" di service.go).
type outboxPayload struct {
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Link  string         `json:"link"`
	Data  map[string]any `json:"data"`
}

// ProcessDueOutbox mengambil satu batch baris outbox yang jatuh tempo,
// mengirim lewat Provider channel masing-masing, dan menandai hasilnya:
//   - sukses            -> sent
//   - ErrNoContact       -> dead LANGSUNG, attempts TIDAK di-increment (tetap
//     0) — mengulang tidak mengubah hasil (mis. whatsapp tanpa nomor telepon,
//     lihat catatan keputusan di internal/notification/whatsapp.go).
//   - error lain         -> attempts++, backoff (lihat backoff.go); setelah
//     jadwal backoff habis -> dead.
//
// Dipanggil StartWorker (loop) — juga dipakai langsung oleh test (batch
// tunggal, tanpa ticker).
func (s *Service) ProcessDueOutbox(ctx context.Context, batchSize int) (processed int, err error) {
	rows, err := s.repo.ListDueOutbox(ctx, int32(batchSize))
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		s.processOutboxRow(ctx, row)
		processed++
	}
	return processed, nil
}

func (s *Service) processOutboxRow(ctx context.Context, row OutboxRow) {
	provider, ok := s.providers[row.Channel]
	if !ok {
		// Channel tanpa provider terdaftar seharusnya tidak pernah masuk
		// outbox (Service.Send hanya menulis baris utk channel yang
		// dikonfigurasi) — jaga-jaga: tandai dead supaya tidak diulang tanpa
		// henti.
		slog.Warn("notification: baris outbox tanpa provider terdaftar", "id", row.ID, "channel", row.Channel)
		_ = s.repo.MarkOutboxDead(ctx, row.ID, row.Attempts)
		return
	}

	var p outboxPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		slog.Error("notification: payload outbox tidak valid", "id", row.ID, "err", err)
		_ = s.repo.MarkOutboxDead(ctx, row.ID, row.Attempts)
		return
	}

	msg := RenderedMessage{SchoolID: row.SchoolID, UserID: row.UserID, Event: row.Event, Title: p.Title, Body: p.Body, Link: p.Link, Data: p.Data}
	sendErr := provider.Send(ctx, msg)
	now := s.clock.Now()

	if sendErr == nil {
		if err := s.repo.MarkOutboxSent(ctx, row.ID, now); err != nil {
			slog.Error("notification: gagal menandai outbox sent", "id", row.ID, "err", err)
		}
		return
	}

	if errors.Is(sendErr, ErrNoContact) {
		slog.Warn("notification: penerima tanpa kontak, ditandai dead", "id", row.ID, "channel", row.Channel, "user_id", row.UserID)
		if err := s.repo.MarkOutboxDead(ctx, row.ID, row.Attempts); err != nil {
			slog.Error("notification: gagal menandai outbox dead", "id", row.ID, "err", err)
		}
		return
	}

	attempts := row.Attempts + 1
	delay, dead := nextBackoff(int(attempts))
	if dead {
		slog.Warn("notification: outbox habis percobaan, ditandai dead", "id", row.ID, "channel", row.Channel, "attempts", attempts, "err", sendErr)
		if err := s.repo.MarkOutboxDead(ctx, row.ID, attempts); err != nil {
			slog.Error("notification: gagal menandai outbox dead", "id", row.ID, "err", err)
		}
		return
	}
	slog.Warn("notification: pengiriman gagal, dijadwalkan retry", "id", row.ID, "channel", row.Channel, "attempts", attempts, "retry_in", delay, "err", sendErr)
	if err := s.repo.MarkOutboxRetry(ctx, row.ID, attempts, now.Add(delay)); err != nil {
		slog.Error("notification: gagal menjadwalkan retry outbox", "id", row.ID, "err", err)
	}
}

// StartWorker menjalankan loop polling outbox setiap interval (docs/08-notification.md
// "worker goroutine membaca outbox... poll pending & next_retry_at due tiap
// 10 dtk, batch 50"). Berhenti dengan rapi saat ctx dibatalkan (graceful
// shutdown, lihat cmd/server/main.go).
func StartWorker(ctx context.Context, svc *Service, interval time.Duration, batchSize int) {
	slog.Info("notification: worker outbox jalan", "interval", interval, "batch_size", batchSize)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("notification: worker outbox berhenti")
			return
		case <-ticker.C:
			if _, err := svc.ProcessDueOutbox(ctx, batchSize); err != nil {
				slog.Error("notification: worker outbox gagal memproses batch", "err", err)
			}
		}
	}
}
