package cursor

import (
	"testing"
	"time"
)

func TestIdaYVuelta(t *testing.T) {
	f := time.Date(2026, 9, 7, 15, 4, 5, 123456789, time.UTC)
	c, ok := Decodificar(Codificar(f, "abc-123"))
	if !ok {
		t.Fatal("no se pudo decodificar lo que se acababa de codificar")
	}
	if !c.Fecha.Equal(f) {
		t.Errorf("fecha = %v, se esperaba %v", c.Fecha, f)
	}
	if c.ID != "abc-123" {
		t.Errorf("id = %q", c.ID)
	}
}

// Un cursor inválido no debe reventar: se trata como «desde el principio».
func TestCursorInvalido(t *testing.T) {
	for _, v := range []string{"", "no-es-base64!!", "YWJj", "Zm9vfGJhcg"} {
		if _, ok := Decodificar(v); ok {
			t.Errorf("Decodificar(%q) lo dio por válido", v)
		}
	}
}

func TestLimite(t *testing.T) {
	casos := []struct{ pedido, esperado int }{
		{0, 20}, {-5, 20}, {10, 10}, {100, 100}, {1000, 100},
	}
	for _, c := range casos {
		if got := Limite(c.pedido, 20, 100); got != c.esperado {
			t.Errorf("Limite(%d) = %d, se esperaba %d", c.pedido, got, c.esperado)
		}
	}
}

// Se consulta un elemento de más para saber si hay página siguiente sin contar
// el total.
func TestPagina(t *testing.T) {
	type fila struct {
		f  time.Time
		id string
	}
	clave := func(x fila) (time.Time, string) { return x.f, x.id }
	base := time.Now().UTC()

	t.Run("cabe entero: no hay cursor siguiente", func(t *testing.T) {
		filas := []fila{{base, "a"}, {base, "b"}}
		out, sig := Pagina(filas, 5, clave)
		if len(out) != 2 || sig != nil {
			t.Fatalf("out=%d sig=%v", len(out), sig)
		}
	})

	t.Run("sobra uno: recorta y devuelve cursor", func(t *testing.T) {
		filas := []fila{{base, "a"}, {base, "b"}, {base, "c"}}
		out, sig := Pagina(filas, 2, clave)
		if len(out) != 2 {
			t.Fatalf("debería recortar a 2, dio %d", len(out))
		}
		if sig == nil {
			t.Fatal("debería haber cursor siguiente")
		}
		c, _ := Decodificar(*sig)
		if c.ID != "b" {
			t.Errorf("el cursor debe apuntar a la ÚLTIMA fila devuelta, no a la descartada: %q", c.ID)
		}
	})
}
