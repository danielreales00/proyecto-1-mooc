# ADR-0008 — Idempotencia en la API y en los workers

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RT-05`, `CA-03`, `CA-04`, `CA-07`, `RNF-06`

## Contexto

Tres condiciones de aceptación distintas exigen idempotencia y hablan de cosas
diferentes: el envío de un quiz (`CA-04`), la entrega duplicada de un trabajo
(`CA-03`) y la emisión de insignia (`CA-07`). Además `RT-05` obliga a soportar la
cabecera `Idempotency-Key` en la API. Conviene un único criterio para no
inventar tres mecanismos.

## Decisión

Tres mecanismos, cada uno en su capa, y ninguno basado en "consultar antes de
escribir" — que bajo concurrencia no garantiza nada.

### 1. API: cabecera `Idempotency-Key`

Obligatoria en `POST`/`PATCH` que crean o mutan de forma no repetible:
`:submit` de un intento, inscripción, publicación, inicio de carga, reencolado.

Tabla `platform.idempotency_keys`:

| Columna | |
| --- | --- |
| `key` | valor de la cabecera |
| `user_id` | el dueño; una clave de otro usuario no colisiona |
| `endpoint` | método + patrón de ruta |
| `request_hash` | SHA-256 del cuerpo canonicalizado |
| `status` | `in_progress` \| `completed` |
| `response_status`, `response_body` | respuesta capturada |

`UNIQUE (user_id, endpoint, key)`.

Flujo: el middleware intenta `INSERT ... ON CONFLICT DO NOTHING`.

- Inserta → ejecuta el handler y guarda la respuesta.
- Conflicto con `completed` y el mismo `request_hash` → **repite la respuesta
  guardada** con `Idempotency-Replayed: true`.
- Conflicto con distinto `request_hash` → `422 idempotency_key_reuse`.
- Conflicto con `in_progress` → `409 request_in_flight`.

Retención: 24 h.

### 2. Workers: `job_key` y reclamo atómico

Cada trabajo lleva una `job_key` **determinista**, derivada de la entidad y de la
intención, no del momento:

```
media.transcode_hls:{asset_id}:{ladder_version}
badge.issue:{enrollment_id}
email.send:{template}:{user_id}:{token_id}
```

Antes de trabajar, el worker reclama la ejecución:

```sql
INSERT INTO platform.job_runs (job_key, type, status, attempt, heartbeat_at)
VALUES ($1, $2, 'running', 1, now())
ON CONFLICT (job_key) DO UPDATE
   SET status = 'running', attempt = job_runs.attempt + 1, heartbeat_at = now()
 WHERE job_runs.status IN ('queued','failed')
RETURNING id;
```

Sin fila devuelta, otro worker lo tiene o ya terminó: el trabajo se descarta y se
cuenta en `jobs_duplicate_skipped_total`. Es lo que demuestra "doble entrega
idempotente" en SEG-4.

Las salidas también son idempotentes por construcción: las claves de objeto son
deterministas (ADR-0005), así que reescribir produce el mismo resultado.

### 3. Dominio: restricciones únicas

Donde la regla es "esto existe una sola vez", la garantiza la base:

| Regla | Restricción |
| --- | --- |
| Una insignia por inscripción aprobada (`CA-07`) | `UNIQUE (enrollment_id)` en `badges.badges` |
| Un intento en curso por quiz y estudiante | índice parcial `UNIQUE (quiz_stable_id, user_id) WHERE status='in_progress'` |
| Una inscripción activa por curso y estudiante | índice parcial `UNIQUE (user_id, course_id) WHERE status='active'` |
| Un número de versión por curso | `UNIQUE (course_id, version_number)` |
| Un `stable_id` por versión | `UNIQUE (course_version_id, stable_id)` |

La emisión de insignia queda entonces protegida dos veces: por `job_key` y por la
restricción única. Que la segunda exista es lo que hace que la primera no tenga
que ser perfecta.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Consultar y luego insertar | Fácil de leer | Condición de carrera clásica; falla justo bajo la carga que se va a probar | No es correcto |
| Bloqueos distribuidos en Redis | Genérico | Redis no es la fuente de verdad; un lock perdido rompe la garantía | ADR-0004 pone la verdad en PostgreSQL |
| `SELECT ... FOR UPDATE` en todo | Correcto | Contención y transacciones largas durante la transcodificación | Se reserva para secciones cortas |
| Idempotencia solo en workers | Menos código | No cubre el `submit` del quiz, que exige `CA-04` | Requisito explícito |

## Consecuencias

- Reintentar es seguro en todas partes, que es lo que hace realista el backoff.
- El middleware de `Idempotency-Key` guarda cuerpos de respuesta: se limita a
  256 KB y se excluyen respuestas de streaming.
- Las claves deterministas obligan a pensar qué identifica a un trabajo antes de
  escribirlo; en la práctica ha sido un buen filtro de diseño.

## Cómo se verifica

- Enviar dos veces el mismo `submit` con igual clave → misma nota, mismo cuerpo,
  cabecera `Idempotency-Replayed: true`, un solo intento en la base.
- Mismo `submit` con clave repetida y cuerpo distinto → `422`.
- Encolar `badge.issue` dos veces para la misma inscripción → una fila en
  `badges.badges`.
