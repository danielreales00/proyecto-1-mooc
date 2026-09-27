# Despliegue en GCP — plan ejecutable de la Entrega 2

Esto es el plan de trabajo del despliegue, no un diseño.

- **Qué exige la entrega:** [`alcance-entrega-2.md`](alcance-entrega-2.md)
- **Qué se decidió y por qué:** [`adr/0016`](adr/0016-despliegue-iaas-en-dos-maquinas.md),
  que corrige [`adr/0015`](adr/0015-terraform-y-provision-de-gcp.md)
- **Cómo se mide:** [`../capacity-planning/pruebas_de_carga_entrega2.md`](../capacity-planning/pruebas_de_carga_entrega2.md)
- **Arquitectura objetivo a largo plazo:** [`disenos/ruta-a-gcp.md`](disenos/ruta-a-gcp.md)

**Este plan está recortado a lo necesario para la entrega.** Semana y media,
cuatro personas. Lo que no sostiene un criterio de evaluación no está aquí: está
en «Lo que se deja fuera a propósito», al final, para que se sepa que se decidió
y no que se olvidó.

> **Revisado el 27 de septiembre de 2026** contra el enunciado de la Entrega 2.
> La versión anterior apuntaba a Cloud Run, Memorystore y Cloud CDN, que el
> enunciado **prohíbe**.

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

## Dos vías en paralelo

Las fases no son una fila india para cuatro personas. Hay dos vías que avanzan a
la vez desde el primer día y confluyen en las pruebas de carga.

```
VÍA A · infraestructura        A0 ─► A1 ─► A2 ─► A3 ─► A4 ─┐
                                                            ├─► C1 ─► C2
VÍA B · código                 B1 ──────────────────► B2 ──┘
```

| | Vía A · infraestructura | Vía B · código |
| --- | --- | --- |
| **A0** | Cimientos en Terraform | **B1** Adaptador de Cloud Storage |
| **A1** | Red, datos y secretos | **B2** Instrumentación para medir |
| **A2** | Imágenes en Artifact Registry | |
| **A3** | Las dos máquinas | |
| **A4** | Verificación funcional | |
| | **C1** Los dos escenarios de carga | |
| | **C2** Entregables, costos y apagado | |

**B1 es lo más largo de la entrega y no depende de las máquinas**: solo necesita
un bucket, que sale en A1. Empieza el primer día o llega tarde.

Lo verdaderamente secuencial es A1 → A3 → A4 → C1, y **C1 come reloj, no
esfuerzo**: cada nivel son entre ocho y quince minutos más las pausas, y el
nivel cercano al límite se repite. Reservar los dos últimos días para correrla
es optimista; reservarlos para *repetirla* es realista.

---

## Herramientas

En esta máquina **no se instala nada**. `gcloud` y `terraform` corren en
contenedor con versión fija, y la credencial vive en el volumen `mooc-gcloud`,
nunca en el repositorio (ADR-0015, D3).

```bash
make gcloud ARGS="projects list"              # cualquier orden de gcloud
make tf ENTORNO=entrega2 ARGS="plan"          # terraform del entorno
```

La autenticación **ya está hecha**: cuenta de usuario y credenciales por defecto
de la aplicación de tipo `authorized_user`. Ninguna clave de cuenta de servicio.

---

## Estado

### Hecho

| Pieza | Estado |
| --- | --- |
| Proyecto, facturación, región | `mooc-509602`, facturación enlazada, `us-central1` |
| Bucket del estado de Terraform | `gs://mooc-tfstate-mooc-509602`, con versionado |
| Autenticación de `gcloud` y Terraform | En el volumen `mooc-gcloud` |
| A0 · Cimientos | APIs y presupuesto en Terraform, `plan` sin cambios (27-09) |
| A1 · Red, datos y secretos | 46 recursos, `plan` sin cambios, Cloud SQL sin IP pública (27-09) |
| Red `default` borrada | La crea Compute Engine con 22 y 3389 abiertos a Internet. Importada y destruida con Terraform (`infra/environments/entrega2/red-default.tf`) (27-09) |
| Preparación del backend | `PORT`, `LOG_FORMAT`, `DB_MAX_CONNS`, `OBJECT_STORE`, puerto `objectstore.Almacen`, `SMTP_USER`/`SMTP_PASSWORD`, `TRUSTED_PROXY_HOPS`, `REDIS_TLS` |

