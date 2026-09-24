// Command api es el monolito modular HTTP (ADR-0001).
//
// No mantiene estado local: sesiones en Redis, datos en PostgreSQL, binarios en
// el almacén de objetos. Por eso `--scale api=N` funciona sin sesiones pegajosas.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/adapters/objectstore"
	"mooc/backend/internal/adapters/playback"
	"mooc/backend/internal/adapters/postgres"
	"mooc/backend/internal/adapters/queue"
	"mooc/backend/internal/adapters/rediscli"
	"mooc/backend/internal/adapters/sessions"
	"mooc/backend/internal/modules/admin"
	"mooc/backend/internal/modules/assessment"
	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/modules/authoring"
	"mooc/backend/internal/modules/badges"
	"mooc/backend/internal/modules/identity"
	"mooc/backend/internal/modules/learning"
	"mooc/backend/internal/modules/media"
	"mooc/backend/internal/platform/config"
	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/idempotency"
	"mooc/backend/internal/platform/jobs"
	"mooc/backend/internal/platform/logging"
	"mooc/backend/internal/platform/metrics"
	"mooc/backend/internal/platform/ratelimit"
	"mooc/backend/openapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("la api no pudo arrancar", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel, cfg.LogFormat)
	// Para que los errores registrados fuera de un handler con logger propio
	// (httpx.Fail) salgan en el mismo formato estructurado.
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- adaptadores ------------------------------------------------------
	pool, err := postgres.Open(ctx, cfg.DatabaseURL, func(c *pgxpool.Config) {
		// Cada instancia abre su propio pool: en Cloud Run el total es este
		// número por el de instancias (ADR-0015, D6).
		c.MaxConns = cfg.DBMaxConns
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	sessionRedis, err := rediscli.Open(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisSessionDB, cfg.RedisTLS)
	if err != nil {
		return err
	}
	defer sessionRedis.Close()

	// Base lógica propia: los contadores de tasa no comparten espacio de
	// claves con las sesiones ni con la cola (ADR-0004).
	limitRedis, err := rediscli.Open(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisLimitDB, cfg.RedisTLS)
	if err != nil {
		return err
	}
	defer limitRedis.Close()
	limiter := ratelimit.New(limitRedis)

	store, err := abrirAlmacen(cfg)
	if err != nil {
		return err
	}

	publisher := queue.NewPublisher(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisQueueDB)
	defer publisher.Close()

	// --- módulos ----------------------------------------------------------
	recorder := audit.NewRecorder(log)
	identityStore := postgres.NewIdentityStore(pool)
	sessionStore := sessions.New(sessionRedis)
	identitySvc := identity.NewService(
		identityStore,
		sessionStore,
		queueBridge{publisher},
		recorder,
		identity.Config{
			SessionTTL:     cfg.SessionTTL,
			VerifyTokenTTL: cfg.VerifyTokenTTL,
			PublicBaseURL:  cfg.PublicBaseURL,
		},
		log,
	)

	// --- ruteo ------------------------------------------------------------
	// El dominio calcula ETags sin conocer crypto ni HTTP.
	authoring.UsarCalculoDeETag(httpx.ETagDe)

	authoringSvc := authoring.NewService(postgres.NewAuthoringStore(pool), recorder, log)
	learningSvc := learning.NewService(postgres.NewLearningStore(pool), recorder,
		learningBridge{publisher}, learning.UmbralesPorDefecto(), log)
	badgeSvc := badges.NewService(postgres.NewBadgeStore(pool), store,
		cfg.S3Buckets.Badges, cfg.S3PublicURL(), log)
	assessmentSvc := assessment.NewService(postgres.NewAssessmentStore(pool), learningSvc, log)
	adminSvc := admin.NewService(postgres.NewAdminStore(pool), sessionStore,
		learningBridge{publisher}, recorder, log)
	mediaSvc := media.NewService(postgres.NewMediaStore(pool), almacenBridge{store},
		learningBridge{publisher}, recorder,
		media.Buckets{Originales: cfg.S3Buckets.Originals, Derivados: cfg.S3Buckets.Derived,
			Cuarentena: cfg.S3Buckets.Quarantine},
		cfg.S3PublicEndpoint, log).
		// Las credenciales de reproducción viven en la base de sesiones: son lo
		// mismo que una sesión, con menos alcance y menos vida.
		ConReproduccion(playback.New(sessionRedis)).
		ConBasePublica(cfg.PublicBaseURL).
		ConInscripciones(inscripcionesBridge{learningSvc})

	metrics.Inicializar(jobs.TypeEmailSend, jobs.TypeBadgeIssue)

	idem := idempotency.New(pool, log).Requerida()

	mux := http.NewServeMux()
	auth := identity.Authenticate(identitySvc)
	soloDocentes := identity.RequireRole(identity.RoleTeacher, identity.RoleAdmin)
	soloAdmin := identity.RequireRole(identity.RoleAdmin)

	identity.NewAPI(identitySvc).Routes(mux, auth, identity.Limitador{
		PorIP: func(r ratelimit.Regla) httpx.Middleware {
			return limiter.Middleware(r, ratelimit.PorIP, log)
		},
		PorCuenta: func(r ratelimit.Regla) httpx.Middleware {
			return limiter.Middleware(r, ratelimit.PorCampoDelCuerpo("email"), log)
		},
	})
	authoring.NewAPI(authoringSvc).Routes(mux, auth, soloDocentes, idem)
	learning.NewAPI(learningSvc, tamperAuditor{pool: pool, rec: recorder, log: log}).
		Routes(mux, auth, func(r ratelimit.Regla) httpx.Middleware {
			// Por sesión, no por IP: varios estudiantes tras el mismo NAT no
			// deben estorbarse.
			return limiter.Middleware(r, ratelimit.PorSesion, log)
		}, idem)
	badges.NewAPI(badgeSvc).Routes(mux, auth, soloAdmin, idem)
	assessment.NewAPI(assessmentSvc).Routes(mux, auth, soloDocentes, idem)
	admin.NewAPI(adminSvc).Routes(mux, auth, soloAdmin, idem)
	media.NewAPI(mediaSvc).Routes(mux, auth, soloDocentes, idem)

	health := &health{pool: pool, redis: sessionRedis, objects: store}
	mux.HandleFunc("GET /healthz", health.live)
	mux.HandleFunc("GET /readyz", health.ready)
	// Solo red interna: Compose no publica este puerto fuera.
	mux.Handle("GET /metrics", metrics.Handler())

	// La API sirve su propio contrato (RT-05, entregable §8).
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(openapi.Spec)
	})

	handler := httpx.Chain(mux,
		httpx.RequestID(),
		httpx.Recover(log),
		metrics.Middleware(),
		httpx.AccessLog(log),
	)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api escuchando", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("apagando la api")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// queueBridge adapta el publicador de asynq al puerto que declara identity.
