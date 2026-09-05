# Pendientes

Notas de trabajo del equipo. **No es entregable.**

Estado a 5 de septiembre de 2026: documentación de arquitectura lista y
**rebanada vertical funcionando** en `feat/esqueleto-backend` (Compose,
migración 0001, plataforma, `identity` con registro/verificación/login/me y el
worker `email.send`).

**Verificado en local:** `make up` levanta los 9 servicios, `make smoke` pasa sus
12 comprobaciones, y `make scale` (3 api + 3 worker) las vuelve a pasar — la
sesión se abre en una instancia y `/me` la resuelve otra, que es la evidencia de
que la API no guarda estado (CE-01).

## Dónde estamos

Medido contra el inventario de `disenos/api-v1.md`: **7 de 84 endpoints**.

| | Hecho | Falta |
| --- | --- | --- |
| Endpoints | 7 | 77 |
| Módulos de dominio | 2 de 10 (`identity` parcial, `audit`) | 8 |
| Tipos de trabajo asíncrono | 1 de 11 | 10 |
| Migraciones | 1 (`identity`, `audit`, `platform`) | El resto del esquema |
| Pruebas automáticas | 0 | Todas |

Está el esqueleto y una rebanada. Falta prácticamente todo el dominio.

## Qué falta — módulos de dominio

Ninguno de estos existe todavía, salvo lo indicado en `identity`.

| Módulo | Endpoints | Qué implica | Requisitos |
| --- | --- | --- | --- |
| `authoring` | 24 | El más grande. Cursos, versiones, módulos, unidades y recursos; reordenamiento; autosave; previsualización; validación de publicación con lista exhaustiva; versiones inmutables y `stable_id`. Incluye el normalizador de Markdown canónico | `RF-03`, `RF-04`, `CA-01`, `CE-03` |
| `assessment` | 10 | Autoría de quizzes, snapshot del intento, guardado parcial, expiración, envío idempotente, calificación en servidor, retroalimentación por política | `RF-08`, `CA-04`, `CE-05` |
| `admin` | 9 | Alta de profesores, roles, estados, sesiones ajenas, consulta de auditoría, listado de la cola y reencolado desde la DLQ. Protección del último administrador activo | `RF-02`, `CE-02` |
| `media` | 9 | Multipart prefirmado, reanudación 24 h, verificación de checksum y MIME real, escaneo antimalware, entrega firmada de HLS y PDF | `RF-05`, `RF-06`, `RF-07`, `CA-02` |
| `catalog` + `enrollment` | 8 | Catálogo con búsqueda, filtros y cursores; inscripción, retiro y reinscripción conservando progreso | `RF-10` |
| `badges` | 5 | Emisión única por inscripción, imagen, URL pública de verificación sin correo, revocación auditada | `RF-09`, `CA-07` |
| `progress` | 3 | Pocos endpoints, mucha regla: ingesta de evidencias, rechazo y auditoría de manipulación, cálculo sobre obligatorios, transición a `completed` y `approved` | `RF-09`, `CA-05`, `CE-06` |
| `identity` | 7 de 12 | Faltan: reenvío de verificación, recuperación y restablecimiento de contraseña, `PATCH /me`, cambio de contraseña, listado y revocación de sesiones propias | `RF-01` |

## Qué falta — plataforma transversal

Esto atraviesa todos los módulos. Si llega tarde, hay que reescribir handlers ya
hechos, así que conviene antes de repartir el dominio.

| Pieza | Estado | Por qué importa |
| --- | --- | --- |
| Middleware de `Idempotency-Key` | La tabla `platform.idempotency_keys` existe **vacía y sin usar** | `RT-05`; sin esto no hay envío idempotente de quiz (`CA-04`) ni inscripción segura |
| Rate limiting en Redis | No existe | `CE-02`. La base lógica 3 está reservada y sin usar |
| `ETag` / `If-Match` | No existe | El autosave de `RF-04` lo necesita para detectar escrituras concurrentes |
| Paginación por cursor | No existe (no hay listas todavía) | `RT-05` |
| OpenAPI 3.1 | `backend/openapi/` está vacío | `RT-05` y entregable §8 |
| `/metrics` (Prometheus) | No existe | Sin métricas no hay alerta de DLQ (`CA-03`) ni p95 (`RNF-05`) |
| Trazas OpenTelemetry | Solo hay logs estructurados | `RT-06`, exigido explícitamente |
| Protección del último administrador | No existe | `RF-02` |

