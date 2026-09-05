package identity

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Mailer es el puerto de correo. Lo implementa adapters/mailer.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// EmailWorker atiende el trabajo email.send. Vive en el módulo porque el
// contenido del correo es dominio; el transporte es el adaptador.
type EmailWorker struct {
	mailer  Mailer
	baseURL string
	log     *slog.Logger
}

func NewEmailWorker(m Mailer, baseURL string, log *slog.Logger) *EmailWorker {
	return &EmailWorker{mailer: m, baseURL: strings.TrimRight(baseURL, "/"), log: log}
}

// Handle es idempotente por construcción: enviar el mismo correo dos veces no
// cambia el estado del sistema, y el reclamo del runner impide que ocurra
// (ADR-0008).
func (w *EmailWorker) Handle(ctx context.Context, data map[string]any) error {
	template, _ := data["template"].(string)
	to, _ := data["to"].(string)
	name, _ := data["name"].(string)
	token, _ := data["token"].(string)

	if to == "" {
		return fmt.Errorf("email.send sin destinatario")
	}

	switch template {
	case PurposeEmailVerify:
		subject := "Verifica tu cuenta"
		body := fmt.Sprintf(`Hola %s:

Para activar tu cuenta, verifica tu correo.

Token de verificación:
%s

Desde Postman o curl:

  POST %s/api/v1/auth/verify-email
  { "token": "%s" }

El token expira en 24 horas.

Plataforma MOOC
`, name, token, w.baseURL, token)
		return w.mailer.Send(ctx, to, subject, body)

	case PurposePasswordReset:
		subject := "Restablece tu contraseña"
		body := fmt.Sprintf(`Hola %s:

Token para restablecer tu contraseña:
%s

Expira en 1 hora. Si no lo pediste, ignora este mensaje.

Plataforma MOOC
`, name, token)
		return w.mailer.Send(ctx, to, subject, body)

	default:
		// Una plantilla desconocida no mejora con reintentos.
		return fmt.Errorf("plantilla de correo desconocida: %q", template)
	}
}
