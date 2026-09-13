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

## Scripts portables entre sistemas — hecho

Los scripts de `scripts/` corrían en el host y arrastraban las diferencias entre
GNU y BSD: `declare -A` y `${var,,}` exigen bash 4 y macOS trae 3.2; `stat -c`,
`sha256sum` y `split -d` son de GNU; y `jq()`, que es un `python3 -c` usado 53
veces, convertía a python3 en dependencia obligatoria de la máquina, que macOS
12.3+ no trae de fábrica. Se arreglaban uno a uno y volvían con el siguiente
script.

Ahora corren dentro de `mooc-herramientas` (`scripts/Dockerfile`): bash,
coreutils, gawk, python3 y newman fijos, más el cliente de Docker para los
`docker compose exec` de los scripts. El Makefile la invoca por debajo, así que
`make demo`, `make smoke` y las demás se escriben y se ven igual que antes —lo
que importaba, porque el guion del video ya estaba ensayado.

**Lo que costó resolver:** los scripts y la colección apuntan a
`http://localhost:8090`, y dentro de un contenedor eso es el propio contenedor.
Las dos salidas evidentes no sirven: reescribir `/etc/hosts` no cambia nada
porque glibc y musl resuelven el nombre `localhost` internamente, sin mirar el
archivo (comprobado en las dos libc), y `--network host` no existe de verdad en
macOS ni en Windows, donde el demonio vive en una máquina virtual. La salida es
`entrada.sh`: socat escucha en `127.0.0.1` en los mismos puertos que publica la
máquina y los reenvía al servicio dentro de la red de Docker. Reenvía en crudo,
así que la firma de las URL prefirmadas de MinIO —que incluye el Host— sigue
siendo válida; sin eso, la carga multimedia fallaría con `SignatureDoesNotMatch`.

**Efecto lateral que conviene saber:** cada ejecución sale de una IP distinta,
la del contenedor, en vez de compartir `127.0.0.1`. Dos corridas seguidas de la
colección ya no se estorban en el limitador de tasa.

## Dónde estamos

Estado a 13 de septiembre de 2026. Medido contra el contrato: **60 de 88
operaciones** responden; las 28 restantes llevan `x-estado: planificado`.

| | Hecho | Falta |
| --- | --- | --- |
| Operaciones de la API | 60 | 28 |
| Módulos de dominio | 8: `identity`, `audit`, `authoring`, `learning`, `assessment`, `badges`, `admin`, `media` | Ninguno entero |
| Tipos de trabajo asíncrono | 3 (`email.send`, `badge.issue`, `media.probe`) + el `reaper` | 7 |
| Migraciones | 2 (esquema completo del dominio) | Ninguna bloqueante |
| Carpetas de la colección con contenido | 9 de 10 | SEG-9, que se demuestra con `make demo` y `make scale` |

**El flujo de trabajo completo funciona de punta a punta**: registro →
verificación por correo → autoría → publicación → carga multimedia verificada →
catálogo → inscripción → consumo → quiz → progreso → aprobación → insignia
verificable con imagen pública. `make demo` recorre 33 aserciones y la colección
entera 140.

Lo que falta de verdad es **el procesamiento de medios**: el escaneo antimalware
con ClamAV y la transcodificación a HLS. La carga, la reanudación y la
verificación —checksum recalculado y MIME por *magic bytes*— sí están.

### Evidencia ejecutable

| Orden | Qué comprueba |
| --- | --- |
| `make demo` | 33 aserciones, el recorrido del video de punta a punta |
| `make postman` | Segmentos rápidos, 112 aserciones; es lo que corre el CI |
| `make postman-completo` | Las diez carpetas, 140 aserciones; necesita `make obs` |
| `make invariantes` | Los invariantes de los ADR que `make demo` no cubre |
| `make arch` | Reglas de los ADR 0001, 0002 y 0010 |
| `make contrato` | Que el contrato y la API no se hayan separado |
| `make ci` | Todo lo anterior más formato, lint, gosec y govulncheck |

## Qué falta — módulos de dominio## Qué falta — módulos de dominio

Ninguno de estos existe todavía, salvo lo indicado en `identity`.

