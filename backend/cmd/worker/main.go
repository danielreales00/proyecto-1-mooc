// Command worker consume la cola de asynq (ADR-0004).
//
// Es el mismo código que la API, con distinta configuración de colas: escalar
// medios es `--scale worker-media=N`, no subir la concurrencia.
package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"mooc/backend/internal/adapters/mailer"
	"mooc/backend/internal/adapters/objectstore"
	"mooc/backend/internal/adapters/postgres"
	"mooc/backend/internal/adapters/queue"
	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/modules/badges"
	"mooc/backend/internal/modules/identity"
	"mooc/backend/internal/modules/media"
	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/logging"
	"mooc/backend/internal/platform/metrics"
)

func main() {
	if err := run(); err != nil {
		slog.Error("el worker no pudo arrancar", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel)
	// Para que los errores registrados fuera de un handler con logger propio
	// (httpx.Fail) salgan en el mismo formato estructurado.
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	runner := queue.NewRunner(pool, log)

	// El reaper recupera los trabajos que quedaron registrados en PostgreSQL
	// pero nunca llegaron a la cola, y los que un worker dejó a medias
	// (ADR-0004). Sin él, la promesa "perder Redis no pierde trabajo" solo se
	// cumple a medias.
	publisher := queue.NewPublisher(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisQueueDB)
	defer publisher.Close()
	queue.NewReaper(pool, publisher, log).Start(ctx, 30*time.Second)
	emails := identity.NewEmailWorker(
		mailer.New(cfg.SMTPAddr, cfg.MailFrom), cfg.PublicBaseURL, log)

	badgeSvc := badges.NewService(postgres.NewBadgeStore(pool), log)

	almacen, err := objectstore.Open(cfg.S3Endpoint, cfg.S3PublicEndpoint, cfg.S3AccessKey, cfg.S3SecretKey,
		cfg.S3UseSSL, cfg.S3Buckets.Originals)
	if err != nil {
		return err
	}
	mediaSvc := media.NewService(postgres.NewMediaStore(pool), almacenBridge{almacen},
		publicadorBridge{publisher}, audit.NewRecorder(log),
		media.Buckets{Originales: cfg.S3Buckets.Originals, Cuarentena: cfg.S3Buckets.Quarantine},
		cfg.S3PublicEndpoint, log)

	// Los contadores deben existir en cero antes del primer suceso, o
	// increase() no verá el salto y la alerta de DLQ nunca disparará.
	metrics.Inicializar(jobs.TypeEmailSend, jobs.TypeBadgeIssue, jobs.TypeMediaProbe)

	mux := asynq.NewServeMux()
	mux.HandleFunc(jobs.TypeEmailSend, runner.Wrap(jobs.TypeEmailSend, emails.Handle))
	mux.HandleFunc(jobs.TypeBadgeIssue, runner.Wrap(jobs.TypeBadgeIssue, badgeSvc.Handle))
	mux.HandleFunc(jobs.TypeMediaProbe, runner.Wrap(jobs.TypeMediaProbe, mediaSvc.Verificar))

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisQueueDB},
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Queues:      queue.ParseQueues(cfg.WorkerQueues),
			// El backoff por defecto de asynq es exponencial con jitter, que es
			// lo que pide RNF-06; el tope de 3 intentos lo fija cada tarea al
			// encolarse (queue.Publish).
			ErrorHandler: runner.OnRetriesExhausted(),
			Logger:       asynqLogger{log},
		},
	)

	// El worker no expone API: solo salud, para el healthcheck de Compose.
	healthSrv := startHealth(log)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthSrv.Shutdown(shutdownCtx)
	}()

	errCh := make(chan error, 1)
	go func() {
		log.Info("worker escuchando", "queues", cfg.WorkerQueues, "concurrency", cfg.WorkerConcurrency)
		if err := srv.Run(mux); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("apagando el worker")
		srv.Shutdown()
		return nil
	}
}

func startHealth(log *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	// El worker también expone sus métricas: jobs_dead_letter_total sale de
	// aquí, y es la que dispara la alerta del §6.
	mux.Handle("GET /metrics", metrics.Handler())
	srv := &http.Server{Addr: ":8081", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warn("servidor de salud del worker detenido", "error", err)
		}
	}()
	return srv
}

// almacenBridge y publicadorBridge adaptan los adaptadores a los puertos que
// declara media. Viven aquí porque la composición es quien conoce ambos lados.
type almacenBridge struct{ s *objectstore.Store }

func (b almacenBridge) CrearMultipart(ctx context.Context, bucket, key, ct string) (string, error) {
	return b.s.CrearMultipart(ctx, bucket, key, ct)
}

func (b almacenBridge) PresignPart(ctx context.Context, bucket, key, uploadID string,
	parte int, ttl time.Duration) (string, error) {
	return b.s.PresignPart(ctx, bucket, key, uploadID, parte, ttl)
}

func (b almacenBridge) PartesSubidas(ctx context.Context, bucket, key, uploadID string) ([]media.Parte, error) {
	ps, err := b.s.PartesSubidas(ctx, bucket, key, uploadID)
	if err != nil {
		return nil, err
	}
	out := make([]media.Parte, 0, len(ps))
	for _, p := range ps {
		out = append(out, media.Parte{Numero: p.Numero, ETag: p.ETag, Bytes: p.Bytes})
	}
	return out, nil
}

func (b almacenBridge) CompletarMultipart(ctx context.Context, bucket, key, uploadID string,
	partes []media.Parte) error {
	out := make([]objectstore.Parte, 0, len(partes))
	for _, p := range partes {
		out = append(out, objectstore.Parte{Numero: p.Numero, ETag: p.ETag, Bytes: p.Bytes})
	}
	return b.s.CompletarMultipart(ctx, bucket, key, uploadID, out)
}

func (b almacenBridge) AbortarMultipart(ctx context.Context, bucket, key, uploadID string) error {
	return b.s.AbortarMultipart(ctx, bucket, key, uploadID)
}

func (b almacenBridge) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	return b.s.PresignGet(ctx, bucket, key, ttl)
}

func (b almacenBridge) Abrir(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	return b.s.Abrir(ctx, bucket, key)
}

func (b almacenBridge) Info(ctx context.Context, bucket, key string) (int64, error) {
	return b.s.Info(ctx, bucket, key)
}

func (b almacenBridge) Mover(ctx context.Context, ob, ok, db, dk string) error {
	return b.s.Mover(ctx, ob, ok, db, dk)
}

type publicadorBridge struct{ p *queue.Publisher }

func (b publicadorBridge) Publish(ctx context.Context, key, tipo, cola string,
	payload map[string]any, traceID string) error {
	return b.p.Publish(ctx, jobs.Job{Key: key, Type: tipo, Queue: cola, Payload: payload}, traceID)
}

// asynqLogger traduce los logs de asynq al logger estructurado del proyecto.
type asynqLogger struct{ log *slog.Logger }

func (l asynqLogger) Debug(args ...any) { l.log.Debug("asynq", "msg", args) }
func (l asynqLogger) Info(args ...any)  { l.log.Info("asynq", "msg", args) }
func (l asynqLogger) Warn(args ...any)  { l.log.Warn("asynq", "msg", args) }
func (l asynqLogger) Error(args ...any) { l.log.Error("asynq", "msg", args) }
func (l asynqLogger) Fatal(args ...any) { l.log.Error("asynq fatal", "msg", args) }
