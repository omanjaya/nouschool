package notification

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
)

const (
	platformConfigVAPIDPublic  = "vapid_public_key"
	platformConfigVAPIDPrivate = "vapid_private_key"
)

// LoadOrGenerateVAPIDKeys mengembalikan pasangan kunci VAPID dari env
// VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY bila terisi; bila kosong, dibaca dari
// tabel platform_config; bila belum ada baris sama sekali, DI-GENERATE lalu
// disimpan supaya STABIL antar-restart (docs fase 9: "generate saat startup,
// simpan ke tabel platform_config... supaya stabil antar-restart").
func LoadOrGenerateVAPIDKeys(ctx context.Context, repo *Repository, envPublic, envPrivate string) (publicKey, privateKey string, err error) {
	if envPublic != "" && envPrivate != "" {
		return envPublic, envPrivate, nil
	}

	pub, pubOK, err := repo.GetPlatformConfig(ctx, platformConfigVAPIDPublic)
	if err != nil {
		return "", "", err
	}
	priv, privOK, err := repo.GetPlatformConfig(ctx, platformConfigVAPIDPrivate)
	if err != nil {
		return "", "", err
	}
	if pubOK && privOK {
		return pub, priv, nil
	}

	priv, pub, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	if err := repo.SetPlatformConfig(ctx, platformConfigVAPIDPublic, pub); err != nil {
		return "", "", err
	}
	if err := repo.SetPlatformConfig(ctx, platformConfigVAPIDPrivate, priv); err != nil {
		return "", "", err
	}
	slog.Info("notification: pasangan kunci VAPID baru di-generate & disimpan", "public_key", pub)
	return pub, priv, nil
}

// pushSender adalah kebutuhan WebPushProvider dari library webpush-go —
// dideklarasikan sebagai interface supaya bisa dites tanpa jaringan nyata
// (lihat service_test.go: fake pushSender mengembalikan 410 utk memverifikasi
// subscription dihapus).
type pushSender interface {
	Send(message []byte, sub *webpush.Subscription, options *webpush.Options) (*http.Response, error)
}

type realPushSender struct{}

func (realPushSender) Send(message []byte, sub *webpush.Subscription, options *webpush.Options) (*http.Response, error) {
	return webpush.SendNotification(message, sub, options)
}

// subscriptionRepo adalah kebutuhan WebPushProvider dari repository —
// dideklarasikan sebagai interface kecil (dipenuhi *Repository secara
// struktural) supaya provider bisa dites dengan fake repository in-memory,
// tanpa DB (lihat service_test.go).
type subscriptionRepo interface {
	ListPushSubscriptionsForUser(ctx context.Context, schoolID, userID int64) ([]PushSubscriptionRow, error)
	DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error
}

// WebPushProvider mengirim lewat Web Push (VAPID, lib webpush-go —
// docs/08-notification.md "web_push"). Selalu dianggap "configured": VAPID
// di-generate otomatis saat startup bila env kosong (lihat LoadOrGenerateVAPIDKeys),
// jadi TIDAK mengimplementasikan Configurable.
type WebPushProvider struct {
	repo       subscriptionRepo
	sender     pushSender
	publicKey  string
	privateKey string
	subscriber string // VAPID "sub" claim — mailto: atau URL kontak platform
}

func NewWebPushProvider(repo subscriptionRepo, publicKey, privateKey, subscriber string) *WebPushProvider {
	return &WebPushProvider{repo: repo, sender: realPushSender{}, publicKey: publicKey, privateKey: privateKey, subscriber: subscriber}
}

// Send mengirim ke SEMUA subscription (device) milik penerima. Subscription
// yang ditolak push service dgn 404/410 (endpoint sudah tidak berlaku)
// DIHAPUS dari push_subscriptions (docs/08 scope tugas fase 9). Bila TIDAK
// ADA subscription tersisa yang berhasil dikirimi, Send mengembalikan error
// (worker menjadwalkan retry normal — subscription bisa ditambahkan lagi
// oleh user kapan saja, BEDA dari kasus whatsapp/email yang bergantung pada
// data profil statis — lihat keputusan di whatsapp.go).
func (p *WebPushProvider) Send(ctx context.Context, msg RenderedMessage) error {
	subs, err := p.repo.ListPushSubscriptionsForUser(ctx, msg.SchoolID, msg.UserID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return errors.New("notification: penerima belum punya subscription web push")
	}

	payload, err := json.Marshal(map[string]any{"title": msg.Title, "body": msg.Body, "link": msg.Link})
	if err != nil {
		return err
	}
	opts := &webpush.Options{
		Subscriber:      p.subscriber,
		VAPIDPublicKey:  p.publicKey,
		VAPIDPrivateKey: p.privateKey,
		TTL:             60,
	}

	var lastErr error
	delivered := 0
	for _, sub := range subs {
		resp, err := p.sender.Send(payload, &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{Auth: sub.Auth, P256dh: sub.P256dh}}, opts)
		if err != nil {
			lastErr = err
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			if delErr := p.repo.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint); delErr != nil {
				slog.Error("notification: gagal menghapus subscription push kedaluwarsa", "endpoint", sub.Endpoint, "err", delErr)
			}
			lastErr = errors.New("notification: subscription push sudah tidak berlaku (dihapus)")
			continue
		}
		if resp.StatusCode >= 300 {
			lastErr = errors.New("notification: push service menolak, status " + resp.Status)
			continue
		}
		delivered++
	}
	if delivered > 0 {
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("notification: tidak ada subscription push yang berhasil dikirimi")
	}
	return lastErr
}
