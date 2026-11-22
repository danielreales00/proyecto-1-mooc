# Guía de despliegue. Entrega 5

Cómo desplegar la plataforma con Terraform, incluida la función de insignias,
desde un clon del repositorio. Todo corre en contenedores: ni `gcloud` ni
`terraform` se instalan en la máquina. Las decisiones están en el
[ADR-0021](../../arquitectura/adr/0021-insignias-en-una-cloud-run-function.md)
y la arquitectura en [`arquitectura.md`](arquitectura.md).

El despliegue y sus actualizaciones son `init`, `plan` y `apply`. Lo único que
se hace fuera de Terraform es lo que Terraform no puede hacer por sí mismo:
crear el proyecto, enlazar su facturación, crear el bucket de su propio estado
y construir las imágenes.

## 1. Requisitos

| Qué | Cómo |
| --- | --- |
| Docker | El único programa que hace falta en la máquina |
| Credenciales de GCP de una persona | `make gcloud ARGS="auth login"` y `make gcloud ARGS="auth application-default login"`. Quedan en el volumen `mooc-gcloud`. Nunca una clave JSON de cuenta de servicio (ADR-0015, D3) |
| Variables privadas | `infra/environments/entrega5/privado.auto.tfvars`, a partir de `privado.auto.tfvars.example`: cuenta de facturación e integrantes. No se versiona |
| Proyecto | `mooc-509602`, región `us-central1` |

`make tf ENTORNO=entrega5 ARGS="..."` corre Terraform en un contenedor desde
`infra/environments/entrega5`. Es `terraform` con los mismos argumentos.

## 2. Parámetros del entorno

Todo lo que cambia entre entornos está en `terraform.tfvars`, que se versiona
porque no tiene secretos, y en `privado.auto.tfvars`, que no se versiona.

| Variable | Qué fija |
| --- | --- |
| `project_id`, `region`, `zona` | Dónde se despliega |
| `version_imagenes` | Qué imágenes corren: el SHA que imprime `make publicar` |
| `cloudsql_encendida`, `entorno_encendido` | El interruptor de la base y de la cola, Cloud Scheduler y Cloud NAT |
| `frontend`, `backend`, `workers` | CPU, memoria, concurrencia y tiempo máximo de cada servicio de Cloud Run |
| `max_instancias` | El máximo de cada servicio, validado en 3 o menos |
| `funcion` | CPU, memoria, tiempo máximo, instancias y conexiones a la base de la función |
| `pubsub_max_entregas` | Entregas antes de marcar un trabajo `dead`, en los workers y en la función |
| `db_max_conns`, `db_max_conns_api` | Conexiones por instancia, contra las 47 útiles de la base |
| `ejecutar_semilla` | Un valor nuevo ejecuta la semilla en el próximo `apply` |
| `cuenta_facturacion`, `integrantes` | En `privado.auto.tfvars` |

El bucket del estado se fija en `backend.tf`, porque Terraform no admite
variables en el bloque `backend`.

## 3. Estado de Terraform y recursos existentes

### Desde un proyecto vacío

Terraform crea todo, incluidos los secretos: la contraseña de la base y la de
las cuentas sintéticas se generan en el `apply` y van directo a Secret
Manager, sin pasar por el repositorio ni por el plan.

```bash
make gcloud ARGS="projects create <proyecto>"
make gcloud ARGS="billing projects link <proyecto> --billing-account=<cuenta>"
make gcloud ARGS="storage buckets create gs://mooc-tfstate-<proyecto> \
  --location us-central1 --uniform-bucket-level-access"
make gcloud ARGS="storage buckets update gs://mooc-tfstate-<proyecto> --versioning"
```

Con otro proyecto se cambian `project_id` en `terraform.tfvars` y el bucket en
`backend.tf`. Los servicios necesitan sus imágenes antes de crearse, así que el
primer `apply` crea solo las APIs y el registro:

```bash
make tf ENTORNO=entrega5 ARGS="init"
make tf ENTORNO=entrega5 ARGS="plan -target=module.registry -out=p.tfplan"
make tf ENTORNO=entrega5 ARGS="apply p.tfplan"
```

Después siguen las secciones 4 y 5.

### Desde el estado de la Entrega 4

Es el caso de este proyecto. El entorno `entrega5` hereda el estado de
`entrega4`, así que Cloud Run, Pub/Sub, la base, los buckets, los secretos, el
registro y la cola no se recrean. Se copia una sola vez:

```bash
make gcloud ARGS="storage cp gs://mooc-tfstate-mooc-509602/entrega4/default.tfstate \
  gs://mooc-tfstate-mooc-509602/entrega5/default.tfstate"
make tf ENTORNO=entrega5 ARGS="init -reconfigure"
```