### Los cuatro huecos del 26 de septiembre, revisados contra el modelo IaaS

| # | Hueco | Estado ahora |
| --- | --- | --- |
| 1 | La cola no hablaba TLS | Resuelto. **Y aquí no hace falta**: Redis no sale de la subred privada, `REDIS_TLS=false` |
| 2 | El correo salía sin autenticar | Resuelto en código. **Queda elegir proveedor** (A4) |
| 3 | El límite por IP se evadía tras el balanceador | Resuelto, y **deja de ser una incógnita**: no hay balanceador, y Caddy *sobrescribe* `X-Forwarded-For`. `TRUSTED_PROXY_HOPS=0` es correcto |
| 4 | `/docs` se quedaba sin ruta | **Resuelto por la arquitectura**: Caddy corre en el Web Server igual que en local |

### No se toca

Docker Compose sigue siendo el entorno de desarrollo y el de la demostración
local. Si algo solo funciona en GCP, se rompió el ADR-0010.

---

## Lo que nadie había mirado: la carga multipart no existe en GCS

Sigue vigente, y con el mismo peso: la carga es directa al almacenamiento
también en esta entrega.

El puerto `objectstore` incluye `CrearMultipart`, `PresignPart` y
`CompletarMultipart`, y **Cloud Storage no tiene multipart con URLs prefirmadas
por parte**. Su API nativa ofrece cargas reanudables de una sola sesión
secuencial, que es otra cosa: no permite subir partes en paralelo ni reanudar
pidiendo «qué partes faltan». La API S3-compatible sí lo tiene, pero exige
**claves HMAC**, que son exactamente la credencial estática de larga vida que
ADR-0015 (D3) prohíbe.

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

# Vía A · Infraestructura

### A0 · Cimientos en Terraform

**Objetivo:** que Terraform pueda crear cosas.

1. Estructura de `infra/`, *backend* de estado apuntando al bucket y
   `versions.tf` con los proveedores fijados.
2. **Habilitar las APIs** con `google_project_service` y
   `disable_on_destroy = false`, o un `destroy` deja el proyecto inservible:

   `compute`, `sqladmin`, `servicenetworking`, `storage`, `secretmanager`,
   `artifactregistry`, `iamcredentials`, `cloudresourcemanager`, `monitoring`,
   `logging`, `iap`, más `iam` (crear las cuentas de servicio) y
   `billingbudgets` (el presupuesto), que faltaban en la primera versión.

   `cloudresourcemanager` va **antes** que las demás: el proveedor comprueba
   cada API a través de ella, y si se habilita en el mismo lote las que
   terminan después fallan con 403. Pasó en el primer `apply`.

3. **Presupuesto con alerta** al 50 %, 80 % y 100 % (`google_billing_budget`).
   Necesita permiso sobre la cuenta de facturación, no sobre el proyecto. Si la
   cuenta no lo permite, se hace desde la consola y **se documenta como
   limitación**, que es lo que el enunciado pide registrar.

   Hecho en Terraform: la cuenta lo permite (`billing.admin`). Dos detalles:
   con ADC de usuario la API de presupuestos exige proyecto de cuota
   (`user_project_override` en el proveedor), y como la cuenta es de
   educación y paga con créditos, el presupuesto **excluye los créditos**; si
   no, el gasto neto sería cero y ninguna alerta saltaría.

**Comprobación:** `apply` y luego `plan` sin cambios pendientes.

### A1 · Red, datos y secretos

