# Despliegue en GCP — plan ejecutable

Esto es el plan de trabajo del despliegue, no un diseño. El diseño está en
[`disenos/ruta-a-gcp.md`](disenos/ruta-a-gcp.md) (arquitectura destino) y en
[`adr/0015`](adr/0015-terraform-y-provision-de-gcp.md) (decisiones). Aquí está
qué se hace, en qué orden, quién lo hace y cómo se comprueba que funcionó.

**Cómo usar este documento en una conversación nueva.** Empieza por «Estado» y
«Antes de empezar»; lo primero dice qué hay hecho y lo segundo qué tienes que
hacer tú, a mano, antes de que nadie escriba código. Después las fases, en
orden. Cada una acaba en una comprobación: si no pasa, no se avanza.

---

## Estado

**Listo en el backend** (nada de esto exige GCP para comprobarse, y está
comprobado en local):

| Pieza | Qué se hizo |
| --- | --- |
| Puerto de escucha | El proceso obedece `PORT` si está puesta, que es como Cloud Run le dice dónde escuchar. `HTTP_ADDR` sigue mandando en Compose |
| Logs | `LOG_FORMAT=gcp` renombra `level`→`severity` y `msg`→`message`. Sin eso, en Cloud Logging todo llega con gravedad `default` y un WARN no se distingue de un INFO |
| Redis | `REDIS_TLS=true` para Memorystore con cifrado en tránsito, verificando el certificado |
| Pool de PostgreSQL | `DB_MAX_CONNS` por entorno. En Cloud Run el total es ese número **por instancia**, y es fácil agotar Cloud SQL (ADR-0015, D6) |
| Almacén de objetos | Puerto `objectstore.Almacen` con comprobación en compilación, y `OBJECT_STORE` como única variable que nombra al proveedor. Añadir GCS es escribir una implementación, no tocar los módulos |
| Sesión de reproducción | `POST /enrollments/{id}/media-sessions` implementado, con su manifiesto. Es el contrato que en GCP pasa de URL firmada a cookie de CDN **sin que el cliente cambie** (ADR-0015, D2) |

**Falta, y solo se puede hacer con GCP delante:** el adaptador de Cloud Storage
(fase 3), la infraestructura (fases 0–1), el despliegue (fases 2–4) y la entrega
por CDN (fase 5).

**No se toca:** Docker Compose sigue siendo el entorno de desarrollo y el de la
demostración local. Si algo solo funciona en GCP, se rompió el ADR-0010.

---

## Herramientas

En esta máquina **no se instala nada**, como en el resto del proyecto. `gcloud`
y `terraform` corren en contenedor con versión fija, y la credencial vive en el
volumen `mooc-gcloud`, nunca en el repositorio (ADR-0015, D3).

```bash
make gcloud ARGS="projects list"          # cualquier orden de gcloud
make tf ENTORNO=dev ARGS="plan"           # terraform de un entorno
```

La sesión de Claude no puede autenticarte: el navegador es tuyo. La primera vez
corre tú estas dos, y quedan guardadas en el volumen para todo lo demás.

```bash
make gcloud ARGS="auth login --no-launch-browser"
make gcloud ARGS="auth application-default login --no-launch-browser"
```

La primera te identifica a ti para `gcloud`. La segunda deja las **credenciales
por defecto de la aplicación**, que son las que usa Terraform. Son tus
credenciales de usuario, no una clave de cuenta de servicio: D3 sigue intacto.

## Antes de empezar

**Todo se hace con Terraform**, incluida la habilitación de las APIs. Lo manual
se reduce a cuatro cosas, y tres son de una sola orden.

1. **Proyecto con facturación activa.** Es lo único que Terraform no crea aquí:
   sin organización, crear el proyecto desde Terraform exige permisos de
   facturación que una cuenta personal no suele tener. Desde la consola, o:

   ```bash
   make gcloud ARGS="projects create mooc-<algo-unico> --name='Plataforma MOOC'"
   ```

   La facturación se enlaza desde la consola. Anota el `project_id`.

2. **Región.** `us-central1` si no hay razón para otra: es de las más baratas y
   tiene todos los servicios. Una sola región para todo; cruzar regiones se
   paga en egreso y en latencia.

