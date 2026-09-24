# Pendientes

Notas de trabajo del equipo. **No es entregable.**

**Entrega 1 entregada y grabada.** Después del video se cerró el módulo
`media` —antivirus, transcodificación a HLS y sesión de reproducción— y se
preparó el backend para GCP. Lo que viene es el despliegue, con su plan en
[`despliegue-gcp.md`](despliegue-gcp.md).

Este documento se revisó el **23 de septiembre de 2026** contra el código, no
contra la memoria: durante dos semanas dijo que no existía casi nada de lo que
ya estaba hecho.

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

Estado a 23 de septiembre de 2026. Medido contra el contrato: **65 de 90
operaciones** responden; las 25 restantes llevan `x-estado: planificado`.

El backend está preparado para GCP: obedece `PORT`, sabe escribir logs de
Cloud Logging, habla TLS con Redis, ajusta el pool por entorno y tiene el puerto
del almacén de objetos con `OBJECT_STORE` como única variable que nombra al
proveedor.

Después de grabar el video entraron el antivirus, la transcodificación a HLS y
la sesión de reproducción. El video no las muestra, y `demo/guion.md` tampoco:
ese documento describe lo que se grabó y no se actualiza hacia atrás.

| | Hecho | Falta |
| --- | --- | --- |
| Operaciones de la API | 65 | 25 |
| Módulos de dominio | 8: `identity`, `audit`, `authoring`, `learning`, `assessment`, `badges`, `admin`, `media` | Ninguno entero; `identity` es el más incompleto (5 de 12) |
| Tipos de trabajo asíncrono | 5 (`email.send`, `badge.issue`, `media.probe`, `media.scan`, `media.transcode_hls`) + el `reaper` | 4: `progress.recompute` y tres programados (poda, expiración de intentos, barrido de cargas) |
| Migraciones | 2 (esquema completo del dominio) | Ninguna bloqueante |
| Carpetas de la colección con contenido | 9 de 10 | SEG-9, que se demuestra con `make demo` y `make scale` |

**El flujo de trabajo completo funciona de punta a punta**: registro →
verificación por correo → autoría → publicación → carga multimedia verificada,
escaneada y transcodificada → catálogo → inscripción → consumo con manifiesto
HLS → quiz → progreso → aprobación → insignia verificable con imagen pública.

### Evidencia ejecutable

| Orden | Qué comprueba | Cuánto tarda |
| --- | --- | --- |
| `make rapido` | Formato, arquitectura, `vet` y pruebas sin detector de carreras | 13 s |
| `make demo` | 33 aserciones, el recorrido del video de punta a punta | ~1 min |
| `make postman` | Segmentos rápidos, ~138 aserciones (varía con los sondeos al worker); es lo que corre el CI | 27 s |
| `make postman-completo` | Las diez carpetas, con los heartbeats reales | ~12 min, a propósito |
| `make invariantes` | Los invariantes de los ADR que `make demo` no cubre, incluida la cadena entera de medios | 44 s |
| `make arch` | Reglas de los ADR 0001, 0002 y 0010 | 3 s |
| `make contrato` | Que el contrato y la API no se hayan separado | 2 s |
| `make ci` | Todo lo anterior más formato, lint, gosec y govulncheck | **1 min 35 s** |

Los tiempos son con las cachés calientes. Hasta el 23 de septiembre `make ci`
tardaba unos doce minutos porque **ningún contenedor montaba la caché de
compilación de Go**: `vet`, `test` y los analizadores recompilaban el árbol
entero cada vez. Si vuelven a tardar así, alguien borró los volúmenes
`mooc-gobuildcache`, `mooc-gomodcache` o `mooc-gobin`.

## Qué falta — módulos de dominio

Los ocho existen y funcionan. Lo que queda son **25 operaciones** del contrato,
todas marcadas `x-estado: planificado`, y ninguna bloquea el flujo completo.