**Objetivo:** la infraestructura sin cómputo, reproducible. Es la fase que
sostiene tres criterios de la rúbrica a la vez.

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
| Reglas de firewall | `443` y `80` desde Internet **solo al Web Server**; `6379` solo del Web Server al Worker Server; `22` solo desde el rango de IAP. Apuntan a **cuentas de servicio**, no a etiquetas. Una regla de denegación explícita con registro cierra el resto. El `5432` no lleva regla: Cloud SQL está al otro lado del peering y el firewall de esta VPC no la gobierna; la protege no tener IP pública |
| IP externa estática | Para el Web Server. Una efímera cambia al reiniciar y rompe el certificado |
| Cloud SQL PostgreSQL 17 | **Zonal, sin HA, sin réplicas, sin IP pública.** Copias diarias activadas: son la vía para borrar la instancia al final sin perder la evidencia, y `retain_backups_on_delete` las conserva tras borrarla. El tier se elige aquí y **se anota en el informe**. Edición `ENTERPRISE` explícita: PostgreSQL 17 crea Enterprise Plus por defecto. Solo conexiones cifradas: `DATABASE_URL` lleva `sslmode=require`. El nombre lleva sufijo aleatorio porque un nombre borrado no se reutiliza en días, y C2 destruye y reconstruye |
| Buckets | `originals`, `derived`, `badges`, `quarantine`. Todos privados salvo `badges` —lectura pública, sin listado, porque verificar una insignia es público (CA-07)—. Ciclo de vida: `tmp/` a 7 días |
| Secret Manager | Terraform crea los **contenedores**. La excepción es `database-url`: la clave de la base la genera Terraform como valor **efímero** y la escribe en el usuario de Cloud SQL y en el secreto con atributos **de solo escritura** (`password_wo`, `secret_data_wo`), que no llegan ni al estado ni al plan. Cumple el motivo de la regla —ningún secreto en el estado— sin dejar un paso a mano. `smtp-password` se carga aparte en A4 |
| Artifact Registry | Repositorio Docker en la misma región |
| Cuentas de servicio | Una por máquina, con lo mínimo. La del Web Server necesita `roles/iam.serviceAccountTokenCreator` **sobre sí misma**: es lo que permite firmar URLs sin clave descargada |

**Comprobación:** `apply`, luego `plan` sin cambios pendientes. Y
`gcloud sql instances describe` sin dirección pública.

### A2 · Imágenes en Artifact Registry

**Objetivo:** que exista qué descargar. Se publica **desde una máquina del
equipo**, no desde el CI (ver «Lo que se deja fuera»).

- `api`, `worker`, `worker-media`, `migrate`, `seed` y `buckets` desde el mismo
  `backend/Dockerfile`.
- **`--platform linux/amd64` explícito.** Si alguien construye desde un Mac con
  Apple Silicon, la imagen sale `arm64` y falla al arrancar con un error que no
  dice eso.
- Etiquetar con el SHA del commit, no con `latest`. El informe tiene que
  registrar la versión exacta de cada corrida de carga.

**No se construye en las máquinas.** Compilar Go, FFmpeg y ClamAV en una VM de
2 GiB es pedir que el despliegue falle por memoria.

**Comprobación:** `gcloud artifacts docker images list` muestra las imágenes con
el SHA del commit.

### A3 · Las dos máquinas

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

**Medir aquí el residente de `clamd`.** Es lo primero que puede no caber en
2 GiB. Si no cabe, se documenta el fallo, el ajuste mínimo y su efecto en costo
y capacidad, que es lo que el enunciado manda hacer en ese caso.

**Comprobación:** `https://<host>/healthz` con certificado válido de una
autoridad pública, `GET /readyz` con los tres en `ok`, y
`gcloud compute instances list` mostrando **una sola IP externa**.

### A4 · Verificación funcional

**Objetivo:** demostrar que la portabilidad del ADR-0010 era cierta.

1. Ejecutar `migrate` contra Cloud SQL.
2. Sembrar los datos y **verificar la integridad de los objetos** y su
   correspondencia con las referencias de la base.
3. **Pasar la colección de Postman entera** apuntando `base_url` al dominio
   desplegado.