| Módulo | Endpoints | Qué implica | Requisitos |
| --- | --- | --- | --- |
| ~~`authoring`~~ | **hecho** | El más grande. Cursos, versiones, módulos, unidades y recursos; reordenamiento; autosave; previsualización; validación de publicación con lista exhaustiva; versiones inmutables y `stable_id`. Incluye el normalizador de Markdown canónico | `RF-03`, `RF-04`, `CA-01`, `CE-03` |
| ~~`assessment`~~ | **hecho** | Autoría de quizzes, snapshot del intento, guardado parcial, expiración, envío idempotente, calificación en servidor, retroalimentación por política | `RF-08`, `CA-04`, `CE-05` |
| ~~`admin`~~ | **hecho** | Alta de profesores con invitación, roles, estados, sesiones ajenas, auditoría, cola y reencolado. Protección del último administrador con prueba de tabla | `RF-02`, `CE-02` |
| `media` | 9 | **Lo único grande que falta.** Esquema, contrato y decisiones ya existen; falta el código. Plan de implementación en [`media-plan.md`](media-plan.md) | `RF-05`, `RF-06`, `RF-07`, `CA-02` |
| ~~`catalog` + `enrollment`~~ | **hecho** (módulo `learning`) | Catálogo con búsqueda, filtros y cursores; inscripción, retiro y reinscripción conservando progreso | `RF-10` |
| ~~`badges`~~ | **hecho** | Emisión única por inscripción, imagen, URL pública de verificación sin correo, revocación auditada | `RF-09`, `CA-07` |
| ~~`progress`~~ | **hecho** (módulo `learning`) | Pocos endpoints, mucha regla: ingesta de evidencias, rechazo y auditoría de manipulación, cálculo sobre obligatorios, transición a `completed` y `approved` | `RF-09`, `CA-05`, `CE-06` |
| `identity` | 7 de 12 | Faltan: reenvío de verificación, recuperación y restablecimiento de contraseña, `PATCH /me`, cambio de contraseña, listado y revocación de sesiones propias | `RF-01` |

## Qué falta — plataforma transversal

Esto atraviesa todos los módulos. Si llega tarde, hay que reescribir handlers ya
hechos, así que conviene antes de repartir el dominio.

| Pieza | Estado | Por qué importa |
| --- | --- | --- |
| Middleware de `Idempotency-Key` | **Hecho**: 11 operaciones no repetibles lo exigen. Repetir con la misma clave devuelve la respuesta guardada con `Idempotency-Replayed`; con cuerpo distinto, `422` | `RT-05` |
| Rate limiting en Redis | **Hecho**: login en dos niveles (cuenta e IP), registro, verificación y progreso. Base lógica 3 | `CE-02` |
| `ETag` / `If-Match` | **Hecho** en el autosave. El ETag sale del contenido, así que guardar lo mismo da el mismo ETag (ADR-0014) | `RF-04` |
| Paginación por cursor | **Hecho** en el catálogo, con `limit` y `cursor` opaco. Los demás listados siguen devolviendo `next_cursor: null` | `RT-05` |
| OpenAPI 3.1 | **Completo**: las 88 operaciones del inventario, validadas con redocly sin advertencias. Las 80 que aún no responden llevan `x-estado: planificado`. La API lo sirve en `GET /openapi.yaml` y `make contrato` detecta la deriva | `RT-05` y entregable §8 |
| `/metrics` (Prometheus) | **Hecho**: API y worker, con Prometheus, Grafana y 4 reglas de alerta. La de DLQ verificada disparando | `CA-03`, `RNF-05` |
| Trazas OpenTelemetry | Solo hay logs estructurados | `RT-06`, exigido explícitamente |
| Protección del último administrador | **Hecho**, comprobada dentro de la transacción y con prueba de tabla | `RF-02` |

## Qué falta — workers

Solo corre `email.send`.

