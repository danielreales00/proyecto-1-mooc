// Package pubsub publica los trabajos en un topic de Cloud Pub/Sub (ADR-0020).
//
// El trabajo ya está registrado en platform.job_runs cuando llega aquí: este
// adaptador solo lo entrega al topic. La suscripción push lo lleva al servicio
// de workers, que lo ejecuta con queue.Push.
package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	ps "cloud.google.com/go/pubsub/v2"

	"mooc/backend/internal/adapters/queue"
	"mooc/backend/internal/platform/jobs"
)

type Publisher struct {
	cliente *ps.Client
	topic   *ps.Publisher
	log     *slog.Logger
}

var _ queue.Publicador = (*Publisher)(nil)

// Open se autentica con la identidad del entorno: la cuenta de servicio del
// servicio de Cloud Run, o el emulador si PUBSUB_EMULATOR_HOST está definida.
func Open(ctx context.Context, proyecto, topic string, log *slog.Logger) (*Publisher, error) {
	cliente, err := ps.NewClient(ctx, proyecto)
	if err != nil {
		return nil, fmt.Errorf("cliente de Pub/Sub: %w", err)
	}
	t := cliente.Publisher(topic)
	// Sin lotes de espera: la API publica después del commit y responde, y un
	// trabajo no debe quedarse esperando a que se llene un lote.
	t.PublishSettings.DelayThreshold = 0
	t.PublishSettings.CountThreshold = 1
	return &Publisher{cliente: cliente, topic: t, log: log}, nil
}

// Publish espera la confirmación de Pub/Sub, no la ejecución del trabajo. Si
// falla, la fila sigue en 'queued' y el reaper la vuelve a publicar.
func (p *Publisher) Publish(ctx context.Context, j jobs.Job, traceID string) error {
	cuerpo, err := json.Marshal(queue.Payload{JobKey: j.Key, TraceID: traceID, Data: j.Payload})
	if err != nil {
		return fmt.Errorf("serializar el trabajo %s: %w", j.Key, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res := p.topic.Publish(ctx, &ps.Message{
		Data: cuerpo,
		// El tipo decide el manejador del worker. job_key y cola van para
		// filtrar los registros y el topic de mensajes muertos.
		Attributes: map[string]string{"tipo": j.Type, "cola": j.Queue, "job_key": j.Key},
	})
	id, err := res.Get(ctx)
	if err != nil {
		return fmt.Errorf("publicar %s: %w", j.Key, err)
	}
	// El primer eslabón para seguir una tarea en los registros: la misma
	// job_key y trace_id aparecen después en el worker.
	p.log.Info("trabajo publicado", "job_key", j.Key, "type", j.Type,
		"trace_id", traceID, "message_id", id)
	return nil
}

// Republicar siempre publica: Pub/Sub no deduplica por clave. Una copia que
// llegue después de terminado el trabajo se confirma sin repetirlo
// (queue.YaTerminado).
func (p *Publisher) Republicar(ctx context.Context, j jobs.Job, traceID string) (bool, error) {
	return true, p.Publish(ctx, j, traceID)
}

func (p *Publisher) Close() error {
	p.topic.Stop()
	return p.cliente.Close()
}
