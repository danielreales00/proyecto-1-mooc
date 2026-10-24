package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Dependencia es algo sin lo que el proceso no puede atender: la base, Redis o
// el almacén de objetos.
type Dependencia struct {
	Nombre string
	Ping   func(context.Context) error
}

// Disponibilidad responde 200 si todas las dependencias contestan y 503 si
// alguna falla. Es la comprobación de salud del balanceador y del grupo de
// instancias, así que sale a Internet: el cuerpo solo dice `ok` o `fail` por
// dependencia, y el error, que puede traer usuario, base o dirección interna,
// va al log (DT-04).
func Disponibilidad(log *slog.Logger, deps ...Dependencia) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		estado := make(map[string]string, len(deps))
		codigo := http.StatusOK
		for _, d := range deps {
			if err := d.Ping(ctx); err != nil {
				log.Warn("dependencia no disponible", "dependencia", d.Nombre, "error", err)
				estado[d.Nombre] = "fail"
				codigo = http.StatusServiceUnavailable
				continue
			}
			estado[d.Nombre] = "ok"
		}
		JSON(w, codigo, estado)
	}
}