// La composición vive en cmd/, que es el único sitio que conoce a ambos lados
// (ADR-0001).
type queueBridge struct {
	p *queue.Publisher
}

func (b queueBridge) Publish(ctx context.Context, j identity.JobRef, traceID string) error {
	return b.p.Publish(ctx, jobs.Job{
		Key: j.Key, Type: j.Type, Queue: j.Queue, Payload: j.Payload,
	}, traceID)
}

// learningBridge adapta el mismo publicador a la firma que declara learning.
// Son dos puertos distintos porque cada módulo define el suyo; el adaptador
// vive aquí, en la composición (ADR-0001).
type learningBridge struct{ p *queue.Publisher }

func (b learningBridge) Publish(ctx context.Context, key, tipo, cola string,
	payload map[string]any, traceID string) error {
	return b.p.Publish(ctx, jobs.Job{Key: key, Type: tipo, Queue: cola, Payload: payload}, traceID)
}

// almacenBridge adapta el adaptador de objetos al puerto que declara media.
// Los tipos Parte son distintos a propósito: el dominio no importa el
// adaptador (ADR-0001).
type almacenBridge struct{ s objectstore.Almacen }

func (b almacenBridge) Subir(ctx context.Context, bucket, key, ct string, datos []byte) error {
	return b.s.Subir(ctx, bucket, key, ct, datos)
}

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

// tamperAuditor registra el intento de enviar un progreso calculado por el
// cliente. Es la evidencia que exige CA-05 y que el segmento 7 debe mostrar.
type tamperAuditor struct {
	pool *pgxpool.Pool
	rec  *audit.Recorder
	log  *slog.Logger
}

