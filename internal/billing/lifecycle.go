package billing

import (
	"context"
	"log/slog"
	"time"
)

// StartLifecycleWorker menjalankan job harian transisi lifecycle langganan
// (docs/09-billing.md "Transisi dijalankan job harian ... cukup goroutine
// ticker — tanpa dependensi eksternal"). interval biasanya 1 jam (job harian
// tidak butuh presisi tinggi, tapi dicek berkala supaya transisi tidak
// menunggu sampai 24 jam bila server baru saja start). TickOnce IDEMPOTEN —
// aman dipanggil ulang oleh ticker maupun manual (mis. startup, verifikasi).
// Berhenti dengan rapi saat ctx dibatalkan (graceful shutdown, sama seperti
// internal/notification.StartWorker).
func StartLifecycleWorker(ctx context.Context, svc *Service, interval time.Duration) {
	slog.Info("billing: worker lifecycle jalan", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("billing: worker lifecycle berhenti")
			return
		case <-ticker.C:
			if err := svc.TickOnce(ctx); err != nil {
				slog.Error("billing: worker lifecycle gagal", "err", err)
			}
		}
	}
}