| Módulo | Estado | Qué falta |
| --- | --- | --- |
| `identity` | 5 de 12 | Reenvío de verificación, recuperación y restablecimiento de contraseña, `PATCH /me`, cambio de contraseña propia, listado y revocación de sesiones propias |
| `authoring` | 16 de 27 | Borrados (curso, módulo, unidad, recurso), los tres `reorder`, `PATCH` de módulo y unidad, y el historial de revisiones de un recurso con su restauración |
| `assessment` | 8 de 11 | CRUD de preguntas sueltas: hoy el quiz se define entero con `PUT /resources/{id}/quiz` |
| `badges` | 3 de 5 | Detalle de una insignia propia y la imagen por código público |
| `learning` | 7 de 9 | Detalle de un recurso dentro de la inscripción y la última posición de reproducción |
| `admin`, `media`, catálogo, operación | **completos** | Nada |

La más incompleta es `identity`, y es la que más se nota: sin recuperación de
contraseña, quien la olvida no entra. Es el primer candidato de la Entrega 2.

## Qué falta — plataforma transversal

Esto atraviesa todos los módulos. Si llega tarde, hay que reescribir handlers ya
hechos, así que conviene antes de repartir el dominio.

| Pieza | Estado | Por qué importa |
| --- | --- | --- |
| Middleware de `Idempotency-Key` | **Hecho**: 12 operaciones no repetibles lo exigen. Repetir con la misma clave devuelve la respuesta guardada con `Idempotency-Replayed`; con cuerpo distinto, `422` | `RT-05` |
| Rate limiting en Redis | **Hecho**: login en dos niveles (cuenta e IP), registro, verificación y progreso. Base lógica 3 | `CE-02` |
| `ETag` / `If-Match` | **Hecho** en el autosave. El ETag sale del contenido, así que guardar lo mismo da el mismo ETag (ADR-0014) | `RF-04` |
| Paginación por cursor | **Hecho** en el catálogo, con `limit` y `cursor` opaco. Los demás listados siguen devolviendo `next_cursor: null` | `RT-05` |
| OpenAPI 3.1 | **Completo**: 90 operaciones, validadas con redocly sin advertencias. Las 25 que aún no responden llevan `x-estado: planificado`. La API lo sirve en `GET /openapi.yaml` y `make contrato` detecta la deriva | `RT-05` y entregable §8 |
| `/metrics` (Prometheus) | **Hecho**: API y worker, con Prometheus, Grafana y 4 reglas de alerta. La de DLQ verificada disparando | `CA-03`, `RNF-05` |
| Trazas OpenTelemetry | **Falta.** Hay logs estructurados correlacionados por `trace_id` y métricas, pero no trazas | `RT-06`, exigido explícitamente |
| Protección del último administrador | **Hecho**, comprobada dentro de la transacción y con prueba de tabla | `RF-02` |

## Qué falta — workers

Corren cinco tipos de trabajo más el `reaper`. Lo que falta son los
**programados**: ninguno existe todavía, y sin ellos las tablas crecen sin
límite y los intentos de quiz caducados se quedan abiertos.

