# Despliegue en GCP — plan ejecutable de la Entrega 2

Esto es el plan de trabajo del despliegue, no un diseño.

- **Qué exige la entrega:** [`alcance-entrega-2.md`](alcance-entrega-2.md)
- **Qué se decidió y por qué:** [`adr/0016`](adr/0016-despliegue-iaas-en-dos-maquinas.md),
  que corrige [`adr/0015`](adr/0015-terraform-y-provision-de-gcp.md)
- **Cómo se mide:** [`../capacity-planning/pruebas_de_carga_entrega2.md`](../capacity-planning/pruebas_de_carga_entrega2.md)
- **Arquitectura objetivo a largo plazo:** [`disenos/ruta-a-gcp.md`](disenos/ruta-a-gcp.md)

Aquí está qué se hace, en qué orden y cómo se comprueba que funcionó. Cada fase
acaba en una comprobación: si no pasa, no se avanza.

> **Revisado el 27 de septiembre de 2026** contra el enunciado de la Entrega 2.
> La versión anterior de este documento apuntaba a Cloud Run, Memorystore y
> Cloud CDN, que el enunciado **prohíbe**. Lo que sobrevive entero es el
> adaptador de Cloud Storage, la autenticación sin claves y Terraform.

---

## La forma del despliegue

```
                    Internet
                       │  443
              ┌────────▼─────────┐
              │   WEB SERVER     │  2 vCPU · 2 GiB · 30 GiB
              │  IP externa fija │
              │  caddy · api     │
              │  swagger         │
              └───┬──────────┬───┘
       red privada│          │red privada
          ┌───────▼──┐   ┌───▼──────────────┐
          │ Cloud SQL│   │  WORKER SERVER   │  2 vCPU · 2 GiB · 30 GiB
          │ PG 17    │   │  sin IP externa  │
          │ zonal    │   │  redis · worker  │
          │ IP priv. │◄──┤  worker-media    │
          └──────────┘   │  clamav          │
                         └───┬──────────────┘
                             │  Cloud NAT (salida)
          ┌──────────────────▼─────────┐
          │      Cloud Storage         │  ← carga directa con URL firmada
          │ originals derived badges   │  ← HLS servido directo, sin CDN
          │ quarantine                 │
          └────────────────────────────┘
```

Dos máquinas fijas. Ni balanceador, ni autoescalado, ni réplicas, ni CDN: el
enunciado los prohíbe, porque lo que se evalúa es **la capacidad medida de esta
configuración**.

---

## Herramientas

En esta máquina **no se instala nada**. `gcloud` y `terraform` corren en
contenedor con versión fija, y la credencial vive en el volumen `mooc-gcloud`,
nunca en el repositorio (ADR-0015, D3).

```bash
make gcloud ARGS="projects list"              # cualquier orden de gcloud
make tf ENTORNO=entrega2 ARGS="plan"          # terraform del entorno
```

La autenticación **ya está hecha** en el volumen: cuenta de usuario y
credenciales por defecto de la aplicación de tipo `authorized_user`. Ninguna
clave de cuenta de servicio.

---

## Estado

### Hecho

| Pieza | Estado |
| --- | --- |
| Proyecto, facturación, región | `mooc-509602`, facturación enlazada, `us-central1` |
| Bucket del estado de Terraform | `gs://mooc-tfstate-mooc-509602`, con versionado |
| Autenticación de `gcloud` y Terraform | En el volumen `mooc-gcloud` |
| Preparación del backend | `PORT`, `LOG_FORMAT`, `DB_MAX_CONNS`, `OBJECT_STORE`, puerto `objectstore.Almacen`, `SMTP_USER`/`SMTP_PASSWORD`, `TRUSTED_PROXY_HOPS`, `REDIS_TLS` |

### Los cuatro huecos del 26 de septiembre, revisados contra el modelo IaaS

| # | Hueco | Estado ahora |
| --- | --- | --- |
| 1 | La cola no hablaba TLS | Resuelto. **Y aquí no hace falta**: Redis no sale de la subred privada, `REDIS_TLS=false`. El trabajo queda para cuando vuelva Memorystore |
| 2 | El correo salía sin autenticar | Resuelto en código. **Queda elegir proveedor** y cargar la contraseña en Secret Manager |
| 3 | El límite por IP se evadía tras el balanceador | Resuelto, y **deja de ser una incógnita**: no hay balanceador, y Caddy *sobrescribe* `X-Forwarded-For`. `TRUSTED_PROXY_HOPS=0` es correcto |
| 4 | `/docs` se quedaba sin ruta | **Resuelto por la arquitectura**: Caddy corre en el Web Server igual que en local |

