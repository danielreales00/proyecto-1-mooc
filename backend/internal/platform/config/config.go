// Package config carga la configuración desde el entorno.
//
// Regla del ADR-0010: nada de ficheros de configuración por entorno y ningún
// valor por defecto de producción en el código. Un despliegue distinto es un
// conjunto de variables distinto.
package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string
	LogLevel string
	// LogFormat: "json" para Compose, "gcp" para Cloud Logging, que espera
	// `severity` y `message` en vez de `level` y `msg` (ADR-0010).
	LogFormat string

	HTTPAddr      string
	PublicBaseURL string

	DatabaseURL string
	// DBMaxConns acota el pool de CADA instancia. En Cloud Run se multiplica
	// por el número de instancias y es fácil agotar Cloud SQL, así que se
	// ajusta por entorno en vez de estar fijo en el código (ADR-0015, D6).
	DBMaxConns int32

	RedisAddr     string
	RedisPassword string
	// RedisTLS lo exige Memorystore cuando se activa el cifrado en tránsito.
	RedisTLS       bool
	RedisQueueDB   int
	RedisSessionDB int
	RedisCacheDB   int
	RedisLimitDB   int

	// ObjectStore elige el adaptador: "s3" (MinIO y compatibles) o "gcs".
	// Es la única variable que nombra al proveedor (ADR-0015, D1).
	ObjectStore string

	S3Endpoint string
	// S3PublicEndpoint es el que se usa para FIRMAR. Dentro de Compose la API
	// habla con `minio:9000`, pero una URL firmada la abre el navegador o
	// Postman desde fuera, donde ese nombre no resuelve. En GCP pasa lo mismo:
	// se firma contra el endpoint público, no contra el interno.
	S3PublicEndpoint string
	S3AccessKey      string
	S3SecretKey      string
	S3UseSSL         bool
	S3Buckets        Buckets

	SMTPAddr string
	MailFrom string

	// ClamAVAddr es el clamd al que habla el worker por INSTREAM (ADR-0011).
	ClamAVAddr string

	SessionTTL        time.Duration
	SessionMaxTTL     time.Duration
	VerifyTokenTTL    time.Duration
	ResetTokenTTL     time.Duration
	WorkerConcurrency int
	WorkerQueues      string
}

type Buckets struct {
	Originals  string
	Derived    string
	Badges     string
	Quarantine string
}

// Load lee el entorno. Devuelve error si falta algo obligatorio, en lugar de
// arrancar con un valor por defecto que nadie eligió.
func Load() (Config, error) {
	var missing []string
	req := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	c := Config{
		Env:       opt("APP_ENV", "development"),
		LogLevel:  opt("LOG_LEVEL", "info"),
		LogFormat: opt("LOG_FORMAT", "json"),

		// Cloud Run inyecta PORT y espera que el proceso escuche ahí. Manda
		// sobre HTTP_ADDR para que la misma imagen sirva en Compose y en Cloud
		// Run sin tocar nada (ADR-0010, regla 4).
		HTTPAddr:      direccionHTTP(),
		PublicBaseURL: opt("PUBLIC_BASE_URL", "http://localhost:8080"),

		DatabaseURL: req("DATABASE_URL"),
		DBMaxConns:  optInt32("DB_MAX_CONNS", 10),

		RedisAddr:      req("REDIS_ADDR"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		RedisTLS:       optBool("REDIS_TLS", false),
		RedisQueueDB:   optInt("REDIS_QUEUE_DB", 0),
		RedisSessionDB: optInt("REDIS_SESSION_DB", 1),
		RedisCacheDB:   optInt("REDIS_CACHE_DB", 2),
		RedisLimitDB:   optInt("REDIS_LIMIT_DB", 3),

		ObjectStore:      opt("OBJECT_STORE", "s3"),
		S3Endpoint:       req("S3_ENDPOINT"),
		S3PublicEndpoint: opt("S3_PUBLIC_ENDPOINT", os.Getenv("S3_ENDPOINT")),
		S3AccessKey:      req("S3_ACCESS_KEY"),
		S3SecretKey:      req("S3_SECRET_KEY"),
		S3UseSSL:         optBool("S3_USE_SSL", false),
		S3Buckets: Buckets{
			Originals:  opt("S3_BUCKET_ORIGINALS", "mooc-originals"),
			Derived:    opt("S3_BUCKET_DERIVED", "mooc-derived"),
			Badges:     opt("S3_BUCKET_BADGES", "mooc-badges"),
			Quarantine: opt("S3_BUCKET_QUARANTINE", "mooc-quarantine"),
		},

		SMTPAddr:   req("SMTP_ADDR"),
		MailFrom:   opt("MAIL_FROM", "no-reply@mooc.local"),
		ClamAVAddr: opt("CLAMAV_ADDR", "clamav:3310"),

		SessionTTL:        optDuration("SESSION_TTL", 12*time.Hour),
		SessionMaxTTL:     optDuration("SESSION_MAX_TTL", 720*time.Hour),
		VerifyTokenTTL:    optDuration("VERIFY_TOKEN_TTL", 24*time.Hour),
		ResetTokenTTL:     optDuration("RESET_TOKEN_TTL", time.Hour),
		WorkerConcurrency: optInt("WORKER_CONCURRENCY", 20),
		WorkerQueues:      opt("WORKER_QUEUES", "critical=6,default=3,bulk=1"),
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("faltan variables de entorno: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

func opt(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func optInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// S3PublicURL es el origen del almacén con esquema, para construir enlaces que
// se publican. S3PublicEndpoint va sin él porque el cliente de MinIO lo exige
// como host:puerto.
func (c Config) S3PublicURL() string {
	if c.S3PublicEndpoint == "" {
		return ""
	}
	if strings.HasPrefix(c.S3PublicEndpoint, "http://") ||
		strings.HasPrefix(c.S3PublicEndpoint, "https://") {
		return c.S3PublicEndpoint
	}
	if c.S3UseSSL {
		return "https://" + c.S3PublicEndpoint
	}
	return "http://" + c.S3PublicEndpoint
}

func optBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func optDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// direccionHTTP resuelve dónde escucha el servidor.
//
// Cloud Run inyecta `PORT` y considera fallido el arranque si el proceso no
// escucha ahí. En Compose no existe esa variable y manda `HTTP_ADDR`. El orden
// importa: si algún día ambas están puestas, la del entorno de ejecución gana,
// porque es la única que no se puede desobedecer.
func direccionHTTP() string {
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return opt("HTTP_ADDR", ":8080")
}

// optInt32 lee un entero acotado. El pool de pgx se declara en int32, y un
// valor absurdo —negativo, o mayor de lo que cabe— vale menos que el de por
// defecto: se descarta en vez de convertirlo a cualquier cosa.
func optInt32(key string, porDefecto int32) int32 {
	v := optInt(key, int(porDefecto))
	if v < 1 || v > math.MaxInt32 {
		return porDefecto
	}
	return int32(v)
}
