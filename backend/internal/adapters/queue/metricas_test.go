package queue

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type inspectorFalso map[string]*asynq.QueueInfo

func (f inspectorFalso) GetQueueInfo(q string) (*asynq.QueueInfo, error) {
	if q == "rota" {
		return nil, errors.New("dial tcp: connection refused")
	}
	if info, ok := f[q]; ok {
		return info, nil
	}
	return nil, errors.New(`rdb.CurrentStats: NOT_FOUND: queue "` + q + `" does not exist`)
}

func TestColectorCola(t *testing.T) {
	c := &ColectorCola{
		ins: inspectorFalso{"bulk": {
			Queue: "bulk", Pending: 7, Active: 1, Retry: 2, Archived: 1,
			Latency: 90 * time.Second, ProcessedTotal: 40, FailedTotal: 3,
		}},
		colas: []string{"bulk", "critical", "rota"},
	}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)

	esperado := `
# HELP queue_depth Trabajos en la cola por estado. archived son los muertos (DLQ).
# TYPE queue_depth gauge
queue_depth{queue="bulk",state="active"} 1
queue_depth{queue="bulk",state="archived"} 1
queue_depth{queue="bulk",state="pending"} 7
queue_depth{queue="bulk",state="retry"} 2
queue_depth{queue="bulk",state="scheduled"} 0
queue_depth{queue="critical",state="active"} 0
queue_depth{queue="critical",state="archived"} 0
queue_depth{queue="critical",state="pending"} 0
queue_depth{queue="critical",state="retry"} 0
queue_depth{queue="critical",state="scheduled"} 0
# HELP queue_inspect_errors 1 si la última lectura de la cola falló: las demás series de esa cola faltan.
# TYPE queue_inspect_errors gauge
queue_inspect_errors{queue="bulk"} 0
queue_inspect_errors{queue="critical"} 0
queue_inspect_errors{queue="rota"} 1
# HELP queue_oldest_pending_age_seconds Antigüedad del trabajo pendiente más viejo: cuánto lleva esperando.
# TYPE queue_oldest_pending_age_seconds gauge
queue_oldest_pending_age_seconds{queue="bulk"} 90
queue_oldest_pending_age_seconds{queue="critical"} 0
# HELP queue_processed_total Trabajos procesados por la cola desde que existe, con éxito o no. rate() da la tasa.
# TYPE queue_processed_total counter
queue_processed_total{queue="bulk"} 40
queue_processed_total{queue="critical"} 0
# HELP queue_failed_total Ejecuciones fallidas en la cola desde que existe, incluidas las que se reintentan.
# TYPE queue_failed_total counter
queue_failed_total{queue="bulk"} 3
queue_failed_total{queue="critical"} 0
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(esperado),
		"queue_depth", "queue_inspect_errors", "queue_oldest_pending_age_seconds",
		"queue_processed_total", "queue_failed_total"); err != nil {
		t.Fatal(err)
	}
}

func TestColasDe(t *testing.T) {
	got := strings.Join(ColasDe("critical=6,default=3"), ",")
	if got != "critical,default" {
		t.Fatalf("ColasDe = %q", got)
	}
}