### Falta

Todo lo que exige GCP delante (fases 0 a 5) más lo que exige medir (6 y 7). El
detalle del código está en [`alcance-entrega-2.md`](alcance-entrega-2.md),
sección «Trabajo de código».

### No se toca

Docker Compose sigue siendo el entorno de desarrollo y el de la demostración
local. Si algo solo funciona en GCP, se rompió el ADR-0010.

---

## Lo que nadie había mirado: la carga multipart no existe en GCS

Sigue vigente, y con el mismo peso: la carga es directa al almacenamiento
también en esta entrega.

ADR-0015 (D1) dice «añadir un adaptador GCS nativo detrás del mismo puerto». El
puerto incluye `CrearMultipart`, `PresignPart` y `CompletarMultipart`, y **Cloud
Storage no tiene multipart con URLs prefirmadas por parte**. Su API nativa
ofrece cargas reanudables de una sola sesión secuencial, que es otra cosa: no
permite subir partes en paralelo ni reanudar pidiendo «qué partes faltan». La
API S3-compatible sí lo tiene, pero exige **claves HMAC**, que son exactamente
la credencial estática de larga vida que D3 prohíbe.

**Salida: `compose`.** Cada parte se sube como un objeto propio con su URL
firmada —`tmp/{asset_id}/part-0001`, …— y al completar se unen con la operación
`compose`, que combina objetos del mismo bucket sin descargarlos.

- Máximo **32 objetos por operación** y **1024 componentes** en el resultado.
  Con partes de 8 MiB son 8 GiB, por encima del tope de 5 GiB del ADR-0011:
  basta componer en dos niveles.
- Componer grupos de 32 en ráfaga puede dar `rateLimitExceeded`; se reintenta
  con *backoff*, que el worker ya sabe hacer.
- Las partes temporales se borran tras componer, y el bucket lleva una regla de
  ciclo de vida que limpia `tmp/` a los 7 días por si un `complete` no llega.

**El contrato de la API no cambia.** `POST /assets/init` sigue devolviendo una
URL por parte y `GET /assets/{id}/upload` sigue diciendo cuáles faltan: la
reanudación se mantiene, que es la condición de aceptación que importa (RF-05).

---

## Fases

### Fase 0 · Cimientos en Terraform

**Objetivo:** que exista dónde poner las cosas y cómo entrar sin claves.

1. Estructura de `infra/`, *backend* de estado apuntando al bucket y
   `versions.tf` con los proveedores fijados.
2. **Habilitar las APIs** con `google_project_service` y
   `disable_on_destroy = false`, o un `destroy` deja el proyecto inservible:

   `compute`, `sqladmin`, `servicenetworking`, `storage`, `secretmanager`,
   `artifactregistry`, `iamcredentials`, `cloudresourcemanager`, `monitoring`,
   `logging`, `iap`.

   > Cambió respecto al plan anterior: **fuera** `run`, `redis` y `vpcaccess`;
   > **dentro** `servicenetworking` —sin él no hay IP privada en Cloud SQL— e
   > `iap`, para administrar el Worker Server sin IP externa.

3. **Workload Identity Federation** para GitHub Actions: *pool*, proveedor OIDC
   **restringido al repositorio** `danielreales00/proyecto-1-mooc`, y una cuenta
   de servicio de despliegue con los roles mínimos.

**Comprobación:** `apply` y luego `plan` sin cambios pendientes. Después, un
*workflow* de prueba que obtiene un token y ejecuta `gcloud auth list` sin que
exista ninguna clave ni en el repositorio ni en los secretos de GitHub.

> La restricción por repositorio del proveedor OIDC no es opcional. Sin ella,
> cualquier repositorio de GitHub puede pedir credenciales de tu proyecto.

### Fase 1 · Red, datos y secretos

**Objetivo:** la infraestructura sin cómputo, reproducible.

```
infra/
  modules/  red · cloudsql · gcs · secretos · registry · maquinas
  environments/entrega2/   main.tf · variables.tf · terraform.tfvars
  versions.tf
```