| Trabajo | Estado |
| --- | --- |
| `email.send` | **Hecho** |
| `media.probe` | Falta — checksum real, MIME por *magic bytes*, duración y resolución |
| `media.scan` | Falta — ClamAV por INSTREAM, cuarentena y auditoría |
| `media.transcode_hls` | Falta — FFmpeg, escalera sin *upscaling*, original intacto |
| `media.poster` | Falta |
| `badge.issue` | **Hecho** |
| `progress.recompute` | Falta |
| `jobs.reaper` (programado) | **Hecho**: recupera los `queued` huérfanos y los `running` sin heartbeat |
| `assessment.expire_attempts` (programado) | Falta |
| `media.sweep_uploads` (programado) | Falta |
| Poda de `job_runs`, `progress_events` e `idempotency_keys` (programado) | Falta |
| `doc.convert_pdf` | Opcional (`RO-01`) |

> **El `reaper` ya está.** Cierra la otra mitad de la garantía del outbox: el
> trabajo se registra en PostgreSQL antes de publicarse, y si la publicación se
> pierde, el reaper lo republica. `make demo` lo demuestra devolviendo un
> trabajo a la cola y comprobando que el worker lo **reejecuta** (intento 2) sin
> producir una segunda insignia. Sin el reaper, esa prueba habría pasado por el
> motivo equivocado: porque nadie lo ejecutaba.

## Qué falta — infraestructura y evidencia

| Pieza | Estado |
| --- | --- |
| `clamav` en Compose | Falta |
| `worker-media` (imagen con FFmpeg) | Falta |
| Perfil `observability` | **Hecho** salvo Jaeger (trazas OTel siguen pendientes) |
| Perfil `carga` (k6) | Falta |
| Pruebas automáticas | **Andamiaje hecho**: dominio, contraseñas, capa HTTP con dobles y `ParseQueues`. Falta cubrir cada módulo nuevo |
| Semilla de datos sintéticos (`make seed`) | **Hecho** — 8 cuentas, idempotente |
| Colección de Postman (SEG-1 … SEG-9) | **Estructura hecha**: SEG-1 con 13 peticiones y aserciones; SEG-2 a SEG-8 vacías a propósito |
| Backup, restauración y `make restore-test` | Falta |
| CI: build, lint, `gosec`/`govulncheck`, migraciones, pruebas | **Hecho** — 4 trabajos, incluido el extremo a extremo con Compose |
| Fallos inyectables para SEG-4 (`FAIL_TRANSCODE`, `DUPLICATE_DELIVERY`) | Falta |

El andamiaje de evidencia ya está montado (pruebas, semilla, Postman y CI). Lo
que queda es **usarlo**: cada módulo nuevo entra con sus pruebas y su carpeta de
Postman en el mismo PR. El enunciado es explícito en §10 — un comportamiento que
no pueda reproducirse no acredita el requisito, aunque exista el código.

## Orden propuesto

Lo de arriba desbloquea lo de abajo.

| # | Tarea | Quién | Estado |
| --- | --- | --- | --- |
| 1 | **Revisar el contrato entre los cuatro** y acordar el resto de las migraciones. El OpenAPI ya está escrito con las 88 operaciones: es un borrador para enmendar, no un acuerdo | Todos | Pendiente |
| 2 | Plataforma transversal: idempotencia HTTP, `ETag`, cursores (el rate limiting ya está) | — | Pendiente |
| 3 | Cerrar el `reaper` y las tareas programadas | — | Pendiente |
| 4 | Normalizador de Markdown canónico y su prueba de ida y vuelta (§11 recomienda hacerlo primero) | — | Pendiente |
| 5 | `authoring` con validación de publicación e inmutabilidad | — | Pendiente |
| 6 | `media` + workers de medios + ClamAV + FFmpeg (en paralelo con 5: es independiente y el que más tarda en afinarse) | — | Pendiente |
| 7 | Completar `identity` y hacer `admin` | — | Pendiente |
| 8 | `catalog` + `enrollment` | — | Pendiente |
| 9 | `assessment` con snapshot y calificación | — | Pendiente |
| 10 | `progress` + `badges` | — | Pendiente |
| 11 | Observabilidad: `/metrics`, OTel, paneles y alerta de DLQ | — | Pendiente |
| 12 | Llenar la carpeta de Postman de cada segmento a medida que entre su módulo | — | Pendiente |
| 13 | Pruebas de carga con k6 y medición de p95 con `api=1` frente a `api=3` | — | Pendiente |
| 14 | `make restore-test` y medición de RTO | — | Pendiente |
| 15 | Grabar el video de la demostración | Todos | Pendiente |

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
| Contrato OpenAPI | Las 88 operaciones, validadas con redocly sin advertencias, servidas por la API. `scripts/contrato.sh` comprueba en CI que lo declarado como implementado exista de verdad |
| ADR-0015, D2 y D3 | Aceptadas: cookie firmada con endpoint `media-sessions` (ya en el contrato) y prohibición de claves JSON (comprobada en el CI) |
| Andamiaje de evidencia | Pruebas unitarias y de capa HTTP, `make seed`, colección de Postman por segmentos, CI con 4 trabajos y plantilla de PR |

