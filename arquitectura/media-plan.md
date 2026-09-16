# Módulo `media` — estado y plan

**Los cuatro pasos están hechos.** La Entrega 1 llevó la carga multipart
reanudable y la verificación en servidor; después se añadieron el escaneo
antimalware, la transcodificación a HLS y la entrega firmada del manifiesto.
Queda la sesión de reproducción por inscripción. Este documento dice qué queda
y con qué evidencia se comprobó lo demás.

Sirve para dos cosas: que el tribunal vea que la ausencia es una decisión con
fecha, y que quien lo implemente no tenga que rehacer el diseño.

## Qué ya existe

| Pieza | Dónde | Estado |
| --- | --- | --- |
| Esquema de datos | `backend/migrations/0002_dominio.up.sql` | **Aplicado**: `media.assets`, `media.uploads`, `media.asset_derivatives`, con sus estados y restricciones |
| Contrato de la API | `backend/openapi/openapi.yaml` | **9 operaciones** validadas, marcadas `x-estado: planificado` |
| Esquemas de la API | idem | `Asset`, `UploadTicket`, `UploadState`, `UploadPartUrl`, `AssetDerivative`, `MediaSession` |
| Puerto de almacenamiento | `internal/adapters/objectstore` | **Funciona**: cliente S3, `PresignGet`, comprobación de salud |
| Buckets | `docker-compose.yml`, servicio `minio-init` | **Creados**: originales, derivados, insignias y cuarentena, con la política pública solo en insignias |
| Decisiones | `adr/0005`, `adr/0011`, `adr/0015` (D2) | Aceptadas |
| Máquina de estados del asset | `disenos/maquinas-de-estado.md` | Definida |
| Catálogo de trabajos | `disenos/trabajos-asincronos.md` | Definido, con sus `job_key` deterministas |
| Infraestructura de workers | `internal/adapters/queue` | **Funciona**: reclamo atómico, reintentos, DLQ, alerta y reaper |

Dicho de otro modo: **la tubería está puesta y probada con `email.send` y
`badge.issue`**. Los trabajos de medios son consumidores nuevos de una
maquinaria que ya funciona, no una maquinaria nueva.

## Qué falta

| Pieza | Trabajo | Riesgo |
| --- | --- | --- |
| `internal/modules/media` | Dominio, servicio, store y HTTP | Bajo: el patrón está establecido en los otros cinco módulos |
| Carga multipart prefirmada | `assets:init`, estado de partes, renovación de URLs, `complete`, `abort` | **Medio**: es la parte con más casos límite |
| Worker `media.probe` | Recalcular SHA-256 leyendo del bucket, detectar MIME real por *magic bytes*, extraer duración y resolución con `ffprobe` | Bajo |
| ~~Worker `media.scan`~~ | **Hecho** | — |
| ~~Worker `media.transcode_hls`~~ | **Hecho** | — |
| ~~Imagen `worker-media`~~ | **Hecha** | — |
| `media-sessions` | Emisión de la credencial de reproducción (ADR-0015, D2) | Bajo: el contrato ya está cerrado |
| Barrido de cargas vencidas | Worker programado `media.sweep_uploads` | Bajo |

## Orden propuesto

Cada paso deja algo demostrable. No hace falta llegar al final para que sume.

### ~~Paso 1 · Carga y verificación~~ — **HECHO**

`assets:init` → subida directa a MinIO con URLs prefirmadas → `GET /upload`
para reanudar → `complete` → worker `media.probe`.

Al terminar este paso se puede demostrar: URLs prefirmadas, **reanudación tras
una interrupción**, verificación de checksum contra lo declarado, detección del
MIME real por los bytes, y rechazo del asset cuando algo no cuadra.

Comprobado en vivo: subida de 2 de 3 partes, consulta de lo que falta, subida
solo de la tercera, completado y verificación. Y los tres negativos: checksum
falso, PDF disfrazado de vídeo, y ambos a cuarentena con el original intacto.

### ~~Paso 2 · Antimalware~~ — **HECHO**

Contenedor de ClamAV con healthcheck, worker `media.scan` en la cola `bulk`,
cuarentena y evento de auditoría. El adaptador habla INSTREAM por TCP sin
añadir dependencias (ADR-0002) y transmite el archivo por trozos de 64 KiB: no
se carga en memoria para escanearlo.

Comprobado en vivo con **EICAR**, el archivo de prueba estándar:

- El asset termina en `infected` con `scan_result: Eicar-Test-Signature`.
- El original queda en `mooc-quarantine` y **desaparece de `mooc-originals`**.
- Hay evento de auditoría `media.infected`.
- **Cero derivados y ningún trabajo de transcodificación**: el escaneo va antes,
  así que lo infectado no llega a FFmpeg.

