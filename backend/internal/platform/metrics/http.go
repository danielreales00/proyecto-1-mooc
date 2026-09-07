package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"mooc/backend/internal/platform/httpx"
)

// Handler sirve /metrics.
func Handler() http.Handler { return promhttp.Handler() }

type observador struct {
	http.ResponseWriter
	status int
}

func (o *observador) WriteHeader(c int) {
	o.status = c
	o.ResponseWriter.WriteHeader(c)
}

func (o *observador) Write(b []byte) (int, error) {
	if o.status == 0 {
		o.status = http.StatusOK
	}
	return o.ResponseWriter.Write(b)
}

// Middleware mide cada petición.
//
// La etiqueta `route` es el PATRÓN de la ruta, no la URL: con la URL, cada
// UUID crearía una serie temporal nueva y Prometheus acabaría inservible.
func Middleware() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inicio := time.Now()
			o := &observador{ResponseWriter: w}
			next.ServeHTTP(o, r)
			if o.status == 0 {
				o.status = http.StatusOK
			}

			patron := r.Pattern
			if patron == "" {
				patron = "desconocida"
			}
			etiquetas := []string{patron, r.Method, strconv.Itoa(o.status)}
			HTTPDuration.WithLabelValues(etiquetas...).Observe(time.Since(inicio).Seconds())
			HTTPRequests.WithLabelValues(etiquetas...).Inc()
		})
	}
}
