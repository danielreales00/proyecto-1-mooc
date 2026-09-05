# ADR-0004 — Cola asíncrona con Redis y asynq

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RF-06`, `CA-02`, `CA-03`, `RNF-06`, `RT-03`

## Contexto

El enunciado nombra explícitamente **asynq** sobre Redis (§4). No hay decisión de
tecnología que tomar; sí hay que decidir la topología de colas, la política de
reintentos, cómo se implementa la DLQ y qué se hace con la advertencia del propio
enunciado (§11) sobre pérdida de trabajos en Redis.

## Decisión

**Colas por prioridad** (pesos de asynq):

| Cola | Peso | Trabajos |
| --- | --- | --- |
| `critical` | 6 | `badge.issue`, `email.send` |
| `default` | 3 | `media.scan`, `media.probe`, `progress.recompute` |
| `bulk` | 1 | `media.transcode_hls`, `media.poster`, `doc.convert_pdf` |

Sin esto, transcodificar un video de 40 minutos bloquea el envío de un correo de
verificación.

**Política de reintentos:** `MaxRetry = 3`, backoff exponencial con jitter
(≈ 30 s, 2 min, 8 min). Al agotarse, asynq mueve la tarea a su lista de
archivadas; un `ErrorHandler` global escribe la fila en `media.dead_letter`,
incrementa la métrica `jobs_dead_letter_total{type}` y dispara la alerta
(`CA-03`).

**Reencolado desde la DLQ:** `POST /api/v1/admin/jobs/{id}:requeue` reencola
**con la misma clave de idempotencia** que tenía el trabajo original, lo que
demuestra el requisito de "reencolado con la misma clave" del segmento SEG-4.

**Idempotencia:** ver ADR-0008. En resumen, cada trabajo lleva `job_key` y la
tabla `job_runs` tiene índice único sobre ella; el worker reclama la ejecución
con un `INSERT ... ON CONFLICT DO NOTHING` antes de hacer trabajo real.

**Durabilidad frente a Redis (recomendación §11 del enunciado):**
PostgreSQL es la fuente de verdad, no Redis.

1. Toda tarea encolada deja una fila `job_runs` en estado `queued` **en la misma
   transacción** que el cambio de dominio que la origina (patrón *outbox*); un
   despachador la publica en asynq después del commit.
2. El worker escribe `heartbeat_at` cada 30 s mientras procesa.
3. Un worker periódico (`jobs.reaper`, cada minuto) reencola las filas `running`
   sin heartbeat por más de 5 minutos y las `queued` que nunca llegaron a asynq.

Así, si Redis pierde datos, ningún trabajo se pierde definitivamente.

**Redis:** una sola instancia con `appendonly yes`. Bases lógicas separadas:
`0` cola, `1` sesiones, `2` caché, `3` rate limiting. No se comparte espacio de
claves entre usos.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| RabbitMQ | DLQ nativa, enrutamiento rico; el equipo ya lo usó | Un servicio más; contradice el enunciado | El enunciado dice asynq |
| `LISTEN/NOTIFY` de PostgreSQL | Sin infraestructura extra | Sin reintentos, sin visibilidad, sin programación diferida | Habría que reescribir asynq |
| Solo asynq, sin *outbox* | Menos código | Un fallo de Redis pierde trabajos; el enunciado advierte de esto | §11 lo señala explícitamente |
| `MaxRetry` alto | Más resiliencia | El enunciado fija tres reintentos antes de DLQ | Es condición de aceptación |

## Consecuencias

- El patrón *outbox* añade una escritura por trabajo encolado; a cambio, la
  tolerancia a fallos es demostrable, que es justo lo que se evalúa.
- La demo de SEG-4 sale sola: matar Redis y ver que el `reaper` recupera.
- `job_runs` crece; se poda a 90 días con un job programado.

## Cómo se verifica

- Entregar el mismo job dos veces → una sola salida y una sola fila en `job_runs`.
- Inyectar fallo con `FAIL_TRANSCODE=1` → tres intentos con backoff creciente en
  los logs, fila en `media.dead_letter`, alerta en Prometheus.
- Reencolar desde la DLQ → misma `job_key` en los logs.
- `docker compose kill redis` durante un job → el `reaper` lo reencola.