Desde la copia, `make tf ENTORNO=entrega4` se niega a correr. Su configuración
sigue en el tag `entrega-4`.

### Recursos que no gestiona Terraform

| Recurso | Quién lo crea | Por qué queda fuera |
| --- | --- | --- |
| El proyecto y su enlace de facturación | Una persona, una vez | Terraform corre dentro del proyecto |
| `gs://mooc-tfstate-mooc-509602` | Una persona, una vez | Guarda el estado de Terraform: no puede crearse a sí mismo |
| `gcf-v2-sources-<número>-us-central1` | Cloud Run functions, con la primera función | Es interno del servicio. La compilación lo lee con un permiso limitado a esos buckets (`funcion.tf`) |
| La suscripción del disparador en `mooc-insignias` | Eventarc, al crear el disparador | La gestiona Eventarc como parte del disparador |
| Las imágenes | `make publicar` | Es la preparación de imágenes. Terraform fija cuál corre |

## 4. Imágenes y código

### Imágenes de los servicios

```bash
make publicar
```

Construye las imágenes para `linux/amd64`, las etiqueta con el SHA del commit y
las sube a Artifact Registry (`us-central1-docker.pkg.dev/mooc-509602/mooc`).
Se niega si hay cambios sin confirmar. El SHA impreso va a `version_imagenes`
en `terraform.tfvars`.

| Imagen | Servicio o trabajo | Contenido |
| --- | --- | --- |
| `frontend` | `mooc-frontend` | Caddy con la interfaz estática y `/config.js` |
| `api` | `mooc-backend` | La API |
| `workers-cloudrun` | `mooc-workers` | El worker, FFmpeg y ClamAV con sus firmas |
| `migrate` | `mooc-migrate` | Las migraciones |
| `seed` | `mooc-seed` | Las cuentas sintéticas |

### Código de la función

No hay paso aparte. Terraform empaqueta el módulo `backend/` con
`archive_file`, lo sube al bucket `mooc-509602-fuentes` con el hash del
contenido en el nombre y despliega la función desde ahí. Cloud Build compila
el paquete raíz (`backend/funcion.go`) con el runtime `go126`. Si el código
cambia, cambia el hash, y el `plan` muestra el objeto y la función como
cambios.

## 5. Desplegar y actualizar

En `terraform.tfvars`, `cloudsql_encendida = true` y `entorno_encendido =
true`. Después, siempre con un plan revisado:

```bash
make tf ENTORNO=entrega5 ARGS="plan -out=p.tfplan"   # leerlo entero
make tf ENTORNO=entrega5 ARGS="apply p.tfplan"
```

Las migraciones corren dentro del `apply`: cada versión de las imágenes crea
una ejecución de `mooc-migrate` y el `apply` espera a que termine. La semilla
solo corre cuando se pide, poniendo un valor nuevo en `ejecutar_semilla`, por
ejemplo la fecha. Con la base apagada, las migraciones no se ejecutan.

| Archivo | Qué crea |
| --- | --- |
| `cloudrun.tf` | Los servicios `mooc-frontend`, `mooc-backend` y `mooc-workers`, y los trabajos `mooc-migrate` y `mooc-seed` |
| `funcion.tf` | El topic `mooc-insignias`, el código empaquetado, la función `mooc-insignias`, su disparador de Eventarc y las cuentas de la función, el disparador y la compilación |
| `pubsub.tf` | El topic `mooc-trabajos`, la suscripción push `mooc-workers`, el topic de mensajes muertos, sus permisos y Cloud Scheduler |
| `identidades.tf` | Las cuentas de servicio de los servicios y sus permisos |
| `red-cloudrun.tf` | La regla de firewall de Cloud Run hacia Redis y Mailpit |
| `main.tf`, `cola.tf` | Red, Cloud SQL, buckets, secretos, registro y la máquina de la cola |

### Lo que conviene saber antes del primer `apply`

Salió en el despliegue del 21 de noviembre y está corregido en la
configuración, pero dos situaciones dependen del momento:

- **APIs nuevas.** Si el plan habilita APIs, el número de proyecto se lee
  durante el `apply` y los permisos que lo usan aparecen como «must be
  replaced». Se evita aplicando primero solo las APIs:

  ```bash
  make tf ENTORNO=entrega5 ARGS="plan -target=google_project_service.apis -out=apis.tfplan"
  make tf ENTORNO=entrega5 ARGS="apply apis.tfplan"
  ```

  y después el plan completo, que ya no reemplaza nada.
- **Propagación.** Justo después de habilitar Eventarc o Cloud Run, la creación
  de la función o de los servicios puede fallar con un 403 o con «Error code
  7». Se repiten el `plan` y el `apply` unos minutos después.

### Comprobar que no queda nada a mano

