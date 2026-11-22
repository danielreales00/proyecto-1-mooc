# Análisis serverless. Entrega 5

Qué partes de la plataforma ya son serverless, cuáles podrían serlo, qué
servicio las sustituiría y con qué beneficio y qué límite. Cierra con la
justificación de la tarea migrada en esta entrega. La implementación está en
[`arquitectura.md`](arquitectura.md) y la decisión en el
[ADR-0021](../../arquitectura/adr/0021-insignias-en-una-cloud-run-function.md).

Un componente es serverless aquí cuando cumple tres condiciones. No hay
máquinas que administrar. Escala solo, hasta cero cuando no hay uso. Se paga
por uso y no por capacidad reservada.

## 1. Lo que ya es serverless

| Componente | Servicio | Desde | Escala a cero |
| --- | --- | --- | --- |
| Interfaz web | Cloud Run, `mooc-frontend` | Entrega 4 | Sí |
| API | Cloud Run, `mooc-backend` | Entrega 4 | Sí |
| Workers de medios y correo | Cloud Run, `mooc-workers` | Entrega 4 | Sí |
| Emisión de insignias | Cloud Run functions, `mooc-insignias` | Entrega 5 | Sí |
| Mensajería entre backend y workers | Pub/Sub | Entrega 4 | No aplica: cobra por mensaje |
| Entrega de eventos a la función | Eventarc | Entrega 5 | No aplica: cobra por evento |
| Reaper y poda periódicos | Cloud Scheduler | Entrega 4 | No aplica: cobra por trabajo programado |
| Archivos, HLS e imágenes de insignias | Cloud Storage | Entrega 2 | No aplica: cobra por GB guardado |
| Secretos | Secret Manager | Entrega 2 | No aplica |
| Compilación de la función | Cloud Build | Entrega 5 | No aplica: cobra por minuto de compilación |

## 2. Lo que todavía no es serverless

| Componente | Servicio actual | Por qué no es serverless |
| --- | --- | --- |
| Base de datos | Cloud SQL, `db-g1-small` | Es gestionada, pero cobra por la instancia encendida haya consultas o no. Se apaga a mano entre sesiones |
| Sesiones, caché y límites de tasa | Redis en la máquina `mooc-cola` | Una máquina virtual con su disco, que se enciende y apaga a mano |
| Correo | Mailpit en `mooc-cola` | Es un buzón de pruebas, no un proveedor. Vive en la misma máquina |
| Salida a Internet de la máquina | Cloud NAT | Cobra por hora y por pasarela, por eso se destruye al apagar |

## 3. Oportunidades

Una fila por oportunidad. Solo la primera se implementa en esta entrega. Las
demás se documentan.

| # | Componente | Servicio propuesto | Beneficio | Limitaciones | Decisión |
| --- | --- | --- | --- | --- | --- |
| O1 | Tarea `badge.issue` del worker | Cloud Run functions con disparador de Pub/Sub por Eventarc | Una pieza pequeña con su propio escalado, su cuenta de servicio y su despliegue. El worker queda solo para medios y correo | Arranque en frío en la primera insignia tras un rato sin uso. Dos caminos de reintento que mantener | **Implementada** |
| O2 | Tarea `progress.recompute` | Cloud Run functions, igual que O1 | Saca otro trabajo corto del worker | Solo toca la base, y su efecto duplicado no se ve. Aporta poco frente a O1 | Documentada |
| O3 | Tarea `email.send` y Mailpit | Cloud Run functions con un proveedor de correo gestionado | Correo real, sin la máquina de la cola | Exige una cuenta externa, un dominio verificado y su costo. Fuera del alcance del curso | Documentada |
| O4 | Tarea `media.transcode_hls` | Transcoder API | Sin FFmpeg en la imagen ni CPU dedicada. Produce HLS y está pensada para trabajos asíncronos. Quita el riesgo de los 600 s del plazo de Pub/Sub, porque el trabajo pasa a ser de Google | Cambian los perfiles de salida y el adaptador de transcodificación. Su costo por uso no se midió. Hay que esperar el final del trabajo con un evento o consultándolo | Documentada |
| O5 | Tarea `media.scan` | Seguir en Cloud Run | Ninguno nuevo | La solución de referencia de Google para escanear archivos de Cloud Storage usa ClamAV en Cloud Run, que es lo que ya hacemos. Una función cargaría más de 1 GiB de firmas en cada arranque en frío | Se queda donde está |
| O6 | Reaper y poda | Trabajos de Cloud Run disparados por Cloud Scheduler | Ya no dependen de una ruta HTTP del worker. Una tarea de un trabajo puede durar hasta 168 horas | Hoy duran segundos y ya son serverless. El cambio no se justifica | Documentada |
| O7 | Sesiones, caché y límites de tasa en Redis | Memorystore for Redis | Sin máquina que administrar. Alta disponibilidad opcional | Es gestionado pero no serverless: cobra por la capacidad reservada mientras existe, haya tráfico o no | Documentada |
| O8 | Sesiones | Firestore | Serverless de verdad: cobra por lectura y escritura, sin capacidad reservada | Cambia el adaptador de sesiones. Más latencia que Redis en cada petición autenticada. Los límites de tasa necesitan contadores atómicos que habría que rehacer | Documentada |
| O9 | Interfaz web | Cloud Storage con Cloud CDN, o Firebase Hosting | Archivos estáticos servidos desde el borde, sin contenedor | `/config.js` se genera al arrancar con la URL de la API, así que pasaría a generarse al construir | Documentada |
| O10 | Entrega de los segmentos HLS | Cloud CDN con cookies firmadas | Video más cerca del estudiante, menos salida desde Cloud Storage | Ya previsto en ADR-0015, D2. Exige un balanceador, que cobra por hora | Documentada |
| O11 | Base de datos | Ninguna alternativa serverless | No aplica | Las invariantes del dominio, como versiones inmutables, una insignia por inscripción y claves únicas, viven en restricciones de PostgreSQL. Una base documental obligaría a rehacerlas en la aplicación | Se queda en Cloud SQL |

