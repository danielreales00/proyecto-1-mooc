# ADR-0002 — Dependencias mínimas y `net/http` de la stdlib para el ruteo

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RT-01`, `RT-05`

## Contexto

Go tiene un ecosistema amplio de frameworks HTTP (Gin, Echo, Fiber, chi) y de
acceso a datos (GORM, sqlc, ent). El enunciado exige que el dominio esté
desacoplado del framework HTTP, y el equipo ya trabajó con éxito en un proyecto
previo usando solo la stdlib.

Desde Go 1.22 el `ServeMux` de la stdlib rutea por método y por comodines de
ruta (`GET /api/v1/courses/{id}`), que era la razón histórica para traer un
router externo.

## Decisión

Se usa `net/http` de la stdlib para el ruteo y el middleware. Se admite una
dependencia externa solo cuando resuelve un problema que la stdlib no resuelve, y
cada una queda justificada aquí.

Dependencias aprobadas para la Entrega 1:

| Dependencia | Para qué | Por qué no la stdlib |
| --- | --- | --- |
| `jackc/pgx/v5` | Cliente PostgreSQL + pool | `database/sql` no expone `COPY`, tipos de PG ni el protocolo binario |
| `hibiken/asynq` | Cola sobre Redis | **Exigida por el enunciado §4** |
| `redis/go-redis/v9` | Cliente Redis (sesiones, caché, rate limit) | — |
| `minio/minio-go/v7` | Cliente S3, multipart y URLs prefirmadas | Firmar V4 a mano es innecesario y arriesgado |
| `google/uuid` | UUIDv4/v7 | — |
| `golang-jwt/jwt/v5` | Solo para tokens de verificación de correo y de URL de insignia | Las sesiones **no** usan JWT (ver ADR-0006) |
| `golang.org/x/crypto` | Argon2id | — |
| `yuin/goldmark` | Parseo y normalización de Markdown extendido | Reimplementar CommonMark no es opción |
| `microcosm-cc/bluemonday` | Sanitización del HTML renderizado | XSS es criterio de evaluación (`CE-02`) |
| `prometheus/client_golang` | Métricas | — |
| `go.opentelemetry.io/otel` (+ exporter OTLP) | Trazas | **Exigido por el enunciado §7** |

Todo lo demás requiere ADR nuevo antes de entrar al `go.mod`.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Gin / Echo / Fiber | Ergonomía, middleware listo | Acopla los handlers al framework; el enunciado pide lo contrario | El `ServeMux` 1.22+ cubre lo que necesitamos |
| chi | Ligero, compatible con `http.Handler` | Una dependencia más para poco beneficio | Marginal frente a la stdlib actual |
| GORM | CRUD rápido | Esquema generado, SQL opaco, difícil de auditar; choca con migraciones versionadas | El esquema es un activo del proyecto, no un derivado |
| sqlc | SQL versionado + tipos generados | Paso de generación en CI; el equipo no lo ha usado | Reconsiderable si el SQL a mano se vuelve pesado |

## Consecuencias

- Los handlers son `func(http.ResponseWriter, *http.Request)`: cualquiera del
  equipo los lee sin aprender un framework.
- El middleware (auth, rate limit, `Idempotency-Key`, trazas, `ETag`) se escribe
  a mano: unas 300 líneas, y bajo nuestro control.
- Escribimos SQL. Más verboso, y también más fácil de explicar en la sustentación.
- `go.mod` corto significa superficie de seguridad corta para el análisis de CI.

## Cómo se verifica

`go list -m all` en CI contra la lista aprobada; una dependencia no listada rompe
la construcción.
