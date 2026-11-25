# Evidencias. Entrega 5

Recogidas el 21 de noviembre de 2026 (UTC) sobre el despliegue de
[`despliegue.md`](../despliegue.md), con las imágenes `a02330ee1485`. Cada
archivo sale de una orden que se puede repetir. La orden está junto a cada
apartado.

## 1. Despliegue con Terraform

El entorno `infra/environments/entrega5` se desplegó solo con `init`, `plan` y
`apply`, sobre el estado heredado de `entrega4`. Después del último `apply`, un
plan con `-detailed-exitcode` no encuentra diferencias entre la configuración y
la nube ([`terraform-plan-final.txt`](terraform-plan-final.txt)). La salida del
`apply` que creó la función está en
[`terraform-apply-funcion.txt`](terraform-apply-funcion.txt).

```bash
make tf ENTORNO=entrega5 ARGS="plan -detailed-exitcode"
```

| Qué | Resultado |
| --- | --- |
| Recursos creados en el despliegue | 22: las tres APIs, el topic, el código, la función, el disparador, tres cuentas de servicio, sus permisos y la NAT del encendido |
| Recursos destruidos | Ninguno con datos. Solo los intentos fallidos de la función y su código, y la NAT al apagar |
| Migraciones | Corrieron dentro del `apply`, con `run_execution_token` |
| Plan final | «No changes», salida 0 |

Los tropiezos del primer `apply` y su corrección están en
[`alcance-entrega-5.md`](../../../arquitectura/alcance-entrega-5.md#hallazgos-del-primer-despliegue).

## 2. La función desplegada

[`funcion.yaml`](funcion.yaml) es la función leída de Google Cloud.

```bash
make gcloud ARGS="functions describe mooc-insignias --region us-central1"
```

| Campo | Valor |
| --- | --- |
| Estado | `ACTIVE` |
| Runtime y punto de entrada | `go126`, `EmitirInsignia` |
| Instancias | De 0 a 3, una entrega a la vez |
| Cuenta de servicio | `mooc-insignias` |
| Ingreso | Interno |
| Secreto | `DATABASE_URL`, por referencia a `database-url` en Secret Manager |
| Disparador | `google.cloud.pubsub.topic.v1.messagePublished` sobre `mooc-insignias`, con reintentos, cuenta `mooc-eventarc` |

## 3. Una tarea de Pub/Sub a la función

La colección de Postman contra el backend desplegado llevó a un estudiante a
aprobar un curso ([`postman-nube.log`](postman-nube.log)). 72 peticiones, 146
aserciones, 0 fallos. SEG-8 encuentra la insignia de esa inscripción.

La insignia, seguida en los registros
([`tarea-y-reentrega.tsv`](tarea-y-reentrega.tsv)):

| UTC | Servicio | Registro |
| --- | --- | --- |
| 23:54:05.336 | `mooc-backend` | trabajo publicado, en el topic `mooc-insignias` |
| 23:54:06.826 | `mooc-insignias` | trabajo iniciado |
| 23:54:07.036 | `mooc-insignias` | trabajo completado |

```bash
make gcloud ARGS="logging read 'log_id(\"run.googleapis.com/stdout\") AND jsonPayload.type=\"badge.issue\"' \
  --freshness 1h --order asc \
  --format='value(timestamp,resource.labels.service_name,jsonPayload.message,jsonPayload.job_key,jsonPayload.topic)'"
```

`mooc-workers` no registra ningún `badge.issue`: el worker ya no atiende la
tarea.

## 4. Reintentos sin duplicar efectos

A las 23:55:14 se publicó a mano en `mooc-insignias` una copia del mismo
trabajo, con su misma `job_key`. Eventarc la entregó a la función, que registró
a las 23:55:17 «entrega repetida de un trabajo terminado; se confirma sin
repetirlo» y no un segundo «trabajo iniciado». La insignia se emitió una sola
vez.

```bash
K=badge.issue:01a20086-718c-7266-8bc9-58810784d03a
make gcloud ARGS="pubsub topics publish mooc-insignias \
  --message='{\"job_key\":\"$K\",\"trace_id\":\"reentrega-entrega5\"}' \
  --attribute=tipo=badge.issue,cola=critical,job_key=$K"
```

## 5. La misma prueba en local

`make prueba-funcion` repite el recorrido sin la nube. Levanta el emulador de
Pub/Sub con los dos topics, la API con el enrutamiento, el worker y la función
con el mismo marco que en Cloud Run functions. Entrega el evento a la función
dos veces, como lo haría Eventarc.

| Comprobación | Resultado |
| --- | --- |
| Recorrido hasta la insignia | 56 peticiones, 114 aserciones, 0 fallos |
| Primera entrega a la función | Insignia emitida |
| Segunda entrega del mismo evento | Confirmada sin repetirla |
| Worker | Ningún `badge.issue` iniciado |
