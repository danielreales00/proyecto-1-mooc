package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// De esta función depende el límite de tasa por IP, así que la prueba cubre el
// caso adverso: un cliente que manda su propia cabecera para hacerse pasar por
// otro en cada petición.
func TestClientIP(t *testing.T) {
	casos := []struct {
		nombre   string
		hops     int
		xff      string
		remoto   string
		esperado string
	}{
		{
			nombre: "sin cabecera usa la conexión, sin el puerto",
			remoto: "203.0.113.9:54321", esperado: "203.0.113.9",
		},
		{
			nombre: "con Caddy delante, que reescribe la cabecera entera",
			hops:   0, xff: "203.0.113.9", remoto: "172.19.0.5:1234",
			esperado: "203.0.113.9",
		},
		{
			nombre: "detrás de un balanceador que añade la suya",
			hops:   1, xff: "203.0.113.9, 130.211.0.1", remoto: "169.254.1.1:8080",
			esperado: "203.0.113.9",
		},
		{
			nombre: "el cliente miente y el balanceador añade detrás",
			hops:   1, xff: "1.2.3.4, 203.0.113.9, 130.211.0.1",
			remoto:   "169.254.1.1:8080",
			esperado: "203.0.113.9",
		},
		{
			nombre: "vienen menos saltos de los configurados",
			hops:   2, xff: "203.0.113.9", remoto: "169.254.1.1:8080",
			esperado: "203.0.113.9",
		},
		{
			nombre: "espacios alrededor",
			hops:   1, xff: "  203.0.113.9 ,  130.211.0.1  ", remoto: "169.254.1.1:8080",
			esperado: "203.0.113.9",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			ConfigurarProxiesDeConfianza(c.hops)
			defer ConfigurarProxiesDeConfianza(0)

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = c.remoto
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}

			if got := ClientIP(r); got != c.esperado {
				t.Fatalf("ClientIP = %q, se esperaba %q", got, c.esperado)
			}
		})
	}
}

// Un número negativo no puede convertirse en un índice desde la derecha que
// caiga fuera de la lista.
func TestConfigurarProxiesDeConfianzaNoAceptaNegativos(t *testing.T) {
	ConfigurarProxiesDeConfianza(-3)
	defer ConfigurarProxiesDeConfianza(0)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 130.211.0.1")
	if got := ClientIP(r); got != "130.211.0.1" {
		t.Fatalf("ClientIP = %q; con 0 saltos debe ser la última de la lista", got)
	}
}