```bash
make tf ENTORNO=entrega5 ARGS="plan -detailed-exitcode"
```

Debe decir «No changes» y salir con 0: la nube coincide con la configuración.

## 6. Configuración desplegada

| Servicio | Región | Mínimo | Máximo | CPU | Memoria | Concurrencia | Tiempo máximo | Ingreso |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `mooc-frontend` | `us-central1` | 0 | 3 | 1 | 512 MiB | 80 | 30 s | Público |
| `mooc-backend` | `us-central1` | 0 | 3 | 1 | 2 GiB | 20 | 60 s | Público |
| `mooc-workers` | `us-central1` | 0 | 3 | 4 | 4 GiB | 1 | 600 s | Interno |
| `mooc-insignias` (función) | `us-central1` | 0 | 3 | 1 | 512 MiB | 1 | 60 s | Interno |

| Pub/Sub y Eventarc | Valor |
| --- | --- |
| `mooc-trabajos` | Todos los trabajos salvo `badge.issue`. Suscripción push `mooc-workers` con token OIDC, plazo de 600 s, reintentos de 10 a 600 s, cinco entregas y después `mooc-trabajos-muertos` |
| `mooc-insignias` | Solo `badge.issue`. Disparador de Eventarc hacia la función, con reintentos. A los cinco intentos la función marca el trabajo `dead` y corta los reintentos |
| Enrutamiento | `PUBSUB_TOPICOS=badge.issue=mooc-insignias` en el backend y los workers |

| Cuenta de servicio | Usada por | Permisos |
| --- | --- | --- |
| `mooc-web` | `mooc-backend`, `mooc-migrate`, `mooc-seed` | Publica en los dos topics, lee `database-url`, firma URL sobre sí misma, objetos según bucket |
| `mooc-worker` | `mooc-workers` | Publica en los dos topics, lee `database-url`, objetos según bucket |
| `mooc-insignias` | La función | Lee `database-url`, escribe objetos en el bucket `badges` |
| `mooc-eventarc` | El disparador | Recibe eventos y solo puede invocar la función |
| `mooc-compilacion` | Cloud Build, al compilar la función | Escribe registros y en Artifact Registry, y lee el código de la función |
| `mooc-frontend` | `mooc-frontend` | Ninguno sobre datos |
| `mooc-pubsub-push` | Token OIDC de la suscripción | `roles/run.invoker` sobre `mooc-workers` |
| `mooc-programador` | Cloud Scheduler (reaper y poda) | `roles/run.invoker` sobre `mooc-workers` |

Los secretos están en Secret Manager y los servicios y la función los reciben
por referencia. El estado de Terraform vive en el bucket del estado, con
versionado. Ninguno de los dos está en el repositorio.

## 7. Verificación

```bash
curl https://mooc-backend-568765476410.us-central1.run.app/readyz
make postman-nube    # la colección entera, hasta la insignia
```

### Seguir una insignia

```bash
make gcloud ARGS="logging read 'log_id(\"run.googleapis.com/stdout\") AND jsonPayload.type=\"badge.issue\"' \
  --freshness 1h --order asc \
  --format='value(timestamp,resource.labels.service_name,jsonPayload.message,jsonPayload.job_key,jsonPayload.topic)'"
```

Cada insignia deja «trabajo publicado» en `mooc-backend`, con el topic
`mooc-insignias`, y «trabajo iniciado» y «trabajo completado» en
`mooc-insignias`, la función. `mooc-workers` no aparece.

### Comprobar una reentrega

Se publica en el topic de la función una copia de un trabajo terminado, con la
`job_key` completa que imprimió la consulta anterior:

```bash
K=badge.issue:<id>
make gcloud ARGS="pubsub topics publish mooc-insignias \
  --message='{\"job_key\":\"$K\",\"trace_id\":\"reentrega-manual\"}' \
  --attribute=tipo=badge.issue,cola=critical,job_key=$K"
```

La función registra «entrega repetida de un trabajo terminado; se confirma sin
repetirlo», sin un segundo «trabajo iniciado».

## 8. Apagar entre sesiones

`cloudsql_encendida = false` y `entorno_encendido = false`, con plan y `apply`.
La base y la cola se detienen, Cloud Scheduler se pausa y Cloud NAT se
destruye. Los servicios de Cloud Run y la función escalan a cero y no cobran
sin peticiones.

## 9. Probar en local

```bash
make up && make prueba-funcion
```

Levanta el emulador de Pub/Sub con los dos topics, la API con el enrutamiento,
el worker de Cloud Run y la función con el mismo marco que en la nube. Lleva a
un estudiante hasta la insignia, entrega el evento a la función dos veces como
lo haría Eventarc y comprueba que la insignia se emite una vez y que el worker
no atiende `badge.issue`. Tarda unos 12 minutos.
