# Guía de despliegue. Entrega 4

Cómo desplegar la plataforma en Cloud Run con Pub/Sub, desde un clon del
repositorio. Todo corre en contenedores: ni `gcloud` ni `terraform` se instalan
en la máquina. Las decisiones están en el
[ADR-0020](../../arquitectura/adr/0020-cloud-run-y-pubsub.md) y la arquitectura
en [`arquitectura.md`](arquitectura.md).

## 1. Requisitos

| Qué | Cómo |
| --- | --- |
| Docker | El único programa que hace falta en la máquina |
| Credenciales de GCP de una persona | `make gcloud ARGS="auth login"` y `make gcloud ARGS="auth application-default login"`. Quedan en el volumen `mooc-gcloud`. Nunca una clave JSON de cuenta de servicio (ADR-0015, D3) |
| Variables privadas | `infra/environments/entrega4/privado.auto.tfvars`, a partir de `privado.auto.tfvars.example`: cuenta de facturación e integrantes. No se versiona |
| Proyecto | `mooc-509602`, región `us-central1` |

## 2. Estado de Terraform

El entorno `entrega4` hereda el estado de `entrega3`, así que la base, los
buckets, los secretos, el registro y la cola no se recrean. Se copia una sola
vez:

```bash
make gcloud ARGS="storage cp gs://mooc-tfstate-mooc-509602/entrega3/default.tfstate \
  gs://mooc-tfstate-mooc-509602/entrega4/default.tfstate"
make tf ENTORNO=entrega4 ARGS="init -reconfigure"
```

Desde la copia, `make tf ENTORNO=entrega3` se niega a correr. Su configuración
sigue en el tag `entrega-3`.

## 3. Imágenes

```bash
make publicar
```

Construye las imágenes para `linux/amd64`, las etiqueta con el SHA del commit y
las sube a Artifact Registry (`us-central1-docker.pkg.dev/mooc-509602/mooc`).
Se niega si hay cambios sin confirmar. Las de esta entrega son:

| Imagen | Servicio o trabajo | Contenido |
| --- | --- | --- |
| `frontend` | `mooc-frontend` | Caddy con la interfaz estática y `/config.js` |
| `api` | `mooc-backend` | La API |
| `workers-cloudrun` | `mooc-workers` | El worker, FFmpeg y ClamAV con sus firmas |
| `migrate` | `mooc-migrate` | Las migraciones |
| `seed` | `mooc-seed` | Las cuentas sintéticas |

El SHA impreso va a `version_imagenes` en
`infra/environments/entrega4/terraform.tfvars`.

## 4. Infraestructura

En `terraform.tfvars`, `cloudsql_encendida = true` y `entorno_encendido =
true`. Después, siempre con un plan revisado:

```bash
make tf ENTORNO=entrega4 ARGS="plan -out=p.tfplan"   # leerlo entero
make tf ENTORNO=entrega4 ARGS="apply p.tfplan"
```

| Archivo | Qué crea |
| --- | --- |
| `cloudrun.tf` | Los servicios `mooc-frontend`, `mooc-backend` y `mooc-workers`, y los trabajos `mooc-migrate` y `mooc-seed` |
| `pubsub.tf` | El topic `mooc-trabajos`, la suscripción push `mooc-workers`, el topic `mooc-trabajos-muertos`, sus permisos y Cloud Scheduler |
| `identidades.tf` | Las cuentas de servicio y sus permisos |
| `red-cloudrun.tf` | La regla de firewall de Cloud Run hacia Redis y Mailpit |
| `main.tf`, `cola.tf` | Red, Cloud SQL, buckets, secretos, registro y la máquina de la cola, heredados |

La primera vez que se activa la API de Cloud Run, su agente tarda unos minutos
en propagarse y la creación de los servicios falla con «Error code 7». Se
repite el `plan` y el `apply` y los servicios se crean.

## 5. Base de datos

```bash
make gcloud ARGS="run jobs execute mooc-migrate --region us-central1 --wait"
make gcloud ARGS="run jobs execute mooc-seed --region us-central1 --wait"
```

Las migraciones corren una vez por despliegue, fuera de las réplicas. La
semilla crea las cuentas sintéticas con la contraseña del secreto
`seed-password`, que nunca aparece en el repositorio ni en el plan.

