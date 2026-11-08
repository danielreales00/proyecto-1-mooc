# ADR-0020 — Cloud Run y Cloud Pub/Sub para la Entrega 4

- **Estado:** Aceptado
- **Fecha:** 2026-11-07
- **Requisitos:** `RT-01`, `RT-06`, `CE-01`, `CA-03`
- **Reemplaza / reemplazado por:** sustituye el transporte de ADR-0004 en la
  nube y el despliegue de ADR-0019. Mantiene el registro de trabajos en
  PostgreSQL y la idempotencia de ADR-0008.

## Contexto

La Entrega 4 despliega la plataforma en tres servicios de Cloud Run (frontend,
backend y workers), con un máximo de tres instancias cada uno, y sustituye la
mensajería previa por un topic de Pub/Sub con una suscripción push autenticada
hacia los workers. La confirmación de un mensaje debe llegar después de
completar y persistir la tarea, y una reentrega no puede duplicar sus efectos.

La aplicación ya registra cada trabajo en `platform.job_runs` dentro de la
transacción que lo origina y lo publica después del commit (ADR-0004). El
worker lo reclama de forma atómica antes de ejecutarlo y anota el resultado
(ADR-0008). Los módulos publican por una interfaz y no conocen asynq.

Cloud Run solo asigna CPU mientras atiende una petición. Un worker que consume
de una cola en un bucle, como el de asynq, no funciona ahí.

## Decisión

El transporte de los trabajos pasa a ser un adaptador elegido con
`QUEUE_BACKEND`. Con `pubsub`, la API publica en un topic y la suscripción push
entrega cada mensaje al worker como una petición HTTP.

### D1 — Dos transportes, un ejecutor

`queue.Publicador` tiene dos implementaciones: asynq sobre Redis, que siguen
usando el entorno local, el CI y la Entrega 3, y `adapters/pubsub`. El núcleo
del worker, `Runner.Ejecutar`, no depende del transporte: reclama, late y
registra igual en los dos casos.

### D2 — La respuesta HTTP es la confirmación

`POST /pubsub/push` responde según el resultado de la ejecución.

| Resultado | Respuesta | Qué hace Pub/Sub |
| --- | --- | --- |
| Completado y persistido | 204 | Confirma el mensaje |
| Ya terminado por otra entrega | 204 | Confirma sin repetir el trabajo |
| Otro worker lo tiene en curso | 409 | Lo entrega de nuevo con backoff |
| Falló | 500 | Lo entrega de nuevo con backoff |
| Mensaje ilegible o de tipo desconocido | 204 | Lo descarta y queda en el log |

Que un trabajo en curso devuelva 409 en lugar de confirmarse es deliberado. Si
el worker que lo tenía muere, su latido vence a los 90 s y la siguiente entrega
lo reclama. Confirmar la copia habría perdido el trabajo.

### D3 — Tiempos y mensajes muertos

| Parámetro | Valor | Por qué |
| --- | --- | --- |
| Plazo de confirmación | 600 s | Es el máximo de Pub/Sub para push. La transcodificación más larga tiene que caber dentro |
| Tiempo máximo de solicitud del worker | 600 s | Igual al plazo: Cloud Run no corta antes de que Pub/Sub reintente |
| Reintentos | De 10 a 600 s, exponencial | Pub/Sub espera más tras cada fallo |
| Entregas máximas | 5 | En la quinta, el worker marca el trabajo `dead` en la base y Pub/Sub manda el mensaje al topic de muertos |

### D4 — Un servicio de workers con FFmpeg y ClamAV

La imagen de workers de Cloud Run trae el binario del worker, FFmpeg y ClamAV
con sus firmas. `clamd` arranca antes que el worker en el mismo contenedor. Es
un contenedor de aplicación por instancia, y la concurrencia es 1 porque FFmpeg
ocupa todos los núcleos.

