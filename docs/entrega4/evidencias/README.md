# Evidencias. Entrega 4

Recogidas el 7 y el 8 de noviembre de 2026 (UTC) sobre el despliegue de
`docs/entrega4/despliegue.md`, imágenes `0f1c56adc00f`. Cada archivo de esta
carpeta sale de una orden que se puede repetir. La orden está junto a cada
tabla.

## 1. Servicios desplegados

| Servicio | URL | Acceso |
| --- | --- | --- |
| Frontend | <https://mooc-frontend-568765476410.us-central1.run.app> | Público |
| Backend | <https://mooc-backend-568765476410.us-central1.run.app> (`/api/v1`, `/docs`, `/readyz`) | Público |
| Workers | `https://mooc-workers-568765476410.us-central1.run.app` | Interno. Desde Internet responde 404 |

La configuración leída de Cloud Run está en [`servicios.yaml`](servicios.yaml)
(`make gcloud ARGS="run services describe mooc-<servicio> --region us-central1"`).

| Servicio | Mínimo | Máximo | CPU | Memoria | Concurrencia | Tiempo máximo | Cuenta |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `mooc-frontend` | 0 | 3 | 1 | 512 MiB | 80 | 30 s | `mooc-frontend` |
| `mooc-backend` | 0 | 3 | 1 | 2 GiB | 20 | 60 s | `mooc-web` |
| `mooc-workers` | 0 | 3 | 4 | 4 GiB | 1 | 600 s | `mooc-worker` |

`servicios.yaml` se capturó antes de subir los workers a 4 vCPU y muestra 2. El
cambio y su motivo están en la sección 5.

## 2. Funcionamiento de punta a punta

| Prueba | Resultado | Archivo |
| --- | --- | --- |
| Colección de Postman contra el backend (`make postman-nube`) | 72 peticiones, 146 aserciones, 0 fallos | [`postman-nube.log`](postman-nube.log) |
| Recorrido de navegador contra el frontend (`make frontend-e2e-nube`) | Pasa. Profesor publica un curso, estudiante se inscribe, ve la lección, la imagen, el video HLS y el archivo | [`frontend/`](frontend/), capturas y [registro](frontend/nube-20261108T0003Z.log) |

## 3. Una tarea de punta a punta

La aprobación de una inscripción publica `badge.issue`. Los tres registros
comparten `job_key` y `trace_id` ([`reentrega-insignia.tsv`](reentrega-insignia.tsv)):

| UTC | Servicio | Registro |
| --- | --- | --- |
| 00:17:51.295 | `mooc-backend` | trabajo publicado |
| 00:17:51.365 | `mooc-workers` | trabajo iniciado |
| 00:17:51.483 | `mooc-workers` | trabajo completado |

Del publicado al completado pasan 188 ms. La orden está en la sección 7 de la
guía de despliegue.

## 4. Reentrega sin efectos duplicados

A las 00:21:08 se publicó en `mooc-trabajos` una copia del mismo `badge.issue`,
con su misma `job_key`. Pub/Sub la entregó al worker, que respondió 204 en 9 ms
y registró «entrega repetida de un trabajo terminado; se confirma sin
repetirlo» con `estado=succeeded`. No hubo un segundo «trabajo iniciado», así
que la insignia no se emitió dos veces.

La prueba de video de la sección 5 dejó otro caso. Tres transcodificaciones se
cortaron a los 600 s, volvieron a la cola y terminaron en la entrega
siguiente. Cada video quedó `ready` una sola vez.

## 5. Escalamiento con carga controlada

Las tablas salen de Cloud Monitoring con
`make escalado-cloudrun DESDE=.. HASTA=..`, por minuto. Las instancias cuentan
las activas y las ociosas. Cloud Run mantiene una instancia ociosa unos
minutos antes de retirarla.

### Backend, solicitudes concurrentes

k6 desde una sola máquina pidió el catálogo anónimo a tasa constante
([`k6-backend.txt`](k6-backend.txt)). Tabla completa en
[`escalado-backend.md`](escalado-backend.md).

