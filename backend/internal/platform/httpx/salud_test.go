package httpx

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDisponibilidadNoExponeElError(t *testing.T) {
	secreto := "dial tcp 10.20.0.8:5432: user=mooc database=mooc"
	h := Disponibilidad(slog.New(slog.NewTextHandler(io.Discard, nil)),
		Dependencia{"postgres", func(context.Context) error { return errors.New(secreto) }},
		Dependencia{"redis", func(context.Context) error { return nil }},
	)
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("código = %d, se esperaba 503", w.Code)
	}
	cuerpo := w.Body.String()
	if strings.Contains(cuerpo, "10.20.0.8") || strings.Contains(cuerpo, "user=") {
		t.Fatalf("el cuerpo expone el error interno: %s", cuerpo)
	}
	for _, esperado := range []string{`"postgres":"fail"`, `"redis":"ok"`} {
		if !strings.Contains(cuerpo, esperado) {
			t.Errorf("falta %s en %s", esperado, cuerpo)
		}
	}
}

func TestDisponibilidadSana(t *testing.T) {
	h := Disponibilidad(slog.New(slog.NewTextHandler(io.Discard, nil)),
		Dependencia{"postgres", func(context.Context) error { return nil }})
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("código = %d, se esperaba 200", w.Code)
	}
}