**Comprobación:** `make postman-completo` en verde contra la nube. Es la misma
evidencia de la Entrega 1 contra otra infraestructura, y vale más que cualquier
captura.

---

# Vía B · Código

### B1 · Adaptador de Cloud Storage — *empieza el primer día*

**Objetivo:** que los objetos vivan en el servicio administrado. Es el trabajo
de código más grande de la entrega y un objetivo explícito del enunciado.

Escribir `internal/adapters/gcs` implementando `objectstore.Almacen`:

- Multipart emulado con `compose`, según la sección de arriba.
- `PresignGet` / `PresignPart`: URL firmadas V4 **sin clave descargada**,
  firmando con la API de IAM Credentials y la identidad de la máquina.
- `Mover`: copiar y borrar; GCS no tiene *rename*.
- Se activa con `OBJECT_STORE=gcs`. Con `s3` todo sigue igual, y el CI local lo
  sigue comprobando.

Solo necesita un bucket para probarse, que sale en A1. No espera a las máquinas.

**Comprobación:** la suite del puerto pasa contra GCS real: subir en partes,
reanudar tras interrumpir, completar, firmar una lectura y mover de cuarentena a
originales.

### B2 · Instrumentación para poder medir

**Objetivo:** que C1 tenga qué mirar. Se hace **antes** de generar carga; si se
deja para después, la corrida se repite.

| Qué | Por qué |
| --- | --- |
| **Métricas de cola**: profundidad, antigüedad del trabajo más viejo y tasa de procesamiento | El enunciado las exige y **hoy no se exportan**. Hay contadores de trabajos, no de cola |
| **Límites de tasa por IP configurables** | El generador es una sola IP; con `login_ip` a 60/min la prueba mediría al limitador. Los de cuenta y sesión se dejan como están |
| **Generador de datos sintéticos** | El `seed` actual crea cuatro cuentas. Los escenarios necesitan 600 estudiantes y 20 cursos, escritos en la base y no por la API (`register` admite 60 por hora y por IP) |
| **Guiones de k6** de los dos escenarios | Se escriben contra el Compose local y se apuntan a la nube después |

Todo esto se desarrolla y se prueba en local. No necesita GCP.

**Comprobación:** en local, provocar un trabajo muerto y verlo en la métrica de
la cola; y correr un nivel bajo de cada escenario contra el Compose.

---

# Confluencia

### C1 · Los dos escenarios de carga

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

### C2 · Entregables, costos y apagado

**Objetivo:** cerrar la entrega sin dejar la factura corriendo.

1. **Documento de arquitectura** en `docs/entrega2/`, referenciado desde el
   `README.md`, con las cinco secciones que pide el enunciado.
2. **Informe de capacidad** completo.
3. **Tag `entrega-2`** y registro del commit evaluado.
4. **Video de sustentación**, máximo 20 minutos, enlazado desde el `README.md`.
5. **Estimación de costos con fecha y supuestos**, contrastada con el consumo
   observado.
6. **Procedimiento de reconstrucción**, probado. No es documentación de adorno:
   el equipo docente puede pedir sustentación síncrona y repetir una prueba, y
   si los recursos se borraron, recrearlos es responsabilidad del equipo.
   **Entre sesiones de trabajo, la base se detiene, no se borra:**
   `cloudsql_encendida = false` en `terraform.tfvars` y `apply`. Detenida no
   cobra cómputo, solo disco y copias, y conserva IP, datos y clave. Falta
   confirmar en la documentación de Google si se reactiva sola, que es una de
   las condiciones que el enunciado pide incorporar al plan de uso.
7. **Borrar la instancia de Cloud SQL** tras registrar las evidencias,
   conservando antes la copia y los *scripts*. Detener una máquina no elimina
   todos sus costos: quedan disco, IP estática, respaldos y objetos.

**Comprobación:** destruir el entorno y volver a levantarlo con el
procedimiento, cronometrándolo. El tiempo de reconstrucción es un dato del
informe.

