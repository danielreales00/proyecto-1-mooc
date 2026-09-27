package queue

import (
	"sort"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
)

// Métricas de la cola, que el enunciado exige como evidencia de capacidad:
// profundidad, antigüedad del trabajo más viejo y tasa de procesamiento.
//
// Los contadores de trabajos (jobs_processed_total) dicen cuánto se hizo, no
// cuánto espera. Esto lo pregunta a Redis en cada lectura de /metrics, con el
// Inspector de asynq: no mantiene estado propio que pueda desviarse del real.
//
// Cada proceso exporta solo las colas que consume. El worker y worker-media
// comparten Redis; si los dos exportaran todas, cada serie saldría dos veces.

var (
	descProfundidad = prometheus.NewDesc("queue_depth",
		"Trabajos en la cola por estado. archived son los muertos (DLQ).",
		[]string{"queue", "state"}, nil)
	descAntiguedad = prometheus.NewDesc("queue_oldest_pending_age_seconds",
		"Antigüedad del trabajo pendiente más viejo: cuánto lleva esperando.",
		[]string{"queue"}, nil)
	descProcesados = prometheus.NewDesc("queue_processed_total",
		"Trabajos procesados por la cola desde que existe, con éxito o no. rate() da la tasa.",
		[]string{"queue"}, nil)
	descFallidos = prometheus.NewDesc("queue_failed_total",
		"Ejecuciones fallidas en la cola desde que existe, incluidas las que se reintentan.",
		[]string{"queue"}, nil)
	descError = prometheus.NewDesc("queue_inspect_errors",
		"1 si la última lectura de la cola falló: las demás series de esa cola faltan.",
		[]string{"queue"}, nil)
)

// inspector es lo que usa el colector de asynq.Inspector; existe para poder
// probarlo sin Redis.
type inspector interface {
	GetQueueInfo(queue string) (*asynq.QueueInfo, error)
}

type ColectorCola struct {
	ins    inspector
	colas  []string
	cierre func() error
}

// NewColectorCola lee de Redis las colas dadas. Se cierra con Close.
func NewColectorCola(opt asynq.RedisClientOpt, colas []string) *ColectorCola {
	ins := asynq.NewInspector(opt)
	return &ColectorCola{ins: ins, colas: ordenadas(colas), cierre: ins.Close}
}

// ColasDe devuelve los nombres de una especificación WORKER_QUEUES.
func ColasDe(spec string) []string {
	out := make([]string, 0)
	for c := range ParseQueues(spec) {
		out = append(out, c)
	}
	return ordenadas(out)
}

func ordenadas(c []string) []string {
	out := append([]string(nil), c...)
	sort.Strings(out)
	return out
}

func (c *ColectorCola) Close() error {
	if c.cierre == nil {
		return nil
	}
	return c.cierre()
}

func (c *ColectorCola) Describe(ch chan<- *prometheus.Desc) {
	ch <- descProfundidad
	ch <- descAntiguedad
	ch <- descProcesados
	ch <- descFallidos
	ch <- descError
}

func (c *ColectorCola) Collect(ch chan<- prometheus.Metric) {
	for _, q := range c.colas {
		info, err := c.ins.GetQueueInfo(q)
		if err != nil {
			// Una cola que aún no recibió ningún trabajo no existe en Redis y
			// asynq responde con error. Es profundidad cero, no un fallo.
			if esColaInexistente(err) {
				info = &asynq.QueueInfo{Queue: q}
			} else {
				ch <- prometheus.MustNewConstMetric(descError, prometheus.GaugeValue, 1, q)
				continue
			}
		}
		ch <- prometheus.MustNewConstMetric(descError, prometheus.GaugeValue, 0, q)
		for estado, n := range map[string]int{
			"pending":   info.Pending,
			"active":    info.Active,
			"scheduled": info.Scheduled,
			"retry":     info.Retry,
			"archived":  info.Archived,
		} {
			ch <- prometheus.MustNewConstMetric(descProfundidad, prometheus.GaugeValue, float64(n), q, estado)
		}
		ch <- prometheus.MustNewConstMetric(descAntiguedad, prometheus.GaugeValue, info.Latency.Seconds(), q)
		ch <- prometheus.MustNewConstMetric(descProcesados, prometheus.CounterValue, float64(info.ProcessedTotal), q)
		ch <- prometheus.MustNewConstMetric(descFallidos, prometheus.CounterValue, float64(info.FailedTotal), q)
	}
}

// esColaInexistente reconoce el error de asynq para una cola que nunca recibió
// trabajos. El tipo vive en un paquete interno de asynq y no se puede
// comparar con errors.As; queda su mensaje, que es estable desde v0.20.
func esColaInexistente(err error) bool {
	return err != nil && strings.Contains(err.Error(), "does not exist")
}