| Recurso | Notas que evitan sorpresas |
| --- | --- |
| VPC y subred | Una subred `/24` en `us-central1`. Rango privado propio, documentado |
| **Acceso privado a servicios** | Rango reservado + peering. **Sin esto Cloud SQL no tiene IP privada**, y el enunciado exige conexión privada |
| Cloud NAT | El Worker Server no tiene IP externa y necesita salir para descargar imágenes y firmas de ClamAV. Es la «conectividad saliente» que el enunciado pide documentar |
| Reglas de firewall | `443` y `80` desde Internet **solo al Web Server**; dentro de la subred, solo los puertos que hacen falta: `6379` (Redis) y `5432` (Cloud SQL) desde las máquinas. Nada más entra |
| IP externa estática | Para el Web Server. Una efímera cambia al reiniciar y rompe el certificado |
| Cloud SQL PostgreSQL 17 | **Zonal, sin HA, sin réplicas, sin IP pública.** Copias diarias activadas. El tier se elige aquí y **se anota en el informe** |
| Buckets | `originals`, `derived`, `badges`, `quarantine`. Todos privados salvo `badges` —lectura pública, sin listado, porque verificar una insignia es público (CA-07)—. Ciclo de vida: `tmp/` a 7 días |
| Secret Manager | Solo los **contenedores**; los valores se cargan aparte. Un secreto en el estado de Terraform es un secreto en texto plano dentro de un bucket |
| Artifact Registry | Repositorio Docker en la misma región |
| Cuentas de servicio | Una por máquina, con lo mínimo. La del Web Server necesita `roles/iam.serviceAccountTokenCreator` **sobre sí misma**: es lo que permite firmar URLs sin clave descargada |

**Comprobación:** `apply`, luego `plan` sin cambios pendientes. Y
`gcloud sql instances describe` sin dirección pública.

### Fase 2 · Imágenes en Artifact Registry

**Objetivo:** que el CI publique las imágenes que las máquinas van a descargar.

- `api`, `worker`, `worker-media`, `migrate`, `seed` y `buckets` desde el mismo
  `backend/Dockerfile`.
- **`--platform linux/amd64` explícito.** Si alguien construye desde un Mac con
  Apple Silicon, la imagen sale `arm64` y falla al arrancar con un error que no
  dice eso.
- Etiquetar con el SHA del commit, no con `latest`. Un despliegue tiene que
  poder señalar qué código corre, y el informe tiene que registrar la versión
  exacta de cada corrida de carga.

**Comprobación:** `gcloud artifacts docker images list` muestra las imágenes con
el SHA del último commit de `main`.

### Fase 3 · Adaptador de Cloud Storage

**Objetivo:** que los objetos vivan en el servicio administrado. Es el trabajo
de código más grande de la entrega.

Escribir `internal/adapters/gcs` implementando `objectstore.Almacen`:

- Multipart emulado con `compose`, según la sección de arriba.
- `PresignGet` / `PresignPart`: URL firmadas V4 **sin clave descargada**,
  firmando con la API de IAM Credentials y la identidad de la máquina.
- `Mover`: copiar y borrar; GCS no tiene *rename*.
- Se activa con `OBJECT_STORE=gcs`. Con `s3` todo sigue igual, y el CI local lo
  sigue comprobando.

**Comprobación:** la suite de pruebas del puerto pasa contra GCS real: subir en
partes, reanudar tras interrumpir, completar, firmar una lectura y moverla de
cuarentena a originales.

### Fase 4 · Las dos máquinas

**Objetivo:** que la plataforma responda en la URL pública.

1. **Composiciones por máquina**: `deploy/compose.web.yml` (caddy, api, swagger)
   y `deploy/compose.worker.yml` (redis, worker, worker-media, clamav), que
   heredan del `docker-compose.yml` de siempre.
2. **Script de arranque** de cada máquina: instala Docker, se autentica contra
   Artifact Registry con la identidad de la instancia, **lee los secretos de
   Secret Manager y escribe el `.env` con permisos `0600`**, y levanta su
   composición.
3. **Agente de métricas** en las dos, desde el arranque. Memoria y disco no
   están en las métricas por defecto de Compute Engine, y el informe las exige.
   Instalarlo después obliga a repetir el primer nivel de carga.
4. **Caddyfile de producción**: nombre público, TLS automático y
   `header_up X-Forwarded-For {remote_host}` como en local.

El nombre público: dominio propio si lo hay, o `<ip-con-guiones>.sslip.io` sobre
la IP estática (ADR-0016, D4). Lo que no se hace es certificado autofirmado.

**Comprobación:** `https://<host>/healthz` con certificado válido de una
autoridad pública, y `GET /readyz` con los tres en `ok` contra Cloud SQL, Redis
y GCS. Y `gcloud compute instances list` mostrando **una sola IP externa**.

### Fase 5 · Migración y verificación funcional

**Objetivo:** demostrar que la portabilidad del ADR-0010 era cierta.

