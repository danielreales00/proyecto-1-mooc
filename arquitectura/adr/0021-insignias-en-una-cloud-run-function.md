# ADR-0021 — Emisión de insignias en una Cloud Run function para la Entrega 5

- **Estado:** Propuesto
- **Fecha:** 2026-11-20
- **Requisitos:** `E5-M1` a `E5-M5`, `E5-T3`, `E5-T4`, `CA-07`
- **Reemplaza / reemplazado por:** saca `badge.issue` del worker de ADR-0020.
  Mantiene el registro de trabajos de ADR-0004 y la idempotencia de ADR-0008.

## Contexto

La Entrega 5 pide migrar al menos una tarea del worker a una Cloud Run function
(segunda generación) activada por Pub/Sub mediante Eventarc. La tarea debe
conservar su comportamiento y resultado, el worker debe dejar de ejecutarla y
los reintentos no pueden duplicar sus efectos. Terraform debe gestionar la
función, su código desplegable y su disparador.

Hoy el worker recibe por un solo topic, `mooc-trabajos`, los siete tipos de
trabajo, y elige el manejador con el atributo `tipo` (ADR-0020). Cada trabajo
está registrado en `platform.job_runs`. El ejecutor (`queue.Runner`) lo reclama
de forma atómica antes de correrlo, sube `attempt` en cada reclamo y lo marca
terminado al final.

## Decisión

`badge.issue` pasa a una Cloud Run function en Go que recibe sus trabajos de un
topic propio por Eventarc. El resto de los trabajos sigue en el worker.

### D1 — La tarea: `badge.issue`

| Tarea | Recursos | Duración | Efecto duplicado | Encaje en una función |
| --- | --- | --- | --- | --- |
| **`badge.issue`** | Cloud SQL y Cloud Storage | Menos de 1 s | Dos insignias, visible | Bueno: corta y sin binarios externos |
| `progress.recompute` | Cloud SQL | Menos de 1 s | Un recálculo de más, invisible | Bueno, pero demuestra mal los reintentos |
| `email.send` | Mailpit en `mooc-cola` | Menos de 1 s | Dos correos | Bueno, pero no usa Cloud SQL ni Cloud Storage |
| `media.probe` | `ffprobe` y Cloud Storage | Segundos | Ninguno | Malo: el runtime de Go no trae `ffprobe` |
| `media.scan` | `clamd` y Cloud Storage | Segundos, más la carga de firmas | Ninguno | Malo: más de 1 GiB de firmas en cada arranque en frío |
| `media.transcode_hls` | FFmpeg y Cloud Storage | Hasta 437 s con 4 vCPU | Ninguno | Malo: necesita FFmpeg y CPU dedicada |

`badge.issue` usa los dos almacenes que dibuja el enunciado: escribe la
insignia en Cloud SQL y su imagen en Cloud Storage. Es corta y su idempotencia
ya es doble: el reclamo de `job_runs` y la restricción `UNIQUE (enrollment_id)`
de `badges.badges` (ADR-0008). Además, un efecto duplicado se vería como una
segunda insignia, lo que permite demostrar los reintentos.

### D2 — Un topic por consumidor

El backend enruta por tipo: `badge.issue` se publica en `mooc-insignias` y el
resto sigue en `mooc-trabajos`. El worker quita `badge.issue` de sus
manejadores.

Un disparador de Eventarc entrega todo lo que llega a su topic. Con un solo
topic, la función recibiría los siete tipos y la tarea tendría dos
consumidores. El enrutamiento vive en el adaptador de Pub/Sub, así que los
módulos no cambian. El reaper republica por el mismo adaptador y respeta el
enrutamiento.

### D3 — La función reutiliza el ejecutor y el servicio

La función decodifica el CloudEvent, extrae el mensaje de Pub/Sub y llama al
mismo `queue.Runner` con el mismo `badges.Service`. Solo cambia la entrada.

| Resultado del ejecutor | La función devuelve | Eventarc |
| --- | --- | --- |
| Completado o ya terminado | `nil` | Confirma |
| Otro lo tiene en curso | Error | Reintenta |
| Falló | Error | Reintenta |
| Mensaje ilegible | `nil` | Confirma, porque reintentarlo no lo arregla |

### D4 — Reintentos y límite

El disparador usa `RETRY_POLICY_RETRY`. Eventarc reintenta con espera
exponencial hasta 7 días y el evento no trae el número de entrega. La función
lee `job_runs.attempt` después de reclamar el trabajo: al quinto fallo lo marca
`dead`, como el worker, y devuelve `nil` para cortar los reintentos.

### D5 — Código desplegable gestionado por Terraform

El paquete de la función vive en la raíz del módulo `backend/`, para que el
buildpack de Go lo compile con los paquetes `internal` del backend. Terraform
empaqueta el módulo con `archive_file`, lo sube a un bucket con su hash en el
nombre y despliega la función con el runtime `go126`. Cambiar el código cambia
el hash, y `apply` redespliega.

### D6 — Red y acceso

| Identidad | Permisos |
| --- | --- |
| `mooc-insignias` (la función) | Lee `database-url`, escribe objetos en el bucket `badges` |
| Disparador de Eventarc | `roles/run.invoker` sobre la función |
| `mooc-web` y `mooc-worker` | Publican en `mooc-insignias` |

La función llega a Cloud SQL por su IP privada con salida directa a la VPC
(`direct_vpc_network_interface`), en la subred de Cloud Run. No hay conector.

### D7 — Dependencia nueva

`github.com/GoogleCloudPlatform/functions-framework-go` y, por la firma que
exige, `github.com/cloudevents/sdk-go/v2`. El runtime de Go de Cloud Run
functions los necesita para registrar la función y recibir el CloudEvent. Se
añaden a la lista de ADR-0002 y a `scripts/arquitectura.sh`.

## Alternativas consideradas

| Alternativa | Por qué no |
| --- | --- |
| Un solo topic con un filtro por tipo en el disparador de Eventarc | No está comprobado que un disparador de Pub/Sub filtre por atributos del mensaje. Un topic propio separa a los consumidores sin depender de eso, y el worker deja de recibir la tarea por construcción |
| Filtrar dentro de la función y confirmar lo que no es suyo | La tarea seguiría llegando también al worker. Habría dos consumidores |
| Conector de Acceso a VPC sin servidor | Cobra por hora aunque no haya tráfico. La salida directa no |
| Código de la función en un módulo de Go separado | Tendría que copiar o reemplazar los paquetes `internal` del backend, y el resultado de la tarea podría divergir |
| Subir el código con `gcloud functions deploy` | Queda fuera de Terraform, que debe gestionar el código desplegable |

## Consecuencias

- El worker pierde un tipo de trabajo y gana nada. Su imagen no cambia de
  tamaño.
- La función arranca en frío en la primera insignia después de un rato sin
  uso. Para una tarea asíncrona no importa.
- Cada cambio del módulo `backend/` redespliega la función, aunque no toque las
  insignias. Es el precio de compartir el código.
- Hay dos caminos de reintento: el push de la suscripción para el worker y
  Eventarc para la función. Los dos terminan en el mismo límite de cinco
  intentos porque lo cuenta la base.

## Cómo se verifica

- Prueba unitaria de la función con un CloudEvent armado a mano: emite la
  insignia y una segunda entrega no crea otra.
- En la nube: un estudiante aprueba un curso, la función registra «trabajo
  iniciado» y «trabajo completado», y aparece una insignia. Una copia publicada
  a mano en `mooc-insignias` se confirma sin una segunda insignia.
- El worker no registra ningún `badge.issue` después del despliegue.