## 6. Configuración de los servicios

| Servicio | CPU | Memoria | Concurrencia | Tiempo máximo | Instancias | Ingreso |
| --- | --- | --- | --- | --- | --- | --- |
| `mooc-frontend` | 1 | 512 MiB | 80 | 30 s | 0 a 3 | Público |
| `mooc-backend` | 1 | 2 GiB | 20 | 60 s | 0 a 3 | Público |
| `mooc-workers` | 4 | 4 GiB | 1 | 600 s | 0 a 3 | Interno |

| Pub/Sub | Valor |
| --- | --- |
| Topic | `mooc-trabajos`, retención de 1 día |
| Suscripción | `mooc-workers`, push a `/pubsub/push` con token OIDC de `mooc-pubsub-push` |
| Plazo de confirmación | 600 s |
| Reintentos | Exponencial de 10 a 600 s |
| Entregas máximas | 5, después al topic `mooc-trabajos-muertos` |

| Cuenta de servicio | Usada por | Permisos |
| --- | --- | --- |
| `mooc-web` | `mooc-backend`, `mooc-migrate`, `mooc-seed` | Publica en `mooc-trabajos`, lee `database-url`, firma URL sobre sí misma, objetos según bucket |
| `mooc-worker` | `mooc-workers` | Publica en `mooc-trabajos`, lee `database-url`, objetos según bucket |
| `mooc-frontend` | `mooc-frontend` | Ninguno sobre datos |
| `mooc-pubsub-push` | Token OIDC de la suscripción | `roles/run.invoker` sobre `mooc-workers` |
| `mooc-programador` | Cloud Scheduler (reaper y poda) | `roles/run.invoker` sobre `mooc-workers` |

`mooc-workers` tiene ingreso interno y solo esas dos últimas cuentas pueden
invocarlo. Frontend y backend admiten `allUsers` porque son la aplicación
pública. Los permisos se definen en `identidades.tf` y `pubsub.tf`.

## 7. Verificación

```bash
make tf ENTORNO=entrega4 ARGS=output       # URL de los tres servicios
curl https://mooc-backend-568765476410.us-central1.run.app/readyz
make postman-nube                          # la colección entera contra el backend
make frontend-e2e-nube                     # el recorrido de navegador contra el frontend
```

`/readyz` responde `ok` para PostgreSQL, Redis y el almacén. Una petición a los
workers desde Internet recibe 404, porque su ingreso es interno.

### Seguir una tarea

Cada tarea deja tres registros con la misma `job_key` y `trace_id`: «trabajo
publicado» en `mooc-backend`, y «trabajo iniciado» y «trabajo completado» en
`mooc-workers`.

```bash
make gcloud ARGS="logging read 'resource.type=\"cloud_run_revision\" AND jsonPayload.job_key=\"<job_key>\"' \
  --format='value(timestamp,resource.labels.service_name,jsonPayload.message)'"
```

El estado final queda en `platform.job_runs` y en el recurso que originó la
tarea, que la API expone: el estado de un archivo o la insignia de una
inscripción.

### Comprobar una reentrega

Se publica en el topic una copia de un trabajo ya terminado, con su misma
`job_key`. Hace falta un usuario con permiso de publicación (el dueño del
proyecto lo tiene):

```bash
K=badge.issue:<id-de-la-inscripción>
make gcloud ARGS="pubsub topics publish mooc-trabajos \
  --message='{\"job_key\":\"$K\",\"trace_id\":\"reentrega-manual\"}' \
  --attribute=tipo=badge.issue,cola=critical,job_key=$K"
```

El worker responde 204 y registra «entrega repetida de un trabajo terminado; se
confirma sin repetirlo», sin un segundo «trabajo iniciado».

## 8. Apagar entre sesiones

`cloudsql_encendida = false` y `entorno_encendido = false`, con plan y `apply`.
La base y la cola se detienen y Cloud Scheduler se pausa. Los servicios de
Cloud Run escalan a cero solos y no cobran sin peticiones.

## 9. Probar en local

```bash
make up && make prueba-pubsub
```

Levanta el emulador de Pub/Sub, una API que publica en el topic y el worker de
Cloud Run recibiendo por push. Pasa la colección de Postman con la cadena de
medios y comprueba que una reentrega de un trabajo terminado se confirma sin
repetirlo.