| UTC | Instancias del backend | Peticiones/s | CPU p90 |
| --- | --- | --- | --- |
| 00:22 | 1 | 20 | 4 % |
| 00:24 | 1 | 601 | 39 % |
| 00:25 | 2 | 600 | 54 % |
| 00:27 | 2 | 594 | 73 % |
| 00:29 | 3 | 595 | 33 % |
| 00:32 | 3 | 20 | 10 % |
| 00:44 | 3 | 1.126 | |

Cloud Run escala el backend por CPU, con objetivo de 60 %. A 600 peticiones
por segundo la segunda instancia llegó al minuto y la tercera a los cinco, y
con tres la CPU bajó al 33 %. Dentro del servidor la mediana fue 3 ms y el p95
5,5 ms. Los 164 fallos de esa fase (0,06 %) fueron plazos de 20 s vencidos en
el cliente. Cloud Run no registró ninguna respuesta 5xx.

La fase de 1.500 peticiones por segundo no sumó instancias, porque el máximo
es 3. Los fallos de esa fase son del generador, que no sostuvo las conexiones
desde una red doméstica. Dentro del servidor la latencia siguió en 3 ms de
mediana. No mide la capacidad del backend.

### Workers, mensajes pendientes

k6 subió cinco videos del perfil C a la vez
([`k6-workers.txt`](k6-workers.txt)). Cada video encadena verificación,
escaneo y transcodificación. Tabla completa en
[`escalado-workers.md`](escalado-workers.md).

| UTC | Instancias de workers | CPU p90 | Mensajes pendientes | Antigüedad del más viejo |
| --- | --- | --- | --- | --- |
| 00:52 | 1 | 47 % | 0 | 0 s |
| 00:53 | 3 | 75 % | 0 | 0 s |
| 00:54 | 3 | 96 % | 7 | 13 s |
| 00:57 | 3 | 100 % | 3 | 195 s |
| 01:05 | 3 | 100 % | 2 | 654 s |
| 01:13 | 5 | 100 % | 3 | 1.134 s |
| 01:16 | 3 | 81 % | 2 | 1.309 s |
| 01:21 | 3 | 0 % | 3 | 1.549 s |

Con concurrencia 1, cada worker atiende un trabajo. Tres trabajos corrieron a
la vez y el resto esperó en la suscripción, que es la cifra de mensajes
pendientes. Pub/Sub recibió «no hay instancia disponible» al intentar
entregarlos y los reintentó con espera creciente. Los cinco videos quedaron
`ready` a las 01:20:44, sin fallos de HTTP en k6.

Las cinco instancias de las 01:13 se deben al cambio de revisión. La revisión
anterior terminaba sus transcodificaciones mientras la nueva arrancaba. Cloud
Run puede superar el máximo de forma temporal en esa transición.

La prueba encontró un defecto de dimensionamiento. Con 2 vCPU, un video del
perfil C tardó 519 s y otros dos llegaron a 600 s, el plazo de confirmación,
y se cortaron. A las 01:12 los workers pasaron a 4 vCPU con un `apply`, y las
transcodificaciones siguientes tardaron de 286 a 437 s
([`transcodificaciones.txt`](transcodificaciones.txt)). Los trabajos cortados
volvieron a la cola y terminaron en la revisión nueva.

## 6. Acceso

[`pubsub-e-iam.yaml`](pubsub-e-iam.yaml) recoge la suscripción y las
políticas de IAM leídas del proyecto.

| Control | Valor |
| --- | --- |
| Suscripción | Push a `/pubsub/push` con token OIDC de `mooc-pubsub-push`, audiencia la URL de los workers |
| Plazo, reintentos, entregas | 600 s, de 10 a 600 s, 5 y después `mooc-trabajos-muertos` |
| Invocan `mooc-workers` | Solo `mooc-pubsub-push` y `mooc-programador` |
| Publican en `mooc-trabajos` | Solo `mooc-web` y `mooc-worker` |
| Invocan frontend y backend | `allUsers`, porque son la aplicación pública |
| Workers desde Internet | 404, por ingreso interno |
| Secretos | `database-url` y los demás en Secret Manager, referenciados por Cloud Run. Ninguno en el repositorio, la imagen o estos archivos |