---

## Lo que se deja fuera a propósito

No se olvidó. Se decidió, y aquí está por qué, para que la decisión se pueda
revisar si sobra tiempo.

| Qué | Por qué se cae | Qué se hace en su lugar |
| --- | --- | --- |
| **Workload Identity Federation** | Existe para que el CI despliegue sin credenciales, y el enunciado **no pide despliegue automatizado**. Ningún criterio lo evalúa | Se publica y se despliega desde una máquina del equipo con el ADC de usuario. **ADR-0015 (D3) sigue intacto**: tampoco hay ninguna clave JSON así |
| **Que el CI publique las imágenes** | Mismo motivo. Montar el *workflow* cuesta más que el `docker push` | `docker push` a Artifact Registry desde la máquina de quien despliega |
| **Tablero de Cloud Monitoring** | Los números están igual en las métricas; el tablero solo los hace cómodos de mirar | Consultas directas al exportar los resultados de cada corrida |
| **Entorno `prod`** | Duplica la factura para proteger un entorno que nadie usa (ADR-0016, D6) | Un solo entorno, `entrega2`. La estructura de módulos permite crear `prod` cuando haga falta |
| **Cloud CDN y cookies firmadas** | Prohibido por el enunciado en esta etapa | HLS directo desde Cloud Storage con URL firmada. `media-sessions` no cambia |
| **Alertas de Cloud Monitoring** | Ningún criterio las evalúa, y con dos máquinas vigiladas a mano durante las corridas no aportan | Las cuatro reglas de Prometheus siguen en local |
| **Restauración medida de Cloud SQL** | El enunciado pide el procedimiento de reconstrucción, no un RTO medido | Se cronometra `destroy` + `apply` en C2, que es el procedimiento que sí piden |

**Si sobra tiempo, el orden para recuperarlas** es: WIF y despliegue desde CI
primero —es lo que más se parece a trabajo real y lo que hará falta en la
entrega siguiente—, luego el tablero, y después las alertas.

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
| **B1 empieza tarde** | A falta de tres días, y no hay forma de acelerarlo | Arrancarlo el primer día, en paralelo con A0 |
| **ClamAV no cabe en 2 GiB** | Al levantar el Worker Server | Medir el residente de `clamd` en A3. Si no cabe, documentar el fallo, el ajuste mínimo y su efecto |
| **El límite de tasa por IP invalida la prueba** | En la primera corrida, y parece un fallo del sistema | B2, **antes** de generar carga |
| **Métricas de cola inexistentes** | Cuando hay que contestar cómo evolucionó la cola, con la corrida ya hecha | B2, antes de C1 |
| **Memoria y disco sin agente** | Al escribir el informe | Agente desde el arranque de las máquinas (A3) |
| **Disco de 30 GiB en la transcodificación** | Con el archivo largo | Vigilar desde el primer nivel; limpiar temporales al terminar cada trabajo |
| **`e2-small` tiene vCPU compartidas** | Cuando los números no se repiten | Tipo personalizado con 2 vCPU dedicadas |
| **IP efímera** | Al reiniciar la máquina, y rompe el certificado | IP estática desde A1 |
| **Firmas sin clave** | En B1 | `serviceAccountTokenCreator` sobre sí misma; es el permiso que todo el mundo olvida |
| **`rateLimitExceeded` al componer** | Con archivos grandes | *Backoff* en el adaptador |
| **Imagen `arm64`** | Al arrancar, con un error confuso | `--platform linux/amd64` explícito al construir |
| **La factura sigue corriendo tras la entrega** | En el recibo del mes | C2: borrar Cloud SQL, revisar disco, IP, respaldos y objetos |

---

## Hoja de datos del entorno

