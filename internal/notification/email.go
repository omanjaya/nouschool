package notification

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// EmailProvider mengirim lewat SMTP (docs/08-notification.md "email": "SMTP
// (config platform), template HTML sederhana" — Fase 9 scope: plain text
// dulu). Konfigurasi platform: SMTP_HOST, SMTP_PORT, SMTP_USERNAME,
// SMTP_PASSWORD, SMTP_FROM.
type EmailProvider struct {
	repo     contactRepo
	host     string
	port     string
	username string
	password string
	from     string
	sendMail func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

func NewEmailProvider(repo contactRepo, host, port, username, password, from string) *EmailProvider {
	return &EmailProvider{repo: repo, host: host, port: port, username: username, password: password, from: from, sendMail: smtp.SendMail}
}

// Configured melaporkan apakah SMTP_HOST terisi (docs fase 9: "channel yang
// config platform-nya kosong -> outbox row TIDAK dibuat").
func (p *EmailProvider) Configured() bool { return p.host != "" }

// Send mengirim email plain text sederhana.
//
// KEPUTUSAN DESAIN (konsisten dengan whatsapp.go): penerima tanpa alamat
// email tersimpan (users.email kosong) -> ErrNoContact -> worker menandai
// outbox 'dead' LANGSUNG (attempts tetap 0), dengan alasan yang sama:
// data profil kosong bukan kegagalan sementara.
func (p *EmailProvider) Send(ctx context.Context, msg RenderedMessage) error {
	_, email, err := p.repo.GetUserContact(ctx, msg.UserID)
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrNoContact
	}

	var auth smtp.Auth
	if p.username != "" {
		auth = smtp.PlainAuth("", p.username, p.password, p.host)
	}

	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n",
		p.from, email, msg.Title, msg.Body)

	addr := p.host + ":" + p.port
	return p.sendMail(addr, auth, p.from, []string{email}, []byte(body))
}
