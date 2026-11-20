package queue

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestEventoConfirmaLoQueNoSeArreglaReintentando(t *testing.T) {
	e := NewEvento(nil, "badge.issue", nil, 5, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for nombre, datos := range map[string]string{
		"evento ilegible": `{no es json`,
		"tipo ausente":    `{"message":{"data":"e30=","attributes":{}}}`,
		"tipo ajeno":      `{"message":{"data":"e30=","attributes":{"tipo":"email.send"}}}`,
	} {
		// nil confirma: Eventarc no lo reintenta durante días.
		if err := e.Atender(context.Background(), []byte(datos)); err != nil {
			t.Errorf("%s: error %v, se esperaba nil", nombre, err)
		}
	}
}

func TestEventoAgotado(t *testing.T) {
	e := &Evento{maxIntentos: 5}
	for intentos, agotado := range map[int]bool{1: false, 4: false, 5: true, 6: true} {
		if got := e.agotado(intentos); got != agotado {
			t.Errorf("intento %d: agotado = %v, se esperaba %v", intentos, got, agotado)
		}
	}
	if (&Evento{}).agotado(99) {
		t.Error("sin límite no se agota")
	}
}