1. Ejecutar `migrate` contra Cloud SQL desde el Web Server.
2. Migrar los objetos existentes y **verificar su integridad** y la
   correspondencia con las referencias de la base.
3. **Pasar la colección de Postman entera** apuntando `base_url` al dominio
   desplegado.

**Comprobación:** `make postman-completo` en verde contra la nube. Es la misma
evidencia de la Entrega 1 contra otra infraestructura, y vale más que cualquier
captura.

### Fase 6 · Instrumentación para poder medir

**Objetivo:** que la fase 7 tenga qué mirar. Se hace **antes** de la carga.

| Qué | Por qué |
| --- | --- |
| **Métricas de cola**: profundidad, antigüedad del trabajo más viejo y tasa de procesamiento | El enunciado las exige y **hoy no se exportan**. Hay contadores de trabajos, no de cola |
| **Límites de tasa por IP configurables** | El generador es una sola IP; con `login_ip` a 60/min la prueba mediría al limitador. Ver §3.1 del informe de capacidad |
| **Generador de datos sintéticos** | El `seed` actual crea cuatro cuentas. Los escenarios necesitan 600 estudiantes y 20 cursos |
| **Tablero** con las métricas de las dos máquinas, Cloud SQL y la cola | Sin esto, la corrida deja números sin contexto |

**Comprobación:** provocar un trabajo muerto y verlo en el tablero y en la
métrica de la cola.

### Fase 7 · Los dos escenarios de carga

**Objetivo:** el 20 % de la nota.

Está definido entero —recorridos, datos, niveles, pausas, criterios de éxito y
de saturación— en
[`../capacity-planning/pruebas_de_carga_entrega2.md`](../capacity-planning/pruebas_de_carga_entrega2.md).
No se improvisa en la corrida.

- Generador **fuera de las dos máquinas**, en una tercera, con su CPU vigilada.
- Línea base más tres niveles crecientes por escenario, y el último nivel
  estable repetido para comprobar estabilidad.
- Infraestructura fija entre corridas; cualquier ajuste se reporta como antes y
  después.

**Comprobación:** las tablas de resultados del informe llenas, con el punto de
degradación identificado y el cuello de botella sustentado con evidencia.

### Fase 8 · Entregables, costos y apagado

**Objetivo:** cerrar la entrega sin dejar la factura corriendo.

1. **Documento de arquitectura** en `docs/entrega2/`, referenciado desde el
   `README.md`, con las cinco secciones que pide el enunciado.
2. **Informe de capacidad** completo.
3. **Tag `entrega-2`** y registro del commit evaluado.
4. **Estimación de costos con fecha y supuestos**, contrastada con el consumo
   observado.
5. **Procedimiento de reconstrucción**, probado. No es documentación de adorno:
   el equipo docente puede pedir sustentación síncrona y repetir una prueba, y
   si los recursos se borraron, recrearlos es responsabilidad del equipo.
6. **Borrar la instancia de Cloud SQL** tras registrar las evidencias,
   conservando antes la copia y los *scripts*. Detener una máquina no elimina
   todos sus costos: quedan disco, IP estática, respaldos y objetos.

**Comprobación:** destruir el entorno y volver a levantarlo con el
procedimiento, cronometrándolo. El tiempo de reconstrucción es un dato del
informe.

---

## Variables por máquina

Lo que cambia respecto a Compose. Lo que no aparece, no cambia.

| Variable | Web Server | Worker Server | Dónde vive |
| --- | --- | --- | --- |
| `APP_ENV` | `production` | `production` | texto |
| `LOG_FORMAT` | `gcp` | `gcp` | texto |
| `PUBLIC_BASE_URL` | `https://<host>` | ídem | texto |
| `DATABASE_URL` | IP privada de Cloud SQL | ídem | **Secret Manager** |
| `DB_MAX_CONNS` | a medir | a medir | texto |
| `REDIS_ADDR` | IP privada del Worker Server, puerto 6379 | `redis:6379` | texto |
| `REDIS_TLS` | `false` | `false` | texto |
| `OBJECT_STORE` | `gcs` | `gcs` | texto |
| `S3_BUCKET_*` | nombres reales | ídem | texto |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | **no se ponen** | **no se ponen** | — |
| `SMTP_ADDR` / `MAIL_FROM` | proveedor real | — | texto |
| `SMTP_USER` | del proveedor | — | texto |
| `SMTP_PASSWORD` | del proveedor | — | **Secret Manager** |
| `TRUSTED_PROXY_HOPS` | `0` | — | texto |
| `CLAMAV_ADDR` | — | `clamav:3310` | texto |
| `WORKER_QUEUES` | — | `critical=6,default=3` y `bulk=1` | texto |
| `WORKER_CONCURRENCY` | — | fijo y registrado; `1` en `worker-media` | texto |