| Trabajo | Estado |
| --- | --- |
| `email.send` | **Hecho** |
| `badge.issue` | **Hecho** |
| `media.probe` | **Hecho** — checksum real, MIME por *magic bytes*, duración y resolución |
| `media.scan` | **Hecho** — ClamAV por INSTREAM, cuarentena y auditoría |
| `media.transcode_hls` | **Hecho** — FFmpeg, escalera sin *upscaling*, original intacto |
| `media.poster` | **Hecho** — dentro de `media.transcode_hls`, no como trabajo aparte |
| `jobs.reaper` (programado) | **Hecho**: recupera los `queued` huérfanos y los `running` sin heartbeat |
| `progress.recompute` | Falta — hoy el progreso se recalcula dentro de la petición |
| `assessment.expire_attempts` (programado) | Falta — un intento vencido no se cierra solo |
| `media.sweep_uploads` (programado) | Falta — las cargas a medias se quedan; el ciclo de vida del bucket las limpiaría en GCP |
| Poda de `job_runs`, `progress_events` e `idempotency_keys` (programado) | Falta — **es el que más urge**: esas tablas solo crecen |
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
| `clamav` en Compose | **Hecho** — imagen con firmas precargadas, arranca en ~15 s sin red |
| `worker-media` (imagen con FFmpeg) | **Hecho** — consume solo la cola `bulk` |
| Imagen de herramientas (`mooc-herramientas`) | **Hecho** — los scripts y newman ya no corren en la máquina |
| Perfil `observability` | **Hecho** salvo trazas: `/metrics`, Prometheus, Grafana y 4 reglas de alerta |
| Perfil `carga` (k6) | **Falta.** Es una de las dos evidencias que el enunciado pide y no existen |
| Backup, restauración y `make restore-test` | **Falta.** La otra. El RTO se demuestra restaurando y cronometrando |
| Pruebas automáticas | **Hechas** por módulo: dominio, contraseñas, capa HTTP con dobles, escalera HLS, respuesta de ClamAV |
| Semilla de datos sintéticos (`make seed`) | **Hecho** — 8 cuentas, idempotente |
| Colección de Postman (SEG-1 … SEG-9) | **Hecha**: 9 carpetas con contenido, ~138 aserciones en los segmentos rápidos |
| CI | **Hecho** — 4 trabajos, incluido el extremo a extremo con Compose |
| Fallos inyectables (`FAIL_TRANSCODE`, `DUPLICATE_DELIVERY`) | **No se harán.** Se diseñaron para poder demostrar la DLQ y el reencolado; `make demo` los demuestra con un trabajo real devuelto a la cola, que es mejor evidencia que un fallo simulado |

Lo que queda de evidencia son las dos medidas que el enunciado pide y que no se
pueden fingir: **la prueba de carga y la restauración cronometrada**. Las dos se
hacen mejor contra el entorno desplegado que contra Compose, así que van después
del despliegue.

## Orden propuesto

Reescrito el 23 de septiembre. La tabla anterior daba por pendiente casi todo lo
que ya estaba hecho, que es la peor forma que tiene una lista de tareas de
volverse inútil.

| # | Tarea | Estado |
| --- | --- | --- |
| 1 | **Desplegar en GCP**, por fases, siguiendo [`despliegue-gcp.md`](despliegue-gcp.md). Empieza por lo que solo puede hacer una persona: proyecto, facturación, región y presupuesto | Siguiente |
| 2 | **Adaptador de Cloud Storage** (ADR-0015, D1). Multipart emulado con `compose`; se escribe contra GCS real porque firmar sin clave descargada no se puede comprobar en local | Con la fase 3 |
| 3 | **Prueba de carga con k6** y p95 medido con `api=1` frente a `api=3`, contra el entorno desplegado | Después del despliegue |
| 4 | **Restauración cronometrada** y `make restore-test`. El RTO se demuestra, no se declara | Después del despliegue |
| 5 | **Trabajos programados**: poda de `job_runs`, `progress_events` e `idempotency_keys` primero; luego expiración de intentos y barrido de cargas | Pendiente |
| 6 | **Completar `identity`**: recuperación de contraseña sobre todo. Sin ella, quien la olvida no entra | Pendiente |
| 7 | **Trazas con OpenTelemetry** hacia Cloud Trace (`RT-06`) | Pendiente |
| 8 | Resto de operaciones `planificado`: borrados y reordenamientos de `authoring`, CRUD de preguntas, detalles de insignia y recurso | Pendiente |
| 9 | **Frontend** (Entrega 2). La tecnología sigue sin decidir | Pendiente |

Las pruebas no son una fila de esta tabla a propósito: van con cada módulo, no al
final. Un módulo sin prueba no está terminado (ver la definición de terminado en
`alcance-entrega-1.md`).

### Ya hecho

