# Objetivos de servicio

El enunciado exige "objetivos de latencia y disponibilidad" documentados (§9) y
medidos bajo carga (SEG-9), pero no fija los números. Estos son los nuestros.
Están pendientes de confirmación con el profesor (`../preguntas-profesor.md`).

## Escala de referencia (enunciado §1)

| Magnitud | Valor |
| --- | --- |
| Usuarios registrados | 50.000 |
| Usuarios concurrentes | 2.000 |
| Cursos publicados estimados | 200 |
| Recursos por curso | ~40 |
| Relación lectura/escritura | ~90/10 |

Con 2.000 concurrentes y una acción cada ~20 s de media: **~100 req/s** de
régimen, con picos a 300 req/s.

## Latencia (p95, medida en el proxy)

| Clase de endpoint | Objetivo p95 | Ejemplos |
| --- | --- | --- |
| Lectura cacheable | ≤ 150 ms | `GET /catalog/courses`, ficha pública |
| Lectura autenticada | ≤ 250 ms | `GET /enrollments/{id}/content` |
| Escritura simple | ≤ 400 ms | login, autosave, guardado parcial de quiz |
| Escritura compleja | ≤ 800 ms | `:publish`, `:submit`, inscripción |
| Ingesta de evidencias | ≤ 200 ms | `POST /progress` — es el de mayor volumen |
| Emisión de URL firmada | ≤ 150 ms | `GET /assets/{id}/content` |
| Encolado asíncrono | ≤ 300 ms | `:complete` responde `202` sin esperar al worker |

p99 ≤ 2× el p95 de su clase. Tasa de error `5xx` < 0,5 % bajo carga nominal.

**No entran en el p95 de la API** la subida y la bajada de binarios: van directo
al almacenamiento de objetos (ADR-0005). Se miden aparte.

## Procesamiento asíncrono

| Métrica | Objetivo |
| --- | --- |
| Tiempo hasta video disponible (10 min de 1080p) | ≤ 3× la duración del original |
| Correo de verificación encolado → enviado | p95 ≤ 30 s |
| Insignia: `approved` → emitida | p95 ≤ 60 s |
| Profundidad de la cola `critical` | < 50 en régimen |

`job_duration_seconds{type="media.transcode_hls"}` dividido por la duración del
original da el **costo por minuto de transcodificación** que el enunciado (§11)
recomienda medir desde el MVP para decidir entre FFmpeg y un servicio gestionado.

## Disponibilidad y recuperación

| Objetivo | Valor | Origen |
| --- | --- | --- |
| Disponibilidad mensual de la API | 99,5 % (≈ 3,6 h/mes) | Propio |
| RPO | ≤ 15 min | Enunciado §9 |
| RTO | ≤ 4 h | Enunciado §9 |

Cómo se cumplen (ADR-0003):

- `pg_dump` completo diario a `mooc-backups`.
- WAL archivado continuo, ciclo ≤ 5 min → RPO real ~5 min, holgado frente a 15.
- `make restore-test`: restaura en un contenedor limpio, corre el smoke test y
  **reporta el tiempo**. Ese número es la evidencia del RTO.
- Redis con `appendonly yes`. Perderlo desloguea a todos, no pierde trabajos
  (patrón *outbox*, ADR-0004).
- Los objetos son la copia; en GCP se activa versionado de bucket.

## Prueba de carga (SEG-9)

Con **k6** en el perfil `carga` de Compose.

| Escenario | Perfil | Qué demuestra |
| --- | --- | --- |
| Navegación de catálogo | 500 VU, 5 min | Lectura y caché |
| Consumo de curso | 1.000 VU, 10 min, con heartbeats cada 15 s | La ruta de mayor volumen |
| Presentación de quiz | 200 VU, 5 min | Escritura con snapshot y calificación |
| Pico mixto | 2.000 VU, 3 min | `RNF-02` |
| Escalado | El mixto con `api=1` y luego `api=3` | Que escalar horizontalmente sirve (`CE-01`) |

Se registra p50/p95/p99 por clase, tasa de error, `queue_depth`, `db_pool_in_use`
y CPU por contenedor. La comparación `api=1` frente a `api=3` es la evidencia más
directa del criterio de arquitectura.

## Criterio de aceptación de la entrega

La prueba de carga de Etapa 1 no puede presentar **incumplimientos críticos**
(§10). Lo definimos así:

- Ningún p95 por encima del doble de su objetivo.
- Tasa de `5xx` < 1 %.
- Ningún trabajo perdido: `jobs_processed_total` cuadra con lo encolado.
- Ningún error de idempotencia: cero salidas duplicadas.
