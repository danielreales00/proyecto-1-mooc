// Package funcion es el punto de entrada de la Cloud Run function que emite las
// insignias (ADR-0021). Vive en la raíz del módulo porque el buildpack de Go
// compila el paquete raíz del código que se sube, y desde aquí se alcanzan los
// paquetes internal del backend.
//
// La función reutiliza el mismo ejecutor y el mismo servicio que el worker:
// cambia la entrada, un CloudEvent de Pub/Sub entregado por Eventarc, no el
// trabajo.
package funcion

import (
	"context"
	"log/slog"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc.local/backend/internal/adapters/almacenes"
	"mooc.local/backend/internal/adapters/postgres"
	"mooc.local/backend/internal/adapters/queue"
	"mooc.local/backend/internal/modules/badges"
	"mooc.local/backend/internal/platform/config"
	"mooc.local/backend/internal/platform/jobs"
	"mooc.local/backend/internal/platform/logging"
)

// Nombre de la función en el despliegue: el `entry_point` de Terraform.
const Nombre = "EmitirInsignia"

func init() {
	functions.CloudEvent(Nombre, emitirInsignia)
}

var (
	mu     sync.Mutex
	evento *queue.Evento
)

// emitirInsignia devuelve nil para que Eventarc confirme el evento y un error
// para que lo reintente.
func emitirInsignia(ctx context.Context, ev event.Event) error {
	e, err := preparar(ctx)
	if err != nil {
		return err
	}
	return e.Atender(ctx, ev.Data())
}

// preparar abre la base y el almacén en la primera entrega de la instancia.
// Si falla, la entrega se reintenta y la siguiente vuelve a intentarlo, en vez
// de dejar la instancia rota para siempre.
func preparar(ctx context.Context) (*queue.Evento, error) {
	mu.Lock()
	defer mu.Unlock()
	if evento != nil {
		return evento, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	log := logging.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(log)

	pool, err := postgres.Open(context.WithoutCancel(ctx), cfg.DatabaseURL, func(c *pgxpool.Config) {
		// Una entrega a la vez por instancia: un pool corto basta y no
		// compite con las conexiones de la API y los workers.
		c.MaxConns = cfg.DBMaxConns
	})
	if err != nil {
		return nil, err
	}
	almacen, err := almacenes.Abrir(cfg)
	if err != nil {
		pool.Close()
		return nil, err
	}
	svc := badges.NewService(postgres.NewBadgeStore(pool), almacen,
		cfg.S3Buckets.Badges, cfg.S3PublicURL(), log)

	evento = queue.NewEvento(queue.NewRunner(pool, log), jobs.TypeBadgeIssue,
		svc.Handle, cfg.PubSubMaxEntregas, log)
	return evento, nil
}
