// Package metrics expone las métricas de Prometheus del ADR-0009.
//
// Solo las que alguien va a mirar. Una métrica que nadie consulta es ruido que
// hay que mantener; estas son las que sostienen un objetivo concreto: el p95
// de RNF-05, la alerta de DLQ que exige el §6 del enunciado, y las evidencias
// de idempotencia y de rechazo de progreso manipulado.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPDuration sostiene el objetivo de latencia p95 (RNF-05).
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duración de las peticiones HTTP.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"route", "method", "status"})

	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Peticiones HTTP atendidas.",
	}, []string{"route", "method", "status"})

	// JobsProcessed mide el rendimiento de los workers.
	JobsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_processed_total",
		Help: "Trabajos procesados por los workers.",
	}, []string{"type", "result"})

	// JobDuration da el costo por minuto de transcodificación que el §11 del
	// enunciado recomienda medir desde el MVP.
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "job_duration_seconds",
		Help:    "Duración de los trabajos asíncronos.",
		Buckets: []float64{.1, .5, 1, 5, 15, 60, 300, 900},
	}, []string{"type"})

	// JobsDeadLetter es la métrica que dispara la alerta del §6: "tras tres
	// reintentos fallidos, el trabajo llega a la DLQ y emite una alerta".
	JobsDeadLetter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_dead_letter_total",
		Help: "Trabajos que agotaron sus reintentos y llegaron a la dead-letter queue.",
	}, []string{"type"})

	// JobsDuplicateSkipped es la evidencia de idempotencia: sube cada vez que
	// una entrega duplicada se descarta sin producir una segunda salida.
	JobsDuplicateSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_duplicate_skipped_total",
		Help: "Entregas duplicadas descartadas por el reclamo atómico.",
	}, []string{"type"})

	// JobsReaped cuenta lo que recupera el reaper: si sube de forma sostenida,
	// algo va mal en la publicación a la cola.
	JobsReaped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_reaped_total",
		Help: "Trabajos huérfanos republicados por el reaper.",
	})

	// ProgressRejected es la evidencia de CA-05, desglosada por motivo.
	ProgressRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "progress_rejected_total",
		Help: "Evidencias de progreso descartadas por el servidor.",
	}, []string{"reason"})

	// RateLimited cuenta los 429 por regla.
	RateLimited = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limited_total",
		Help: "Peticiones rechazadas por exceder el límite de tasa.",
	}, []string{"rule"})
)

// Inicializar declara en cero los contadores de los tipos de trabajo conocidos.
//
// No es cosmético: `increase()` de Prometheus **no detecta** el salto de una
// serie que no existía a su primer valor. Un contador que solo aparece cuando
// ya vale 1 deja la alerta muda justo el día que hace falta.
//
// Declarándolos en cero al arrancar, el paso de 0 a 1 sí es un incremento
// observable, y `increase(jobs_dead_letter_total[5m]) > 0` dispara como debe.
func Inicializar(tiposDeTrabajo ...string) {
	for _, t := range tiposDeTrabajo {
		JobsDeadLetter.WithLabelValues(t)
		JobsDuplicateSkipped.WithLabelValues(t)
		JobsProcessed.WithLabelValues(t, "ok")
		JobsProcessed.WithLabelValues(t, "error")
	}
	// Lo mismo para los motivos de rechazo de progreso, que alimentan la
	// evidencia de CA-05.
	for _, motivo := range []string{
		"cadence", "position_jump", "position_out_of_range",
		"missing_open", "not_enrolled", "unknown_resource", "client_computed_value",
	} {
		ProgressRejected.WithLabelValues(motivo)
	}
}