3. **Bucket del estado de Terraform**, con versionado. Es la única excepción a
   «todo por Terraform», porque Terraform no puede crear el sitio donde guarda
   su propio estado:

   ```bash
   make gcloud ARGS="storage buckets create gs://mooc-tfstate-<project_id> \
     --project=<project_id> --location=us-central1 --uniform-bucket-level-access"
   make gcloud ARGS="storage buckets update gs://mooc-tfstate-<project_id> --versioning"
   ```

4. **Presupuesto con alerta** al 50 %, 80 % y 100 %, desde la consola de
   facturación. El egreso de video es lo que se desmanda, y una alerta tardía
   se paga.

Opcional: un **dominio**, si quieres HTTPS con nombre propio. Sin dominio se
despliega igual con la URL de `run.app`, pero no hay CDN con certificado
gestionado y la fase 5 se queda a medias.

Apunta las respuestas en la tabla del final («Hoja de datos del entorno»). La
conversación nueva las va a pedir.

---

## Decisiones ya tomadas

De ADR-0015, para no volver a discutirlas:

| | Decisión |
| --- | --- |
| **D2** | La credencial de reproducción se emite una vez por sesión, no por segmento. Ya implementado con URL firmada; en GCP pasa a cookie de Cloud CDN |
| **D3** | **Nunca se crean claves JSON de cuenta de servicio.** CI entra por Workload Identity Federation; las cargas usan su propia identidad |
| **D5** | Terraform con módulos y entornos, estado remoto en un bucket de GCS |
| **D6** | Cloud SQL PostgreSQL 17 con alta disponibilidad, Memorystore con réplica |
| **D7** | Dos entornos: `dev` y `prod`. Sin un sitio donde equivocarse, se prueba en producción |

Y una que queda abierta, **a decidir midiendo en la fase 4**:

| **D4** | Dónde corre `worker-media`. Cloud Run si la transcodificación cabe; grupo de instancias gestionado si no. No afecta al código: los workers tiran de la cola, no reciben peticiones |

---

## Lo que nadie había mirado: la carga multipart no existe en GCS

ADR-0015 (D1) dice «añadir un adaptador GCS nativo detrás del mismo puerto». El
puerto incluye `CrearMultipart`, `PresignPart` y `CompletarMultipart`, y **Cloud
Storage no tiene multipart con URLs prefirmadas por parte**. Su API nativa
ofrece cargas reanudables de una sola sesión secuencial, que es otra cosa: no
permite subir partes en paralelo ni reanudar pidiendo «qué partes faltan».

La API S3-compatible de GCS sí tiene multipart, pero exige **claves HMAC**, que
son exactamente la credencial estática de larga vida que D3 prohíbe.

**Salida: `compose`.** Cada parte se sube como un objeto propio con su URL
firmada —`tmp/{asset_id}/part-0001`, …— y al completar se unen con la operación
`compose`, que combina objetos del mismo bucket sin descargarlos.

- Máximo **32 objetos por operación** de compose, y **1024 componentes** en el
  objeto resultante. Con partes de 8 MiB son 8 GiB, por encima del tope de 5 GiB
  del ADR-0011: basta componer en dos niveles (grupos de 32 y luego los grupos).
- Componer grupos de 32 en ráfaga puede dar `rateLimitExceeded`; hay que
  reintentar con backoff, que es lo que el worker ya sabe hacer.
- Las partes temporales se borran tras componer, y el bucket lleva una regla de
  ciclo de vida que limpia `tmp/` a los 7 días por si un `complete` no llega.

**El contrato de la API no cambia.** `POST /assets/init` sigue devolviendo una
URL por parte y `GET /assets/{id}/upload` sigue diciendo cuáles faltan: la
reanudación se mantiene, que es la condición de aceptación que importa (RF-05).

Esto convierte D1 de «unas 200 líneas» en unas 350, y es la razón de que el
adaptador se escriba **contra GCS real** y no a ciegas: su parte delicada
—firmar URLs con la identidad de la carga, sin clave descargada— no se puede
comprobar en Compose.