La colección lo recorre entero en las peticiones `6d`…`6g` de SEG-3. La cadena
de EICAR se arma en tiempo de ejecución desde tres trozos, para que no aparezca
escrita en ningún archivo del repositorio: si estuviera, el antivirus de quien
lo clone pondría la colección en cuarentena.

**Falla cerrado.** Si clamd no responde, el trabajo devuelve error y se
reintenta; el asset se queda en `scanning` y no pasa a transcodificarse. Un
escáner caído no se convierte en un permiso de paso.

### ~~Paso 3 · Transcodificación~~ — **HECHO**

Imagen `worker-media` con FFmpeg, consumiendo **solo la cola `bulk`**: la
transcodificación satura CPU durante minutos y no debe retrasar un correo de
verificación. El worker normal no la consume, así que tampoco puede recibir un
trabajo que su imagen no sabe ejecutar.

`media.probe` encola `media.transcode_hls` en la misma transacción que deja el
asset `clean`, y lo publica tras el commit (outbox, ADR-0004).

Comprobado en vivo:

- Un original de 640×360 genera **una sola variante, 360p**: anunciar 720p
  sería inventar información. La regla vive en `VariantesPara` y tiene prueba
  unitaria.
- Reejecutar el trabajo —devolviéndolo a la cola y dejando que lo recupere el
  reaper— deja **los mismos tres derivados**, no seis (CA-03).
- El original sigue intacto en `mooc-originals` (CA-02).
- Un MP4 truncado deja el asset en `failed` con el motivo, **sin agotar
  reintentos ni ensuciar la DLQ**: un archivo roto no es una avería del
  sistema. `jobs_dead_letter_total{type="media.transcode_hls"}` sigue en cero.

### ~~Paso 4 · Entrega~~ — **HECHO para el asset**

El manifiesto maestro lo sirve la API autenticada; las playlists de variante van
sin sesión, autorizadas por una **credencial de reproducción** de 30 minutos que
el propio maestro incrusta en la URL, y cada segmento sale reescrito como URL
firmada de 15 minutos hacia MinIO. El bucket de derivados sigue privado.

Hizo falta porque un reproductor pide cada documento del manifiesto por su
cuenta y **no envía cabeceras de autorización a los sub-recursos**: si los
segmentos se dejan como nombres relativos, se resuelven contra una URL firmada,
salen sin firma y el almacén responde 403.

Queda pendiente `POST /enrollments/{id}/media-sessions`, que es la misma
credencial con alcance de inscripción en vez de asset: lo que necesita el
estudiante para reproducir desde el árbol del curso (ADR-0015, D2).

### Paso 4b · Entrega desde la inscripción

`media-sessions` y la entrega firmada del manifiesto. El contrato ya está
cerrado desde el ADR-0015, así que es cablear, no diseñar.

## Lo que hay que tener presente al implementarlo

**El original no se toca nunca.** Ni al transcodificar, ni al reintentar. Es
condición de aceptación (CA-02), y la forma de garantizarlo es que las claves
de objeto sean deterministas: reintentar escribe en el mismo sitio.

**El MIME lo decide el servidor.** Nunca la extensión, nunca el
`Content-Type` que declare el cliente. Lo mismo con el checksum: el cliente
declara, el servidor recalcula leyendo del bucket.

**Sin *upscaling*.** Solo se generan variantes cuya altura sea menor o igual a
la del original. El segmento 4 de la demostración lo nombra explícitamente.

**Los temporales de FFmpeg mueren con el trabajo.** Van al `tmpfs` del
contenedor y se borran con `defer`, incluso al fallar: los workers no acumulan
estado (ADR-0001).

**La escalera completa alarga la grabación.** Para la demostración conviene
recortar a 360p y 720p, y clips de treinta segundos. Está anotado como decisión
abierta en `pendientes.md`.

## Qué criterios desbloquea

| Criterio | Hoy | Con `media` |
| --- | --- | --- |
| CE-04 · Multimedia y distribución | Sin cubrir | Cubierto salvo CDN, que va con GCP |
| CE-01 · Arquitectura | Cubierto | Refuerza: transcodificación como carga real de los workers |
| CE-07 · Calidad operativa | Parcial | Refuerza: `job_duration_seconds` da el costo por minuto que el §11 pide medir |

## Por qué no entró en la Entrega 1

Con el plazo encima, la decisión fue **anchura antes que profundidad**: el
profesor pidió evidenciar *el flujo de trabajo completo*, y un camino que
atraviesa autoría, publicación, inscripción, evaluación, progreso e insignia
demuestra más que dos módulos perfectos y siete ausentes.

`media` es el módulo más grande de los que faltaban y el que más piezas nuevas
introduce a la vez —multipart de S3, ClamAV, FFmpeg—, así que era también el de
mayor riesgo de romper lo que ya funcionaba. Empezarlo y no terminarlo habría
sido peor que no empezarlo.