## 4. Servicios para ejecutar un trabajo del worker

Las cuatro formas de ejecutar en Google Cloud un trabajo que llega por
Pub/Sub.

| | Cloud Run, servicio con push | Cloud Run functions | Cloud Run, trabajo | Transcoder API |
| --- | --- | --- | --- | --- |
| Cómo llega el trabajo | Suscripción push de Pub/Sub | Disparador de Eventarc sobre un topic | Se ejecuta, no recibe mensajes | Se le envía un trabajo por API |
| Qué se despliega | Una imagen de contenedor | El código fuente. Google lo compila | Una imagen de contenedor | Nada: es un servicio de Google |
| Binarios propios, como FFmpeg o ClamAV | Sí | No, solo el runtime del lenguaje | Sí | No aplica |
| Tiempo máximo | 60 minutos por petición, limitado a 600 s por el plazo de Pub/Sub | Lo fija la función. La nuestra, 60 s | 168 horas por tarea | No aplica |
| Reintentos | Los de la suscripción | Los del disparador | Los del trabajo | Los de quien lo envía |
| En la plataforma | `mooc-workers` | `mooc-insignias` | `mooc-migrate`, `mooc-seed` | Oportunidad O4 |

Los tiempos máximos de Cloud Run salen de su documentación de [tiempo máximo
de petición](https://cloud.google.com/run/docs/configuring/request-timeout) y
de [tiempo máximo de tarea](https://cloud.google.com/run/docs/configuring/task-timeout).
El de la función es el que fija `infra/environments/entrega5/terraform.tfvars`.

Una función encaja con un trabajo corto, sin binarios propios y que se
dispara por un evento. Un servicio de Cloud Run encaja con un trabajo que
necesita FFmpeg o ClamAV en la imagen. Un trabajo de Cloud Run encaja con lo
que se ejecuta una vez por despliegue, como las migraciones.

## 5. La tarea elegida: `badge.issue`

| Tarea | Recursos | Duración | Binarios propios | Efecto duplicado | Encaje en una función |
| --- | --- | --- | --- | --- | --- |
| **`badge.issue`** | Cloud SQL y Cloud Storage | Menos de 1 s | No | Dos insignias, visible | **Bueno** |
| `progress.recompute` | Cloud SQL | Menos de 1 s | No | Un recálculo de más, invisible | Bueno, pero demuestra mal los reintentos |
| `email.send` | Mailpit en `mooc-cola` | Menos de 1 s | No | Dos correos | Bueno, pero no usa Cloud SQL ni Cloud Storage |
| `media.probe` | Cloud Storage | Segundos | `ffprobe` | Ninguno | Malo: el runtime no trae `ffprobe` |
| `media.scan` | Cloud Storage | Segundos | `clamd` y sus firmas | Ninguno | Malo: arranque en frío con más de 1 GiB de firmas |
| `media.transcode_hls` | Cloud Storage | Hasta 437 s con 4 vCPU | FFmpeg | Ninguno | Malo: necesita FFmpeg y CPU dedicada |

`badge.issue` es la mejor candidata por cuatro razones:

1. **Usa los dos almacenes.** Escribe la insignia en Cloud SQL y su imagen en
   Cloud Storage, el mismo recorrido que dibuja el componente pedido.
2. **No necesita nada fuera del runtime de Go.** La imagen de la insignia es un
   SVG que se dibuja con la biblioteca estándar
   (`backend/internal/modules/badges/imagen.go`).
3. **Su idempotencia ya era doble.** El reclamo atómico en `platform.job_runs`
   impide ejecutarla dos veces, y la restricción `UNIQUE (enrollment_id)` de
   `badges.badges` impide dos filas aunque el reclamo fallara (ADR-0008).
4. **Un duplicado se vería.** Sería una segunda insignia para la misma
   inscripción, que rompe la invariante 5 del proyecto. Eso hace que la prueba
   de reintentos demuestre algo.

## 6. Qué se conservó

La función no reimplementa la tarea. Llama al mismo `badges.Service` con el
mismo ejecutor (`queue.Runner`) que usaba el worker, así que el comportamiento
y el resultado son los mismos. Solo cambia por dónde llega la tarea.

| Antes, en el worker | Ahora, en la función |
| --- | --- |
| Topic `mooc-trabajos`, suscripción push | Topic `mooc-insignias`, disparador de Eventarc |
| `POST /pubsub/push`, responde 204, 409 o 500 | CloudEvent, devuelve `nil` o un error |
| Quinta entrega fallida: `dead`, por el `deliveryAttempt` de Pub/Sub | Quinto reclamo fallido: `dead`, contado en `job_runs.attempt` |
| Reclamo atómico, latido, `succeeded` | Lo mismo, el mismo código |

La evidencia está en [`evidencias/`](evidencias/README.md). La insignia emitida
por la función en la nube, una reentrega confirmada sin repetirla, y el worker
sin ningún `badge.issue`.
