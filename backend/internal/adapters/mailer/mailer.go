// Package mailer implementa el puerto de correo. En local apunta a Mailpit;
// en la nube, al SMTP de un proveedor (ADR-0010).
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
	auth smtp.Auth
}

// New construye el adaptador.
//
// `usuario` vacío significa sin autenticación, que es como habla Mailpit. Con
// usuario se usa PLAIN, que el paquete `smtp` solo envía sobre una conexión
// cifrada: si el servidor no ofrece STARTTLS, la llamada falla en vez de
// mandar la contraseña en claro.
//
// La distinción importa porque no es una comodidad de desarrollo: ningún
// proveedor real acepta correo sin autenticar, así que sin esto la
// verificación por correo no saldría del entorno local.
func New(addr, from, usuario, contrasena string) *SMTP {
	m := &SMTP{addr: addr, from: from}
	if usuario != "" {
		m.auth = smtp.PlainAuth("", usuario, contrasena, hostDe(addr))
	}
	return m
}

// hostDe recorta el puerto: PLAIN se valida contra el host, no contra
// host:puerto.
func hostDe(addr string) string {
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}

func (m *SMTP) Send(ctx context.Context, to, subject, body string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", m.from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(body)

	if err := smtp.SendMail(m.addr, m.auth, m.from, []string{to}, []byte(b.String())); err != nil {
		return fmt.Errorf("enviar correo a %s: %w", to, err)
	}
	return nil
}
