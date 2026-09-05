package identity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

type fakeMailer struct {
	enviados []struct{ to, subject, body string }
	fallo    error
}

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	if f.fallo != nil {
		return f.fallo
	}
	f.enviados = append(f.enviados, struct{ to, subject, body string }{to, subject, body})
	return nil
}

func nuevoWorker() (*EmailWorker, *fakeMailer) {
	m := &fakeMailer{}
	return NewEmailWorker(m, "http://localhost:8090/", slog.New(slog.NewTextHandler(io.Discard, nil))), m
}

// El correo de verificación es la única forma de activar una cuenta: si el
// token no llega, la demostración se detiene en el segmento 1.
func TestEmailWorkerEnviaElTokenDeVerificacion(t *testing.T) {
	w, m := nuevoWorker()
	const token = "H1J6XRe_zD5mhvTe6I4Q408YXTjwCAFzKSI2lOySLB0"

	err := w.Handle(context.Background(), map[string]any{
		"template": PurposeEmailVerify,
		"to":       "ana@mooc.local",
		"name":     "Ana",
		"token":    token,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(m.enviados) != 1 {
		t.Fatalf("se esperaba 1 correo, hubo %d", len(m.enviados))
	}

	env := m.enviados[0]
	if env.to != "ana@mooc.local" {
		t.Errorf("destinatario = %q", env.to)
	}
	if !strings.Contains(env.body, token) {
		t.Error("el cuerpo no contiene el token")
	}
	if !strings.Contains(env.body, "Ana") {
		t.Error("el cuerpo no saluda por el nombre")
	}

	// El smoke y la colección de Postman extraen el token con una expresión
	// anclada a una línea propia: si el formato cambia, se rompen los dos.
	var solo bool
	for _, linea := range strings.Split(env.body, "\n") {
		if strings.TrimSpace(linea) == token {
			solo = true
			break
		}
	}
	if !solo {
		t.Error("el token debe ir en una línea propia; scripts/smoke.sh y Postman lo extraen así")
	}

	// La URL base se normaliza: no debe quedar una barra doble.
	if strings.Contains(env.body, "8090//api") {
		t.Error("la barra final de PUBLIC_BASE_URL no se recortó")
	}
	if !strings.Contains(env.body, "http://localhost:8090/api/v1/auth/verify-email") {
		t.Error("el cuerpo no indica a dónde enviar el token")
	}
}

func TestEmailWorkerPlantillaDeRestablecimiento(t *testing.T) {
	w, m := nuevoWorker()

	err := w.Handle(context.Background(), map[string]any{
		"template": PurposePasswordReset,
		"to":       "ana@mooc.local",
		"name":     "Ana",
		"token":    "token-de-restablecimiento",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(m.enviados) != 1 {
		t.Fatalf("se esperaba 1 correo, hubo %d", len(m.enviados))
	}
	if !strings.Contains(m.enviados[0].body, "token-de-restablecimiento") {
		t.Error("el cuerpo no contiene el token")
	}
}

func TestEmailWorkerRechazaTrabajosInvalidos(t *testing.T) {
	casos := map[string]map[string]any{
		"sin destinatario":      {"template": PurposeEmailVerify, "token": "x"},
		"plantilla desconocida": {"template": "inventada", "to": "ana@mooc.local"},
		"payload vacío":         {},
	}
	for nombre, data := range casos {
		t.Run(nombre, func(t *testing.T) {
			w, m := nuevoWorker()
			if err := w.Handle(context.Background(), data); err == nil {
				t.Fatal("se esperaba error y no lo hubo")
			}
			if len(m.enviados) != 0 {
				t.Error("no debe enviarse nada cuando el trabajo es inválido")
			}
		})
	}
}

// Un fallo del servidor de correo debe propagarse para que asynq reintente con
// backoff y, tras tres intentos, el trabajo llegue a la DLQ (CA-03).
func TestEmailWorkerPropagaElFalloDelServidorDeCorreo(t *testing.T) {
	w, m := nuevoWorker()
	m.fallo = errors.New("smtp no responde")

	err := w.Handle(context.Background(), map[string]any{
		"template": PurposeEmailVerify,
		"to":       "ana@mooc.local",
		"name":     "Ana",
		"token":    "x",
	})
	if err == nil {
		t.Fatal("el fallo de SMTP debe propagarse para que se reintente")
	}
}