Cada instancia tiene 4 vCPU. Con 2, como las máquinas de la Entrega 2, un video
del perfil C tardó 519 s en Cloud Run y otros dos superaron los 600 s y se
cortaron. En la máquina virtual el mismo video tardaba 196 s. Con 4 vCPU tardó
de 286 a 437 s el 8 de noviembre, dentro del plazo de confirmación
(`docs/entrega4/evidencias/transcodificaciones.txt`).

### D5 — Acceso

| Identidad | Permisos |
| --- | --- |
| `mooc-backend` | Publica en el topic, lee sus secretos, firma URL sobre sí misma, objetos según bucket |
| `mooc-workers` | Publica en el topic (los trabajos encolan sus sucesores), lee sus secretos, objetos según bucket |
| `mooc-pubsub-push` | `roles/run.invoker` sobre el servicio de workers. Es la identidad del token OIDC de la suscripción |
| `mooc-programador` | `roles/run.invoker` sobre el servicio de workers, para el reaper y la poda |

El servicio de workers tiene ingreso interno y nadie más puede invocarlo. El
frontend no tiene permisos sobre ningún recurso.

### D6 — El reaper y la poda los dispara Cloud Scheduler

`POST /tareas/reaper` cada 5 minutos y `POST /tareas/podar` cada hora. Con
Pub/Sub, un trabajo `queued` puede estar esperando turno sin estar perdido, así
que el reaper solo lo republica tras 10 minutos (`REAPER_EDAD_MINIMA`). Una
copia que llegue después de terminado el trabajo se confirma sin repetirlo.

### D7 — Frontend y backend en orígenes distintos

El frontend recibe la URL del backend por variable de entorno y la sirve en
`/config.js`. El backend responde CORS solo a los orígenes de `CORS_ORIGINS`. La
sesión viaja en `Authorization`, sin cookies, así que no hacen falta
credenciales CORS.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Suscripción pull con un worker siempre encendido | Sin plazo de confirmación por petición | Necesita CPU fuera de las peticiones y una instancia mínima permanente | La entrega pide push autenticado |
| Confirmar las copias de un trabajo en curso | Menos reentregas | Si el worker que lo tenía muere, el trabajo se pierde | El 409 cuesta un reintento y no pierde nada |
| ClamAV como contenedor auxiliar | Imagen del worker más liviana | Dos contenedores por instancia | La entrega pide un contenedor de aplicación por instancia |
| ClamAV como cuarto servicio | Escala aparte | Un servicio más de los tres que pide la entrega | Fuera de la arquitectura pedida |
| Memorystore para sesiones | Administrado | ~1,2 USD al día frente a la máquina que ya existe | Pub/Sub sustituye la mensajería, no las sesiones |
| Frontend que reenvía `/api` al backend | Sin CORS | El frontend atendería todo el tráfico de la API | El diagrama de la entrega conecta el frontend con la API directamente |

## Consecuencias

- La API publica y responde sin esperar la ejecución. El estado del trabajo se
  consulta en `job_runs`, o en el recurso que lo originó, como el estado de un
  archivo.
- El stack local no cambia: sigue con asynq y Redis, y `make ci` prueba el
  dominio igual que antes.
- Un worker frío tarda en arrancar porque `clamd` carga sus firmas. El primer
  mensaje tras un periodo sin carga espera ese arranque.
- Sin una instancia mínima, el primer usuario tras un periodo sin carga espera
  el arranque del backend.
- Redis sigue siendo un punto único de falla para las sesiones, y ya no para
  los trabajos.

## Cómo se verifica

- `make rapido`: el adaptador compila y el ADR-0002 aprueba la dependencia.
- Prueba local con el emulador de Pub/Sub: un trabajo publicado llega al worker
  por push, se ejecuta una vez y una reentrega se confirma sin repetirlo.
- En Cloud Run: el registro de una tarea muestra su publicación en el backend
  y su inicio y fin en el worker con la misma `job_key`.
