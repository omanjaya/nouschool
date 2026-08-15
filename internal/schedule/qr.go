package schedule

import "github.com/skip2/go-qrcode"

// qrPixelSize — ukuran sisi PNG QR ruangan (docs/04-schedule.md: 512).
const qrPixelSize = 512

// generateRoomQRPNG menghasilkan gambar PNG (persegi qrPixelSize) berisi
// string "nouschool:room:{qr_token}" — token acak unik per ruangan, ditempel
// fisik di ruangan dan dipindai guru (fase 6 teaching).
func generateRoomQRPNG(token string) ([]byte, error) {
	content := "nouschool:room:" + token
	return qrcode.Encode(content, qrcode.Medium, qrPixelSize)
}
