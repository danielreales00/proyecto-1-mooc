// Package transporte abre el publicador de trabajos que pide la configuración:
// asynq sobre Redis o Cloud Pub/Sub (ADR-0020). Lo usan la API, los workers y
// el reaper, que no necesitan saber cuál corre.
package transporte

import (
	"context"
	"fmt"
	"log/slog"

	"mooc/backend/internal/adapters/pubsub"
	"mooc/backend/internal/adapters/queue"
	"mooc/backend/internal/platform/config"
)

func Abrir(ctx context.Context, cfg config.Config, log *slog.Logger) (queue.Publicador, error) {
	switch cfg.QueueBackend {
	case "asynq":
		return queue.NewPublisher(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisQueueDB, cfg.RedisTLS), nil
	case "pubsub":
		return pubsub.Open(ctx, cfg.PubSubProject, cfg.PubSubTopic, log)
	default:
		return nil, fmt.Errorf("QUEUE_BACKEND desconocido: %q (asynq o pubsub)", cfg.QueueBackend)
	}
}