---

## Fases

### Fase 0 · Cimientos, ya en Terraform

**Objetivo:** que exista dónde poner las cosas y cómo entrar sin claves. Con lo
de «Antes de empezar» hecho, esto ya es HCL.

1. Estructura de `infra/` y el *backend* de estado apuntando al bucket, más
   `versions.tf` con los proveedores fijados.
2. **Habilitar las APIs con `google_project_service`**: `run`, `sqladmin`,
   `redis`, `storage`, `secretmanager`, `artifactregistry`, `compute`,
   `vpcaccess`, `iamcredentials`, `cloudresourcemanager`, `monitoring`,
   `logging`. Con `disable_on_destroy = false`, o un `destroy` deja el proyecto
   inservible.
3. **Workload Identity Federation** para GitHub Actions: *pool*, proveedor OIDC
   **restringido al repositorio** `danielreales00/proyecto-1-mooc`, y una cuenta
   de servicio de despliegue con los roles mínimos.

**Comprobación:** `make tf ENTORNO=dev ARGS=apply`, y después
`make tf ENTORNO=dev ARGS=plan` sin cambios pendientes. Luego, un *workflow* de
prueba en GitHub que obtiene un token y ejecuta `gcloud auth list` sin que
exista ninguna clave ni en el repositorio ni en los secretos de GitHub.

> El `restricción por repositorio` del proveedor OIDC no es opcional. Sin ella,
> cualquier repositorio de GitHub puede pedir credenciales de tu proyecto.

### Fase 1 · Infraestructura con Terraform

**Objetivo:** red, datos y secretos, reproducibles.

```
infra/
  modules/  red · cloudsql · memorystore · gcs · secretos · registry · cloudrun · cdn
  environments/dev/   main.tf · variables.tf · terraform.tfvars
  environments/prod/
  versions.tf   proveedores con versión fijada
```

| Recurso | Notas que evitan sorpresas |
| --- | --- |
| VPC + subred | Cloud Run no llega a Memorystore ni a Cloud SQL por IP privada sin salida a la VPC. Dos formas: **salida directa a VPC** (más barata, sin instancias que mantener) o el **conector de acceso sin servidor** (el clásico). Comprobar cuál está disponible en la región y elegir; se presupuesta distinto |
| Cloud SQL PostgreSQL 17 | IP privada, HA regional, copias automáticas y PITR. **Sin IP pública** |
| Memorystore Redis | Con réplica, cifrado en tránsito → `REDIS_TLS=true` |
| Buckets | `originals`, `derived`, `badges`, `quarantine`. Todos privados salvo `badges`, que es de lectura pública porque la verificación de una insignia lo es (CA-07). Ciclo de vida: `tmp/` a 7 días |
| Secret Manager | Solo los **contenedores**; los valores se cargan aparte. Un secreto en el estado de Terraform es un secreto en texto plano dentro de un bucket |
| Artifact Registry | Repositorio Docker en la misma región |
| Cuentas de servicio | Una por carga (`api`, `worker`, `worker-media`), con lo mínimo. La de firma necesita `roles/iam.serviceAccountTokenCreator` **sobre sí misma**: es lo que permite firmar URLs sin clave descargada |

**Comprobación:** `terraform apply` y luego `terraform plan` sin cambios
pendientes. Eso, y no el `apply`, es lo que demuestra que el estado describe la
realidad.

### Fase 2 · Imágenes en Artifact Registry

**Objetivo:** que el CI publique las cuatro imágenes.

- `api`, `worker`, `worker-media`, `migrate` desde el mismo `backend/Dockerfile`.
- **`--platform linux/amd64` explícito.** Si alguien construye desde un Mac con
  Apple Silicon, la imagen sale `arm64` y Cloud Run la rechaza al arrancar, con
  un error que no dice eso.
- Etiquetar con el SHA del commit, no con `latest`. Un despliegue tiene que
  poder señalar qué código corre.

**Comprobación:** `gcloud artifacts docker images list` muestra las cuatro con
el SHA del último commit de `main`.

### Fase 3 · Adaptador de Cloud Storage, migraciones y API

**Objetivo:** que la API responda en Cloud Run contra datos reales.

