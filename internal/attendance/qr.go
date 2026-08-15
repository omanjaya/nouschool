package attendance

import "github.com/skip2/go-qrcode"

// qrPixelSize — ukuran sisi PNG QR kartu siswa (sama seperti QR ruangan,
// lihat internal/schedule/qr.go & docs/04-schedule.md: 512).
const qrPixelSize = 512

// qrCardPrefix adalah awalan isi QR kartu siswa — token MENTAH tidak pernah
// dipindai langsung (docs/05-attendance.md "QR kartu siswa": "token acak,
// BUKAN NIS polos"). Sisi scan (ScanQRCard) menerima dengan ATAU tanpa
// awalan ini (lihat normalizeQRToken di service.go).
const qrCardPrefix = "nouschool:student:"

// generateStudentQRPNG menghasilkan gambar PNG (persegi qrPixelSize) berisi
// string "nouschool:student:{token}" — dicetak di kartu fisik siswa.
func generateStudentQRPNG(token string) ([]byte, error) {
	return qrcode.Encode(qrCardPrefix+token, qrcode.Medium, qrPixelSize)
}