| Tarea | Detalle |
| --- | --- |
| Los ocho módulos de dominio | `identity`, `audit`, `authoring`, `learning`, `assessment`, `badges`, `admin` y `media`, con sus pruebas |
| Plataforma | Errores RFC 9457 con `trace_id`, logs estructurados, outbox e idempotencia en `job_runs`, reclamo atómico, reaper, `ETag`, cursores, límites de tasa, `/healthz` y `/readyz` |
| Medios completos | Carga multipart reanudable, verificación en servidor, antivirus, transcodificación a HLS sin *upscaling* y entrega del manifiesto con credencial de reproducción |
| Contrato OpenAPI | 90 operaciones validadas; `scripts/contrato.sh` comprueba en el CI que lo declarado como implementado exista de verdad |
| Evidencia | `make demo` (33 aserciones), colección de Postman (~138), invariantes de los ADR, `make arch`, semilla sintética y CI con 4 trabajos |
| Infraestructura local | 12 servicios en Compose con healthchecks, escalado verificado, observabilidad y la imagen de herramientas |
| Video de la Entrega 1 | Grabado |
| Preparación para GCP | `PORT`, logs de Cloud Logging, TLS a Redis, pool por entorno, puerto del almacén de objetos y plan de despliegue |

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
| ~~Generación de la imagen de insignia~~ | — | — | **Resuelta**: SVG generado en Go, sin dependencias nuevas. `internal/modules/badges/imagen.go` |
| Proxy de entrada | Caddy (TLS automático) / Nginx | Equipo | Caddy ya está en Compose; queda confirmarlo como definitivo |
| `sqlc` para el acceso a datos | SQL a mano / `sqlc` | Equipo | Reconsiderar si el SQL a mano se vuelve pesado (ADR-0002) |
| Tecnología del frontend (E2) | React / Angular / Svelte | Equipo | No bloquea la Entrega 1 |
| Token en claro dentro de `job_runs.payload` | Dejarlo / cifrar el payload / que el worker genere el token | Equipo | Hoy el token de verificación viaja en claro en el payload del trabajo, porque el correo debe contenerlo y `one_time_tokens` solo guarda el hash. Acotado: expira en 24 h y la fila se poda. Ver `internal/modules/identity/service.go` |
| ~~Recorte de la escalera HLS para la demo~~ | — | — | **Sin objeto**: el video ya está grabado, y la escalera se recorta sola porque no hace *upscaling*. Un original de 360p da una variante |
| ~~Cliente de objetos en GCP~~ | — | — | **Resuelta** el 23-09: adaptador GCS nativo, con la carga multipart emulada por `compose` porque GCS no la tiene. ADR-0015 (D1) |
| ~~Alcance de la credencial de reproducción~~ | — | — | **Resuelta**: por asset. Por versión de curso obligaría a duplicar los derivados en cada versión nueva, que es justo lo que el `stable_id` evita. El campo `scope` deja cambiarlo sin romper al cliente |

## Riesgos vigilados

| Riesgo | Señal temprana | Qué haríamos |
| --- | --- | --- |
| Que el despliegue se coma la Entrega 2 | La fase 1 de `despliegue-gcp.md` no está lista en una semana | Desplegar solo `dev` y dejar `prod` para después; el frontend no depende del despliegue |
| Las pruebas se dejan para el final | Un módulo se da por hecho sin `_test.go` | La plantilla de PR lo pide como casilla explícita; el CI corre `go test` en cada empuje |
| El egreso de video dispara la factura de GCP | La alerta de presupuesto al 50 % llega en la primera semana | Medir antes de abrir el CDN; está en los riesgos de `despliegue-gcp.md` |
| ~~ClamAV falla al arrancar~~ | — | **Resuelto**: la imagen trae las firmas dentro y clamd queda sano en ~15 s sin tocar la red. `freshclam` desactivado a propósito |
| ~~La carga multipart es incómoda desde Postman~~ | — | **Resuelto**: SEG-3 la recorre entera, y `make subir` hace lo mismo desde la terminal con cualquier tamaño de archivo |
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
