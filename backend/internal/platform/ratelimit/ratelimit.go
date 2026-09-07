// Package ratelimit implementa límites de tasa sobre Redis (CE-02).
//
// Ventana fija con contador: INCR más EXPIRE al crear la clave. Es lo más
// barato que da una garantía atómica, y basta para lo que protege aquí —
// fuerza bruta de contraseñas y de tokens, e inundación de evidencias de
// progreso.
//
// La ventana fija admite una ráfaga en el borde: 5 peticiones al final de un
// minuto y 5 al principio del siguiente pasan, aunque el límite sea 5/min. Con
// una ventana deslizante no ocurriría, a cambio de guardar una marca de tiempo
// por petición. Para frenar un ataque de fuerza bruta la diferencia es
// irrelevante; si algún día hace falta precisión, se cambia aquí sin tocar a
// quien llama.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Regla describe un límite. Se declaran como constantes junto al endpoint que
// protegen, para que el número esté a la vista de quien lee el handler.
type Regla struct {
	// Nombre entra en la clave de Redis y en las métricas.
	Nombre string
	// Limite es cuántas peticiones se admiten por ventana.
	Limite int
	// Ventana es la duración de la ventana.
	Ventana time.Duration
}

// Veredicto es el resultado de consultar el límite.
type Veredicto struct {
	Permitido bool
	// Restantes es cuántas quedan en esta ventana; 0 cuando ya se superó.
	Restantes int
	// ReintentarEn es lo que falta para que la ventana se renueve.
	ReintentarEn time.Duration
}

type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter { return &Limiter{rdb: rdb} }

// incrementarYExpirar hace las dos operaciones en un solo viaje y de forma
// atómica. Sin el script, un proceso que muriera entre el INCR y el EXPIRE
// dejaría una clave sin caducidad, y ese sujeto quedaría bloqueado para
// siempre.
var incrementarYExpirar = redis.NewScript(`
	local actual = redis.call('INCR', KEYS[1])
	if actual == 1 then
		redis.call('PEXPIRE', KEYS[1], ARGV[1])
	end
	return {actual, redis.call('PTTL', KEYS[1])}
`)

// Permitir consulta y consume una unidad del límite.
//
// Si Redis falla, se PERMITE la petición. Un limitador caído no debe dejar el
// servicio inaccesible: se prefiere aceptar tráfico de más a rechazar tráfico
// legítimo. El error se devuelve para que el llamante lo registre.
func (l *Limiter) Permitir(ctx context.Context, r Regla, sujeto string) (Veredicto, error) {
	clave := fmt.Sprintf("rl:%s:%s", r.Nombre, sujeto)

	res, err := incrementarYExpirar.Run(ctx, l.rdb, []string{clave},
		r.Ventana.Milliseconds()).Slice()
	if err != nil {
		return Veredicto{Permitido: true}, fmt.Errorf("consultar el límite %s: %w", r.Nombre, err)
	}

	actual, _ := res[0].(int64)
	ttlMs, _ := res[1].(int64)
	reintentar := time.Duration(ttlMs) * time.Millisecond
	if reintentar < 0 {
		reintentar = r.Ventana
	}

	restantes := r.Limite - int(actual)
	if restantes < 0 {
		restantes = 0
	}
	return Veredicto{
		Permitido:    int(actual) <= r.Limite,
		Restantes:    restantes,
		ReintentarEn: reintentar,
	}, nil
}
