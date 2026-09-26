package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/platform/idempotency"
)

// Podador borra lo que ya no sirve de las tablas que solo crecen.
//
// Tres tablas se escriben en cada petición y no se borran nunca: el registro de
// trabajos, las claves de idempotencia y las evidencias de progreso. Ninguna
// molesta el primer mes y las tres acaban siendo el grueso de la base, con el
// agravante de que los índices se degradan antes que el disco se llene.
//
// Lo que NO se borra es tan importante como lo que sí:
//
//   - Los trabajos muertos se conservan enteros. Son la cola de fallos: la
//     evidencia de que algo se rompió y lo que permite reencolarlos (RF-02).
//   - Los trabajos vivos —en cola, corriendo o entre reintentos— tampoco, por
//     razones obvias.
//   - Las evidencias rechazadas se conservan igual que las aceptadas mientras
//     estén dentro de la ventana: son la prueba del CA-05, y borrar solo las
//     rechazadas dejaría la auditoría contando media historia.
type Podador struct {
	pool *pgxpool.Pool
	log  *slog.Logger

	// RetencionTrabajos es cuánto se guarda un trabajo ya completado con
	// éxito. Lo suficiente para investigar algo de la semana pasada.
	RetencionTrabajos time.Duration
	// RetencionEvidencias es cuánto se guardan las señales de progreso.
	RetencionEvidencias time.Duration
}

func NewPodador(pool *pgxpool.Pool, log *slog.Logger) *Podador {
	return &Podador{
		pool:                pool,
		log:                 log,
		RetencionTrabajos:   7 * 24 * time.Hour,
		RetencionEvidencias: 30 * 24 * time.Hour,
	}
}

// claveDeCandado identifica el candado consultivo de la poda. Es un número
// arbitrario y constante: lo único que importa es que nadie más use el mismo.
const claveDeCandado int64 = 0x6d6f6f63706f6461 // "moocpoda"

// Resumen es lo que se borró en una pasada.
type Resumen struct {
	Trabajos    int64
	Claves      int64
	Evidencias  int64
	SinCandado  bool
	Transcurrio time.Duration
}

// Run hace una pasada.
//
// Toma un candado consultivo primero: con varias instancias de worker, las tres
// intentarían podar a la vez y se estorbarían borrando lo mismo. La que no lo
// consigue no espera, porque la siguiente pasada llega igual.
func (p *Podador) Run(ctx context.Context) (Resumen, error) {
	inicio := time.Now()

	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return Resumen{}, err
	}
	defer conn.Release()

	var tomado bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, claveDeCandado).Scan(&tomado); err != nil {
		return Resumen{}, fmt.Errorf("pedir el candado de la poda: %w", err)
	}
	if !tomado {
		return Resumen{SinCandado: true}, nil
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock($1)`, claveDeCandado); err != nil {
			p.log.Warn("no se pudo soltar el candado de la poda", "error", err)
		}
	}()

	res := Resumen{}

	// Solo los que terminaron bien. Un 'dead' es la cola de fallos y un
	// 'failed' todavía puede reintentarse.
	tag, err := conn.Exec(ctx, `
		DELETE FROM platform.job_runs
		 WHERE status = 'succeeded'
		   AND finished_at < now() - $1::interval`,
		p.RetencionTrabajos.String())
	if err != nil {
		return res, fmt.Errorf("podar job_runs: %w", err)
	}
	res.Trabajos = tag.RowsAffected()

	// La ventana de las claves la fija el ADR-0008, no esta poda.
	res.Claves, err = idempotency.Podar(ctx, conn)
	if err != nil {
		return res, fmt.Errorf("podar idempotency_keys: %w", err)
	}

	// La borra `learning`, que es de quien es la tabla: escribir en el esquema
	// de otro módulo se salta sus reglas, y `make arch` lo comprueba.
	res.Evidencias, err = PodarEvidencias(ctx, conn, p.RetencionEvidencias)
	if err != nil {
		return res, fmt.Errorf("podar progress_events: %w", err)
	}

	res.Transcurrio = time.Since(inicio)
	return res, nil
}

// Start arranca el ciclo. Hace una pasada al empezar y luego cada `cada`.
//
// La primera pasada al arrancar importa: si el stack lleva semanas apagado, lo
// vencido ya está vencido y no hay motivo para esperar una hora más.
func (p *Podador) Start(ctx context.Context, cada time.Duration) {
	go func() {
		t := time.NewTicker(cada)
		defer t.Stop()
		for {
			p.pasada(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

func (p *Podador) pasada(ctx context.Context) {
	res, err := p.Run(ctx)
	switch {
	case err != nil:
		p.log.Error("la poda falló", "error", err)
	case res.SinCandado:
		p.log.Debug("otra instancia está podando")
	case res.Trabajos+res.Claves+res.Evidencias > 0:
		p.log.Info("poda",
			"trabajos", res.Trabajos,
			"claves_idempotencia", res.Claves,
			"evidencias_progreso", res.Evidencias,
			"duration_ms", res.Transcurrio.Milliseconds())
	}
}
