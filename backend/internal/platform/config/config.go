// Package config carga la configuración desde el entorno.
//
// Regla del ADR-0010: nada de ficheros de configuración por entorno y ningún
// valor por defecto de producción en el código. Un despliegue distinto es un
// conjunto de variables distinto.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string
	LogLevel string

	HTTPAddr      string
	PublicBaseURL string

	DatabaseURL string

	RedisAddr      string
	RedisPassword  string
	RedisQueueDB   int
	RedisSessionDB int
	RedisCacheDB   int
	RedisLimitDB   int

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
	S3Buckets   Buckets

	SMTPAddr string
	MailFrom string

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
		Env:      opt("APP_ENV", "development"),
		LogLevel: opt("LOG_LEVEL", "info"),

		HTTPAddr:      opt("HTTP_ADDR", ":8080"),
		PublicBaseURL: opt("PUBLIC_BASE_URL", "http://localhost:8080"),

		DatabaseURL: req("DATABASE_URL"),

		RedisAddr:      req("REDIS_ADDR"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		RedisQueueDB:   optInt("REDIS_QUEUE_DB", 0),
		RedisSessionDB: optInt("REDIS_SESSION_DB", 1),
		RedisCacheDB:   optInt("REDIS_CACHE_DB", 2),
		RedisLimitDB:   optInt("REDIS_LIMIT_DB", 3),

		S3Endpoint:  req("S3_ENDPOINT"),
		S3AccessKey: req("S3_ACCESS_KEY"),
		S3SecretKey: req("S3_SECRET_KEY"),
		S3UseSSL:    optBool("S3_USE_SSL", false),
		S3Buckets: Buckets{
			Originals:  opt("S3_BUCKET_ORIGINALS", "mooc-originals"),
			Derived:    opt("S3_BUCKET_DERIVED", "mooc-derived"),
			Badges:     opt("S3_BUCKET_BADGES", "mooc-badges"),
			Quarantine: opt("S3_BUCKET_QUARANTINE", "mooc-quarantine"),
		},

		SMTPAddr: req("SMTP_ADDR"),
		MailFrom: opt("MAIL_FROM", "no-reply@mooc.local"),

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