## Revisión del 7 de septiembre de 2026

Se revisaron todos los módulos contra lo que los ADR **prometen verificar**, no
contra lo que parecía funcionar. Dos hallazgos.

**El ADR-0001 anunciaba un `make arch-test` que nunca se escribió.** Al
escribirlo, el código incumplía su regla 3 en 39 sitios: 5 escrituras y 34
lecturas del esquema de otro módulo.

- La escritura de `assessment` en `progress.resource_progress` era el problema
  real: marcaba completado el recurso de un quiz saltándose las reglas de
  `learning`. **Corregida**: ahora se pide por interfaz de servicio.
- Las de `admin` sobre `identity` quedan como **excepción documentada**: son el
  mismo contexto con dos caras y comparten el agregado.
- Las 34 lecturas llevaron a **enmendar el ADR-0001**: en un monolito con una
  sola base, un `JOIN` en la capa de adaptadores es más simple y más rápido que
  reunir en memoria lo que la base sabe reunir. La regla pasa a prohibir
  escrituras, no lecturas, y el script las cuenta para que el coste de extraer
  un módulo esté a la vista.

**El invariante más citado no estaba probado.** «El progreso sobrevive a una
versión nueva» aparece en el ADR-0007, en el README y en el guion, y nunca se
había verificado de punta a punta. Ahora sí: `make invariantes` publica una
versión 2, comprueba que el `stable_id` se conserva y el `id` de fila cambia, y
que el estudiante conserva su 100 % y su aprobación.

`make arch` y `make invariantes` corren en `make ci` y en el CI.

## MinIO es una dependencia congelada

MinIO terminó en octubre de 2025 la distribución de imágenes precompiladas y
retiró sus repositorios de Docker Hub: `docker pull minio/mc` falla con
«repository does not exist». `docker-compose.yml` apunta ahora a `quay.io` con
etiqueta fija.

**Lo que hay que saber:** quay.io conserva las etiquetas históricas pero **no
publicará versiones nuevas**. La que usamos es de septiembre de 2025, así que no
habrá parches de seguridad por esa vía.

**Por qué se asume:** MinIO solo existe en el entorno local. En GCP el
almacenamiento de objetos es GCS (ADR-0005, `disenos/ruta-a-gcp.md`), así que es
una dependencia de desarrollo, no de producción.

**Si molesta más adelante:** cambiar a otra imagen compatible con S3 que siga
mantenida, o compilarla. No es trabajo para las primeras entregas.

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
| **Cliente de objetos en GCP** | Interoperabilidad S3 / **adaptador GCS nativo** | Equipo | ADR-0015 (D1). D3 ya lo implica casi por completo: las claves HMAC también son una credencial estática |
| Alcance de la credencial de reproducción | Por asset / por versión de curso | Equipo | Consecuencia abierta de D2. **No toca el contrato**: la respuesta lleva un campo `scope`. Se decide al implementar `media` |

## Riesgos vigilados

| Riesgo | Señal temprana | Qué haríamos |
| --- | --- | --- |
| El alcance de E1 es amplio para 4 personas | La tarea 1 no está lista al final de la semana 1 | Congelar el contrato aunque esté imperfecto; se puede corregir con migraciones |
| Las pruebas se dejan para el final | Un módulo se da por hecho sin `_test.go` | La plantilla de PR lo pide como casilla explícita; el CI corre `go test` en cada empuje |
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
- **`make ci` en verde antes de empujar.** Corre lo mismo que el CI: formato,
  vet, pruebas, contrato, seguridad, smoke y Postman. Empujar y dejar que el CI
  encuentre el fallo llena de correos a todo el equipo, y el fallo se descubre
  diez minutos más tarde.