func (t tamperAuditor) TamperingAttempt(r *http.Request, enrollmentID uuid.UUID, cuerpo map[string]any) {
	p, _ := httpx.PrincipalFrom(r.Context())
	t.rec.RecordBestEffort(r.Context(), t.pool, audit.Event{
		ActorID: &p.UserID, ActorRole: p.Role,
		Action: "progress.tampering_attempt", EntityType: "enrollment", EntityID: &enrollmentID,
		IP: httpx.ClientIP(r), UserAgent: r.UserAgent(),
		TraceID:  httpx.RequestIDFrom(r.Context()),
		Metadata: map[string]any{"body": cuerpo},
	})
	t.log.Warn("intento de enviar progreso calculado por el cliente",
		"enrollment_id", enrollmentID, "user_id", p.UserID, "body", cuerpo)
}

// inscripcionesBridge deja que `media` pregunte por una inscripción sin
// importar `learning`: cada módulo habla con el otro por su interfaz de
// servicio, nunca por sus tablas (ADR-0001).
//
// La comprobación de derecho la hace `learning.Content`, que ya aplica el
// guard de propiedad; aquí solo se traduce el resultado.
type inscripcionesBridge struct{ s *learning.Service }

func (b inscripcionesBridge) AssetDeRecurso(ctx context.Context, enrollmentID,
	resourceStableID uuid.UUID, a media.Actor) (uuid.UUID, error) {

	_, recursos, err := b.s.Content(ctx, enrollmentID, learning.Actor{
		UserID: a.UserID, Role: a.Role, IP: a.IP, Agent: a.Agent, Trace: a.Trace,
	})
	if err != nil {
		return uuid.Nil, traducirLearning(err)
	}

	for _, r := range recursos {
		if r.StableID != resourceStableID {
			continue
		}
		if r.AssetID == nil {
			// El recurso existe pero no tiene archivo: un bloque de texto o un
			// quiz. No es un error del sistema, es que no hay nada que
			// reproducir.
			return uuid.Nil, media.ErrNotFound
		}
		return *r.AssetID, nil
	}
	// Un recurso que no está en el árbol de ESTA inscripción no existe para
	// quien pregunta: no se distingue de uno inexistente, para no filtrar el
	// contenido de otros cursos.
	return uuid.Nil, media.ErrNotFound
}

// traducirLearning pasa los errores de un módulo a los del otro. Sin esto, la
// capa HTTP de `media` no sabría qué código devolver.
func traducirLearning(err error) error {
	switch {
	case errors.Is(err, learning.ErrProhibido):
		return media.ErrProhibido
	case errors.Is(err, learning.ErrNotFound):
		return media.ErrNotFound
	default:
		return err
	}
}

// abrirAlmacen elige la implementación del almacén de objetos.
//
// Es el único sitio donde el proveedor se nombra. `s3` cubre MinIO en Compose
// y cualquier almacén compatible; `gcs` será el adaptador nativo de Cloud
// Storage (ADR-0015, D1), que aún no existe: se escribe contra GCS real en la
// fase 3 de `arquitectura/despliegue-gcp.md`, porque su parte delicada —firmar
// URLs con la identidad de la carga, sin clave descargada— no se puede
// comprobar en local.
//
// Falla al arrancar y no a mitad de una subida: un almacén mal configurado
// tiene que notarse en el despliegue.
func abrirAlmacen(cfg config.Config) (objectstore.Almacen, error) {
	switch strings.ToLower(cfg.ObjectStore) {
	case "", "s3":
		return objectstore.Open(cfg.S3Endpoint, cfg.S3PublicEndpoint, cfg.S3AccessKey,
			cfg.S3SecretKey, cfg.S3UseSSL, cfg.S3Buckets.Originals)
	case "gcs":
		return nil, fmt.Errorf("OBJECT_STORE=gcs: el adaptador de Cloud Storage " +
			"todavía no está escrito; ver arquitectura/despliegue-gcp.md, fase 3")
	default:
		return nil, fmt.Errorf("OBJECT_STORE=%q no es una opción válida (s3, gcs)", cfg.ObjectStore)
	}
}
