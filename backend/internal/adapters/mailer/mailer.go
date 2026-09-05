// Package mailer implementa el puerto de correo. En local apunta a Mailpit;
// en GCP se sustituye el adaptador sin tocar el dominio (ADR-0010).
package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

type SMTP struct {
	addr string
	from string
}

func New(addr, from string) *SMTP { return &SMTP{addr: addr, from: from} }

func (m *SMTP) Send(ctx context.Context, to, subject, body string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", m.from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(body)

	// Mailpit no pide autenticación; en producción el adaptador cambia.
	if err := smtp.SendMail(m.addr, nil, m.from, []string{to}, []byte(b.String())); err != nil {
		return fmt.Errorf("enviar correo a %s: %w", to, err)
	}
	return nil
}
