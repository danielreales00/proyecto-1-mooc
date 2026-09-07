# Módulo `media` — estado y plan

Multimedia es la pieza del alcance mínimo que **no** entra en la Entrega 1. No
es un olvido: está diseñada, contratada y con su esquema aplicado. Lo que falta
es el código, y este documento dice exactamente cuál y en qué orden.

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
| Worker `media.scan` | ClamAV por INSTREAM, cuarentena y auditoría | **Medio**: el contenedor tarda en arrancar y descargar firmas |
| Worker `media.transcode_hls` | FFmpeg, escalera sin *upscaling*, póster, original intacto | **Alto**: es el que más tarda en afinar |
| Imagen `worker-media` | Etapa nueva del Dockerfile con FFmpeg | Bajo |
| `media-sessions` | Emisión de la credencial de reproducción (ADR-0015, D2) | Bajo: el contrato ya está cerrado |
| Barrido de cargas vencidas | Worker programado `media.sweep_uploads` | Bajo |

## Orden propuesto

Cada paso deja algo demostrable. No hace falta llegar al final para que sume.

### Paso 1 · Carga y verificación — desbloquea SEG-3 casi entero

`assets:init` → subida directa a MinIO con URLs prefirmadas → `GET /upload`
para reanudar → `complete` → worker `media.probe`.

Al terminar este paso se puede demostrar: URLs prefirmadas, **reanudación tras
una interrupción**, verificación de checksum contra lo declarado, detección del
MIME real por los bytes, y rechazo del asset cuando algo no cuadra.

Es el paso con mejor relación entre esfuerzo y criterio ganado.

### Paso 2 · Antimalware — completa SEG-3

Contenedor de ClamAV con `healthcheck`, worker `media.scan`, cuarentena y
evento de auditoría. Se prueba con el archivo **EICAR**, que es inofensivo y
detectado por cualquier antivirus.

### Paso 3 · Transcodificación — desbloquea SEG-4 con medios

Imagen `worker-media` con FFmpeg y el trabajo `media.transcode_hls`. Aquí es
donde SEG-4 pasa de demostrarse con `badge.issue` a demostrarse con lo que el
enunciado describe: HLS sin *upscaling*, doble entrega idempotente, backoff,
DLQ y reencolado con la misma clave.

### Paso 4 · Entrega — completa SEG-5

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