1. **Escribir `internal/adapters/gcs`**, implementando `objectstore.Almacen`:
   - Multipart emulado con `compose`, según la sección de arriba.
   - `PresignGet`/`PresignPart`: URL firmadas V4 **sin clave descargada**,
     usando la API de IAM Credentials para firmar. Es lo que no se puede probar
     en local y por eso se escribe aquí.
   - `Mover`: copiar y borrar; GCS no tiene *rename*.
   - Activar con `OBJECT_STORE=gcs`. Con `s3` todo sigue funcionando igual, y el
     CI local lo seguirá comprobando.
2. **Migraciones como Cloud Run job**, ejecutado antes de publicar la revisión
   nueva. El binario ya está separado (ADR-0003).
3. **Desplegar `api`** con `min-instances: 1` —el arranque en frío se pagaría en
   el p95— y las variables de la tabla del final.

**Comprobación:** `GET /readyz` devuelve los tres en `ok` contra Cloud SQL,
Memorystore y GCS; y la colección de Postman pasa apuntando `base_url` al
dominio desplegado. Esa es la misma evidencia de la Entrega 1, contra otra
infraestructura: es la prueba de que la portabilidad del ADR-0010 era cierta.

### Fase 4 · Workers

**Objetivo:** que el trabajo asíncrono corra.

- `worker` (colas `critical` y `default`) en Cloud Run, con **CPU siempre
  asignada**: un worker que solo recibe CPU mientras atiende una petición no
  consume la cola, porque aquí no hay peticiones. Si el proyecto tiene
  disponibles los *worker pools* de Cloud Run —pensados justo para cargas que
  tiran de una cola— son mejor encaje que un servicio.
- `worker-media` (cola `bulk`) con **ClamAV como contenedor lateral** en la
  misma revisión, hablándole por `localhost:3310`. La alternativa —un servicio
  aparte— obliga a exponer clamd en la red, y no hace falta.
- **Aquí se decide D4**: medir `job_duration_seconds{type="media.transcode_hls"}`
  con material real. Si un video pasa del máximo cómodo de una instancia, se
  mueve a un grupo de instancias gestionado. **El código no cambia.**

**Comprobación:** subir un video por la API desplegada y verlo llegar a `ready`,
con sus derivados en el bucket. Y el EICAR terminando en `infected` sin
derivados, que es la prueba de que el orden se respeta también en la nube.

### Fase 5 · CDN y cookies firmadas

**Objetivo:** cerrar D2 del todo.

- Balanceador con certificado gestionado y Cloud CDN sobre el bucket de
  derivados.
- `media-sessions` pasa a devolver `delivery: "signed_cookie"` y a poner la
  cookie acotada por prefijo de ruta. **El contrato no cambia**: el cliente lee
  `delivery` y actúa.
- El manifiesto pasa a apuntar al dominio del CDN.

**Comprobación:** reproducir un video midiendo dónde se sirven los segmentos.
Si salen del CDN y no de la API, la fase está hecha.

### Fase 6 · Observabilidad

- Cloud Monitoring con las mismas cuatro reglas de alerta que hoy tiene
  Prometheus, empezando por la de la DLQ.
- Métricas por Managed Prometheus, apuntando al `/metrics` que ya existe.
- Tablero equivalente al de Grafana.

**Comprobación:** provocar un trabajo muerto y recibir la alerta.

### Fase 7 · Carga y recuperación

Lo que el enunciado pide demostrar y hoy no está:

- **Prueba de carga** con k6 contra el entorno desplegado, con los objetivos de
  `disenos/objetivos-de-servicio.md`.
- **Restauración medida**: restaurar Cloud SQL a un punto en el tiempo y
  cronometrarlo. El RTO se demuestra, no se declara.

---

## Variables por servicio

Lo que cambia respecto a Compose. Lo que no aparece aquí, no cambia.

