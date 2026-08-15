package notification

import "time"

// backoffSchedule — jeda retry per percobaan ke-N (1-indexed): percobaan #1
// gagal -> tunggu 1 menit, #2 -> 5 menit, #3 -> 30 menit, #4 -> 2 jam.
// Percobaan #5 yang gagal -> 'dead' (docs/08-notification.md "retry dengan
// backoff, max N, lalu dead").
var backoffSchedule = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
}

// nextBackoff adalah fungsi MURNI (mudah dites tanpa DB/worker, lihat
// service_test.go): attempts adalah jumlah percobaan SETELAH kegagalan ini
// (sudah di-increment oleh pemanggil). dead=true berarti attempts sudah
// melebihi panjang backoffSchedule — baris outbox ditandai 'dead'.
func nextBackoff(attempts int) (delay time.Duration, dead bool) {
	if attempts <= 0 {
		attempts = 1
	}
	idx := attempts - 1
	if idx >= len(backoffSchedule) {
		return 0, true
	}
	return backoffSchedule[idx], false
}
