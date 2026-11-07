package queue

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPushDescartaLoQueNoSeArreglaReintentando(t *testing.T) {
	p := NewPush(nil, map[string]Handler{"email.send": nil}, 5,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	for nombre, cuerpo := range map[string]string{
		"sobre ilegible":   `{no es json`,
		"tipo ausente":     `{"message":{"data":"e30=","attributes":{}}}`,
		"tipo ajeno":       `{"message":{"data":"e30=","attributes":{"tipo":"otro.trabajo"}}}`,
		"datos sin base64": `{"message":{"data":"%%%","attributes":{"tipo":"email.send"}}}`,
	} {
		w := httptest.NewRecorder()
		p.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/pubsub/push", strings.NewReader(cuerpo)))
		// 204 confirma: Pub/Sub no lo reintenta en bucle hasta los muertos.
		if w.Code != http.StatusNoContent {
			t.Errorf("%s: código %d, se esperaba 204", nombre, w.Code)
		}
	}
}

func TestPushUltimaEntrega(t *testing.T) {
	p := &Push{maxEntregas: 5}
	for intento, ultima := range map[int]bool{1: false, 4: false, 5: true, 6: true} {
		if got := p.ultimaEntrega(intento); got != ultima {
			t.Errorf("entrega %d: última = %v, se esperaba %v", intento, got, ultima)
		}
	}
	if (&Push{}).ultimaEntrega(9) {
		t.Error("sin topic de muertos no hay última entrega")
	}
}
