package main

import (
	"testing"
	"time"
)

// Los trabajos se cortan antes de que asynq deje de esperarlos, y con margen
// para que el más largo de la Entrega 2 (196 s) termine en worker-media.
func TestPlazoDeCorte(t *testing.T) {
	casos := []struct {
		plazo, want time.Duration
	}{
		{8 * time.Second, 7200 * time.Millisecond},
		{50 * time.Second, 45 * time.Second},
		{280 * time.Second, 252 * time.Second},
	}
	for _, c := range casos {
		got := plazoDeCorte(c.plazo)
		if got != c.want {
			t.Errorf("plazoDeCorte(%v) = %v, quiero %v", c.plazo, got, c.want)
		}
		if got >= c.plazo {
			t.Errorf("plazoDeCorte(%v) = %v: no deja margen antes del plazo", c.plazo, got)
		}
	}
	if plazoDeCorte(280*time.Second) <= 196*time.Second {
		t.Error("la transcodificación más larga medida no cabría antes del corte")
	}
}
