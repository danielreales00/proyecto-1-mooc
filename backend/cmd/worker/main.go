// Command worker consume la cola de asynq (ADR-0004).
//
// Es el mismo código que la API, con distinta configuración de colas: escalar
// medios es `--scale worker-media=N`, no subir la concurrencia.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"mooc/backend/internal/adapters/mailer"
	"mooc/backend/internal/adapters/postgres"
	"mooc/backend/internal/adapters/queue"
	"mooc/backend/internal/modules/identity"
	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/logging"
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
	emails := identity.NewEmailWorker(
		mailer.New(cfg.SMTPAddr, cfg.MailFrom), cfg.PublicBaseURL, log)

	mux := asynq.NewServeMux()
	mux.HandleFunc(jobs.TypeEmailSend, runner.Wrap(jobs.TypeEmailSend, emails.Handle))

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
	srv := &http.Server{Addr: ":8081", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warn("servidor de salud del worker detenido", "error", err)
		}
	}()
	return srv
}

// asynqLogger traduce los logs de asynq al logger estructurado del proyecto.
type asynqLogger struct{ log *slog.Logger }

func (l asynqLogger) Debug(args ...any) { l.log.Debug("asynq", "msg", args) }
func (l asynqLogger) Info(args ...any)  { l.log.Info("asynq", "msg", args) }
func (l asynqLogger) Warn(args ...any)  { l.log.Warn("asynq", "msg", args) }
func (l asynqLogger) Error(args ...any) { l.log.Error("asynq", "msg", args) }
func (l asynqLogger) Fatal(args ...any) { l.log.Error("asynq fatal", "msg", args) }