| Variable | `api` | `worker` | `worker-media` | Dónde vive |
| --- | --- | --- | --- | --- |
| `PORT` | lo pone Cloud Run | lo pone Cloud Run | lo pone Cloud Run | — |
| `LOG_FORMAT` | `gcp` | `gcp` | `gcp` | texto |
| `APP_ENV` | `production` | `production` | `production` | texto |
| `PUBLIC_BASE_URL` | dominio público | ídem | ídem | texto |
| `DATABASE_URL` | IP privada de Cloud SQL | ídem | ídem | **Secret Manager** |
| `DB_MAX_CONNS` | `5` con 20 instancias máximo | `5` | `2` | texto |
| `REDIS_ADDR` | IP de Memorystore | ídem | ídem | texto |
| `REDIS_TLS` | `true` | `true` | `true` | texto |
| `REDIS_PASSWORD` | según AUTH | ídem | ídem | **Secret Manager** |
| `OBJECT_STORE` | `gcs` | `gcs` | `gcs` | texto |
| `S3_BUCKET_*` | nombres reales | ídem | ídem | texto |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | **no se ponen** | — | — | — |
| `SMTP_ADDR` / `MAIL_FROM` | proveedor real | ídem | — | texto + secreto |
| `CLAMAV_ADDR` | — | — | `localhost:3310` | texto |
| `WORKER_QUEUES` | — | `critical=6,default=3` | `bulk=1` | texto |

`S3_ACCESS_KEY` y `S3_SECRET_KEY` desaparecen con `OBJECT_STORE=gcs`: ahí está
el sentido de D3. Si alguna vez vuelven a hacer falta, algo se torció.

---

## Riesgos, ordenados por lo que cuesta descubrirlos tarde

| Riesgo | Cuándo aparece | Qué hacer |
| --- | --- | --- |
| **Egreso de video** | En la factura | Medir con la métrica de transcodificación antes de abrir el CDN; poner presupuesto con alerta desde el día uno |
| **Conexiones a Cloud SQL** | Cuando escala, no antes | `DB_MAX_CONNS` bajo y `max-instances` acotado. Es aritmética: instancias × pool ≤ límite de la instancia |
| **Transcodificación en Cloud Run** | Con el primer video largo | Medir en la fase 4 y decidir D4 con el dato |
| **Conector de VPC** | Al primer despliegue | Está en la fase 1 a propósito: sin él nada llega a los datos |
| **Imagen `arm64`** | Al arrancar, con un error confuso | `--platform linux/amd64` explícito en el CI |
| **`rateLimitExceeded` al componer** | Con archivos grandes | Backoff en el adaptador, que el worker ya sabe hacer |
| **Firmas sin clave** | En la fase 3 | `serviceAccountTokenCreator` sobre sí misma; es el permiso que todo el mundo olvida |

---

## Hoja de datos del entorno

Rellénala antes de la fase 0 y tenla a mano en la conversación del despliegue.

| Dato | Valor |
| --- | --- |
| `project_id` | |
| Región | |
| Dominio (o «sin dominio») | |
| Bucket del estado de Terraform | |
| Repositorio de Artifact Registry | |
| Correo saliente (proveedor) | |
| Presupuesto mensual tope | |

---

## Cómo arrancar la conversación del despliegue

Pégale esto, con la hoja de datos rellenada:

> Vamos a desplegar el proyecto en GCP siguiendo
> `arquitectura/despliegue-gcp.md`. Lee ese documento, `adr/0015` y
> `disenos/ruta-a-gcp.md` antes de escribir nada.
>
> **Todo con infraestructura como código.** Nada de crear recursos a mano desde
> la consola ni con `gcloud`: lo único manual es lo de «Antes de empezar», que
> ya está hecho. `gcloud` se usa solo para consultar y comprobar.
>
> Mis datos: `project_id` = …, región = …, bucket de estado = …, dominio = …
>
> Estoy en la fase 0. Empieza comprobando con `make gcloud` qué hay creado ya en
> el proyecto, antes de escribir HCL: si algo existe, o se importa al estado o
> se borra, pero no se duplica.
>
> Trabaja por fases, y al final de cada una corre su comprobación y enséñame el
> resultado antes de seguir. Si una comprobación no pasa, no avances.

Y ten hechas las dos autenticaciones de la sección «Herramientas»: la sesión no
puede autenticarte porque el navegador es tuyo.
