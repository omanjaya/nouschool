package notification

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// contactRepo adalah kebutuhan WhatsAppProvider/EmailProvider dari repository
// — dideklarasikan sebagai interface kecil (dipenuhi *Repository secara
// struktural) supaya provider bisa dites dengan fake repository in-memory,
// tanpa DB (lihat service_test.go).
type contactRepo interface {
	GetUserContact(ctx context.Context, userID int64) (phone, email string, err error)
}

// WhatsAppProvider mengirim lewat HTTP gateway generik gaya Fonnte
// (docs/08-notification.md "whatsapp": "Fonnte-style HTTP gateway (murah,
// cepat mulai)"). Konfigurasi platform: WA_GATEWAY_URL, WA_GATEWAY_TOKEN.
type WhatsAppProvider struct {
	repo   contactRepo
	url    string
	token  string
	client *http.Client
}

func NewWhatsAppProvider(repo contactRepo, gatewayURL, token string) *WhatsAppProvider {
	return &WhatsAppProvider{repo: repo, url: gatewayURL, token: token, client: http.DefaultClient}
}

// Configured melaporkan apakah WA_GATEWAY_URL terisi (docs fase 9: "channel
// yang config platform-nya kosong -> outbox row TIDAK dibuat").
func (p *WhatsAppProvider) Configured() bool { return p.url != "" }

// Send mem-POST {target, message} ke gateway (form-urlencoded, header
// Authorization = token — pola umum gateway WA gaya Fonnte).
//
// KEPUTUSAN DESAIN (didokumentasikan sesuai instruksi tugas): bila penerima
// TIDAK punya nomor telepon tersimpan (users.phone kosong), Send
// mengembalikan ErrNoContact — worker menandai baris outbox 'dead' LANGSUNG
// dengan attempts TETAP 0 (lihat worker.go), karena mengulang tidak akan
// mengubah hasil sampai admin mengisi nomor telepon user tsb (perubahan data
// profil, bukan kegagalan sementara — beda kelas masalah dari kegagalan
// jaringan/gateway yang wajar di-retry dengan backoff).
func (p *WhatsAppProvider) Send(ctx context.Context, msg RenderedMessage) error {
	phone, _, err := p.repo.GetUserContact(ctx, msg.UserID)
	if err != nil {
		return err
	}
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ErrNoContact
	}

	form := url.Values{"target": {phone}, "message": {msg.Title + "\n\n" + msg.Body}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if p.token != "" {
		req.Header.Set("Authorization", p.token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return &httpStatusError{status: resp.StatusCode}
	}
	return nil
}

type httpStatusError struct{ status int }

func (e *httpStatusError) Error() string {
	return "notification: gateway WA membalas status non-2xx: " + http.StatusText(e.status)
}