## Qué falta — workers

Solo corre `email.send`.

| Trabajo | Estado |
| --- | --- |
| `email.send` | **Hecho** |
| `media.probe` | Falta — checksum real, MIME por *magic bytes*, duración y resolución |
| `media.scan` | Falta — ClamAV por INSTREAM, cuarentena y auditoría |
| `media.transcode_hls` | Falta — FFmpeg, escalera sin *upscaling*, original intacto |
| `media.poster` | Falta |
| `badge.issue` | Falta |
| `progress.recompute` | Falta |
| `jobs.reaper` (programado) | Falta — **ver abajo** |
| `assessment.expire_attempts` (programado) | Falta |
| `media.sweep_uploads` (programado) | Falta |
| Poda de `job_runs`, `progress_events` e `idempotency_keys` (programado) | Falta |
| `doc.convert_pdf` | Opcional (`RO-01`) |

> **El `reaper` merece mención aparte.** La mitad de la garantía del outbox está
> escrita: el trabajo se registra en PostgreSQL en la misma transacción que el
> cambio de dominio, antes de publicarse en asynq. Pero **nadie recupera todavía
> las filas huérfanas**, así que la afirmación "perder Redis no pierde trabajo"
> (ADR-0004, recomendación §11 del enunciado) hoy es cierta a medias. Es barato
> de cerrar y conviene hacerlo antes de que se apoye más código en esa promesa.

## Qué falta — infraestructura y evidencia

| Pieza | Estado |
| --- | --- |
| `clamav` en Compose | Falta |
| `worker-media` (imagen con FFmpeg) | Falta |
| Perfil `observability` (Prometheus, Grafana, Jaeger) | Falta |
| Perfil `carga` (k6) | Falta |
| **Pruebas automáticas** | **Cero.** `make test` corre y no ejecuta nada |
| Semilla de datos sintéticos (`make seed`) | Falta |
| Colección de Postman (SEG-1 … SEG-9) | Falta |
| Backup, restauración y `make restore-test` | Falta |
| CI: build, lint, `gosec`/`govulncheck`, migraciones, pruebas | Falta |
| Fallos inyectables para SEG-4 (`FAIL_TRANSCODE`, `DUPLICATE_DELIVERY`) | Falta |

Que no haya ninguna prueba es lo más urgente de esta tabla: es criterio Must
(`CE-07`) y el enunciado es explícito en §10 — un comportamiento que no pueda
reproducirse no acredita el requisito, aunque exista el código.

## Orden propuesto

Lo de arriba desbloquea lo de abajo.

| # | Tarea | Quién | Estado |
| --- | --- | --- | --- |
| 1 | Acordar entre los cuatro el OpenAPI y el resto de las migraciones. Sigue siendo el cuello de botella para trabajar en paralelo | Todos | Pendiente |
| 2 | Plataforma transversal: idempotencia, rate limiting, `ETag`, cursores | — | Pendiente |
| 3 | Cerrar el `reaper` y las tareas programadas | — | Pendiente |
| 4 | Normalizador de Markdown canónico y su prueba de ida y vuelta (§11 recomienda hacerlo primero) | — | Pendiente |
| 5 | `authoring` con validación de publicación e inmutabilidad | — | Pendiente |
| 6 | `media` + workers de medios + ClamAV + FFmpeg (en paralelo con 5: es independiente y el que más tarda en afinarse) | — | Pendiente |
| 7 | Completar `identity` y hacer `admin` | — | Pendiente |
| 8 | `catalog` + `enrollment` | — | Pendiente |
| 9 | `assessment` con snapshot y calificación | — | Pendiente |
| 10 | `progress` + `badges` | — | Pendiente |
| 11 | Observabilidad: `/metrics`, OTel, paneles y alerta de DLQ | — | Pendiente |
| 12 | Semilla de datos sintéticos (`make seed`) | — | Pendiente |
| 13 | Colección de Postman, una carpeta por segmento SEG-1…SEG-9 | — | Pendiente |
| 14 | Pruebas de carga con k6 y medición de p95 con `api=1` frente a `api=3` | — | Pendiente |
| 15 | `make restore-test` y medición de RTO | — | Pendiente |
| 16 | CI: build, lint, análisis de seguridad, migraciones, pruebas | — | Pendiente |
| 17 | Grabar el video de la demostración | Todos | Pendiente |