`S3_ACCESS_KEY` y `S3_SECRET_KEY` desaparecen con `OBJECT_STORE=gcs`: ahí está
el sentido de D3. Si alguna vez vuelven a hacer falta, algo se torció.

**`REDIS_ADDR` del Web Server apunta a la otra máquina.** Es el salto de red más
repetido de la plataforma y la primera hipótesis de cuello de botella del
escenario 1 (ADR-0016, D2).

---

## Riesgos, ordenados por lo que cuesta descubrirlos tarde

| Riesgo | Cuándo aparece | Qué hacer |
| --- | --- | --- |
| **ClamAV no cabe en 2 GiB** | Al levantar el Worker Server | Medir el residente de `clamd` en la fase 4. Si no cabe, documentar el fallo, el ajuste mínimo y su efecto, como manda el enunciado |
| **El límite de tasa por IP invalida la prueba** | En la primera corrida, y parece un fallo del sistema | Hacerlo configurable en la fase 6, **antes** de generar carga |
| **Métricas de cola inexistentes** | Cuando hay que contestar cómo evolucionó la cola, con la corrida ya hecha | Fase 6, antes de la 7 |
| **Memoria y disco sin agente** | Al escribir el informe | Agente desde el arranque de las máquinas |
| **Disco de 30 GiB en la transcodificación** | Con el archivo largo | Vigilar desde M0; limpiar temporales al terminar cada trabajo |
| **`e2-small` tiene vCPU compartidas** | Cuando los números no se repiten | Tipo personalizado con 2 vCPU dedicadas |
| **IP efímera** | Al reiniciar la máquina, y rompe el certificado | IP estática desde la fase 1 |
| **Firmas sin clave** | En la fase 3 | `serviceAccountTokenCreator` sobre sí misma; es el permiso que todo el mundo olvida |
| **`rateLimitExceeded` al componer** | Con archivos grandes | *Backoff* en el adaptador |
| **Imagen `arm64`** | Al arrancar, con un error confuso | `--platform linux/amd64` explícito en el CI |
| **La factura sigue corriendo tras la entrega** | En el recibo del mes | Fase 8: borrar Cloud SQL, revisar disco, IP, respaldos y objetos |

---

## Hoja de datos del entorno

| Dato | Valor | Estado |
| --- | --- | --- |
| `project_id` | `mooc-509602` | Activo |
| Cuenta de facturación | `01D669-827C63-9BFAA3` | **Enlazada** |
| Región | `us-central1` | **Confirmada**, fijada en `gcloud config` |
| Bucket del estado de Terraform | `gs://mooc-tfstate-mooc-509602` | **Creado**, con versionado y acceso uniforme |
| Entorno de Terraform | `entrega2` | Único (ADR-0016, D6) |
| Tipo de máquina | 2 vCPU dedicadas, 2048 MiB | **Por confirmar** en la fase 1 |
| Tier de Cloud SQL | | **Por decidir** en la fase 1, y se anota en el informe |
| Nombre público | | **Por decidir**: dominio propio o `sslip.io` |
| Repositorio de Artifact Registry | `mooc` | Lo crea Terraform |
| Correo saliente (proveedor) | | **Sin elegir**; hace falta en la fase 5 |
| Presupuesto mensual tope | | **Falta la alerta** |

**El proyecto está vacío** salvo el bucket del estado. No hay nada que importar.

---

## Cómo arrancar la conversación del despliegue

> Vamos a desplegar el proyecto en GCP. Lee `arquitectura/despliegue-gcp.md`,
> `arquitectura/alcance-entrega-2.md` y `arquitectura/adr/0016` antes de
> escribir nada. **El enunciado de la Entrega 2 manda sobre el ADR-0015**, que
> describe una arquitectura de Cloud Run que aquí está prohibida.
>
> Dos máquinas virtuales, base administrada zonal, objetos en Cloud Storage,
> Redis en contenedor. Nada de CDN, autoescalado, balanceador ni alta
> disponibilidad.
>
> **Todo con infraestructura como código.** `gcloud` solo para consultar y
> comprobar. La única excepción ya está hecha: el bucket del estado.
>
> Estoy en la fase 0. Empieza comprobando con `make gcloud` qué hay en el
> proyecto antes de escribir HCL. Al final de cada fase corre su comprobación y
> enséñame el resultado antes de seguir. Si una comprobación no pasa, no
> avances.