| Dato | Valor | Estado |
| --- | --- | --- |
| `project_id` | `mooc-509602` | Activo |
| Cuenta de facturación | `01D669-827C63-9BFAA3` | **Enlazada** |
| Región | `us-central1` | **Confirmada**, fijada en `gcloud config` |
| Bucket del estado de Terraform | `gs://mooc-tfstate-mooc-509602` | **Creado**, con versionado y acceso uniforme |
| Entorno de Terraform | `entrega2` | Único (ADR-0016, D6) |
| Tipo de máquina | `e2-custom-2-2048`: 2 vCPU completas, 2048 MiB | **Confirmado** en `us-central1-a` |
| Zona | `us-central1-a` | Máquinas y Cloud SQL en la misma |
| Tier de Cloud SQL | `db-g1-small`: núcleo compartido, 1,7 GiB, 10 GiB SSD fijos | **Decidido** el 27-09. El enunciado lo deja al presupuesto, y con 50 USD cuesta la mitad que 1 vCPU dedicada. Limitación a registrar: núcleo compartido, sin SLA, puede ralentizarse sin aviso |
| Red | VPC `mooc`, subred `10.10.0.0/24`, peering de servicios `10.20.0.0/20` | Web `10.10.0.10`, Worker `10.10.0.20`, fijas |
| IP externa del Web Server | `35.184.146.250`, estática | **Reservada** en A1 |
| Nombre público | `35-184-146-250.sslip.io` si no hay dominio propio | **Por decidir** en A3 |
| Cloud SQL | `mooc-pg-55e5`, IP privada `10.20.0.3`, base y usuario `mooc` | **Creada** en A1. `DATABASE_URL` ya está en Secret Manager |
| Buckets | `mooc-509602-{originals,derived,badges,quarantine}` | **Creados** en A1 |
| Cuentas de servicio | `mooc-web`, `mooc-worker` | **Creadas** en A1 |
| Repositorio de Artifact Registry | `mooc` | Lo crea Terraform |
| Correo saliente (proveedor) | | **Sin elegir**; hace falta en A4 |
| Presupuesto con alerta | `mooc-entrega2`, 50 USD/mes, 50/80/100 %, sin créditos | **Creado** en A0. **Es la cifra real** disponible: se puede ampliar, pero se presupuesta con ella |

**El proyecto está vacío** salvo el bucket del estado. No hay nada que importar.

---

## Cómo arrancar la conversación del despliegue

> Vamos a desplegar el proyecto en GCP. Lee `arquitectura/despliegue-gcp.md`,
> `arquitectura/alcance-entrega-2.md` y `arquitectura/adr/0016` antes de
> escribir nada. **El enunciado de la Entrega 2 manda sobre el ADR-0015**, que
> describe una arquitectura de Cloud Run que aquí está prohibida.
>
> Dos máquinas virtuales de 2 vCPU y 2 GiB, Cloud SQL zonal con IP privada,
> objetos en Cloud Storage, Redis en contenedor en el Worker Server. Nada de
> CDN, autoescalado, balanceador ni alta disponibilidad.
>
> **El plan está recortado a lo necesario para la entrega.** No montes Workload
> Identity Federation ni despliegue desde CI: están en «Lo que se deja fuera a
> propósito» y no los evalúa ningún criterio. Si crees que algo de esa lista
> hace falta, dímelo antes de escribirlo.
>
> Esta conversación cubre **A0 y A1**. Al terminar A1 paramos.
>
> Todo con infraestructura como código. `gcloud` solo para consultar y
> comprobar. La única excepción ya está hecha: el bucket del estado. Se opera
> con `make gcloud ARGS=...` y `make tf ENTORNO=entrega2 ARGS=...`; no hay
> gcloud ni terraform instalados en la máquina y no deben instalarse.
>
> Dos cosas que no se pueden olvidar: `google_project_service` con
> `disable_on_destroy = false`, o un `destroy` deja el proyecto inservible; y
> sin `servicenetworking` con su rango reservado, Cloud SQL no tiene IP privada,
> que es lo que el enunciado exige.
>
> Empieza comprobando con `make gcloud` qué hay en el proyecto antes de escribir
> HCL. Al final de cada fase corre su comprobación y enséñame el resultado antes
> de seguir. Si una comprobación no pasa, no avances.