Las pruebas no son una fila de esta tabla a propósito: van con cada módulo, no al
final. Un módulo sin prueba no está terminado (ver la definición de terminado en
`alcance-entrega-1.md`).

### Ya hecho

| Tarea | Detalle |
| --- | --- |
| Esqueleto del repo | `backend/` con `cmd/`, `internal/{platform,adapters,modules}`, `Dockerfile` multi-stage, `Makefile` |
| `docker-compose.yml` | 9 servicios con healthchecks y orden de arranque; falta `clamav`, `worker-media` y observabilidad |
| Migración `0001` | Esquemas `identity`, `audit` y `platform`, con el trigger de inmutabilidad de la auditoría |
| Plataforma, parcial | Errores RFC 9457 con `trace_id`, logs estructurados, `job_runs` (outbox, idempotencia y DLQ), reclamo atómico, `/healthz` y `/readyz` |
| `identity`, parcial | Registro, verificación de correo, login, `/me`, logout con revocación inmediata |
| `audit` | Registro inmutable, verificado contra `UPDATE` |
| Proxy y escalado | Caddy con reparto por turnos; `--scale api=3 --scale worker=3` verificado |

## Decisiones abiertas

Cuando se resuelvan, cada una se convierte en un ADR.

| Tema | Opciones | Quién decide | Notas |
| --- | --- | --- | --- |
| Caché del catálogo en Redis | Cachear la lista con invalidación al publicar / no cachear en E1 | Equipo | Depende de lo que muestre la prueba de carga. Medir antes de optimizar |
| Búsqueda del catálogo | `tsvector` de PostgreSQL / `pg_trgm` para tolerar erratas | Equipo | Empezar con `tsvector`; ya cubre `RF-10` |
| Generación de la imagen de insignia | Plantilla SVG → PNG en Go / composición con `ffmpeg` | Equipo | La SVG es más fácil de ajustar |
| Proxy de entrada | Caddy (TLS automático) / Nginx | Equipo | Caddy ya está en Compose; queda confirmarlo como definitivo |
| `sqlc` para el acceso a datos | SQL a mano / `sqlc` | Equipo | Reconsiderar si el SQL a mano se vuelve pesado (ADR-0002) |
| Tecnología del frontend (E2) | React / Angular / Svelte | Equipo | No bloquea la Entrega 1 |
| Token en claro dentro de `job_runs.payload` | Dejarlo / cifrar el payload / que el worker genere el token | Equipo | Hoy el token de verificación viaja en claro en el payload del trabajo, porque el correo debe contenerlo y `one_time_tokens` solo guarda el hash. Acotado: expira en 24 h y la fila se poda. Ver `internal/modules/identity/service.go` |
| Recorte de la escalera HLS para la demo | 360p+720p / la escalera completa | Equipo | La completa alarga mucho la grabación |

## Riesgos vigilados

| Riesgo | Señal temprana | Qué haríamos |
| --- | --- | --- |
| El alcance de E1 es amplio para 4 personas | La tarea 1 no está lista al final de la semana 1 | Congelar el contrato aunque esté imperfecto; se puede corregir con migraciones |
| Las pruebas se dejan para el final | Un módulo se da por hecho sin `_test.go` | La definición de terminado ya lo prohíbe; hacerlo visible en la revisión del PR |
| FFmpeg alarga la grabación de la demo | Un video de prueba tarda más de 5 min | Usar clips de 30 s y una escalera recortada |
| ClamAV falla al arrancar en la máquina de la demo | El healthcheck no pasa en 3 min | Imagen con firmas precargadas; ensayar en la máquina donde se grabará |
| La carga multipart es incómoda desde Postman | El script pre-request no parte bien el archivo | Un `scripts/upload.sh` con `curl` como respaldo, mostrado en el video |
| Dos personas tocan las mismas migraciones | Números repetidos en un PR | Verificación en CI; renumerar al rebasar |
| Colisión de puertos con otros stacks de Docker de la máquina | `port is already allocated` al levantar | Ya resuelto: los puertos del host se parametrizan en `.env` y el proyecto usa un rango propio (8090, 8026, 9010/9011, 5433, 6380) |

## Convenciones acordadas

- Ramas: `feat/<modulo>-<descripcion>`, `fix/…`, `docs/…`.
- Commits en español, con el ID del requisito: `feat(media): carga multipart reanudable (RF-05)`.
- Un PR por módulo o por tarea de esta lista, no por sesión de trabajo.
- Nadie hace merge de un PR propio sin revisión de otro del equipo.
