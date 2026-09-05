# ADR-0010 — Portabilidad a GCP sin acoplarse a GCP

- **Estado:** Aceptado
- **Fecha:** 2026-09-05
- **Requisitos:** `RT-01`, `RT-04`, `CE-01`
- **Refinado por:** ADR-0015 (Terraform y provisión de GCP)

## Contexto

La Entrega 1 exige Docker Compose. Entregas posteriores despliegan en **GCP**. El
enunciado pide además que el dominio esté desacoplado del proveedor cloud (§7).
El riesgo real es doble: decidir hoy algo que obligue a reescribir en E3, o
sobre-abstraer por una nube que todavía no se usa.

## Decisión

**No se escribe código específico de GCP en la Entrega 1.** En su lugar se
respetan cuatro reglas que hacen el traslado barato, y se documenta el mapeo en
`disenos/ruta-a-gcp.md`.

### Reglas

1. **Todo lo externo pasa por un puerto del dominio.** `objectstore.Store`,
   `mailer.Sender`, `antivirus.Scanner`, `queue.Publisher`, `transcoder.Encoder`.
   Cambiar de proveedor es escribir un adaptador nuevo bajo `internal/adapters/`.
2. **Toda la configuración viene del entorno**, sin valores por defecto de
   producción en el código y sin ficheros de configuración por entorno. Un
   despliegue distinto es un conjunto de variables distinto.
3. **El proceso escribe solo a `stdout`/`stderr`** y no guarda nada en disco que
   deba sobrevivir a la petición. Es lo que exige un contenedor efímero como
   Cloud Run.
4. **Nada de credenciales en la imagen.** En local, `.env`; en GCP, Secret
   Manager inyectado como variable. El código solo ve `os.Getenv`.

### Mapeo previsto

| Local (E1) | GCP (E3+) | Cómo cambia el código |
| --- | --- | --- |
| Contenedor `api` | Cloud Run (servicio) | Nada; escuchar en `$PORT` |
| Contenedor `worker` | Cloud Run (worker pool) o MIG | Nada |
| PostgreSQL en contenedor | Cloud SQL para PostgreSQL | Cadena de conexión |
| Redis en contenedor | Memorystore for Redis | Cadena de conexión |
| MinIO | Cloud Storage | Reusar el adaptador S3 contra el endpoint de interoperabilidad, o adaptador `gcs` nuevo |
| MinIO público | Cloud CDN + balanceador | Cambia la emisión de URL: de firma S3 a cookie firmada de CDN |
| Mailpit | SendGrid / servicio SMTP | Adaptador `mailer` |
| Jaeger | Cloud Trace (OTLP) | Endpoint del exportador |
| Prometheus + Grafana | Cloud Monitoring | Endpoint |
| `.env` | Secret Manager | Nada |
| ClamAV en contenedor | ClamAV en el worker, o Cloud Run job | Nada |

### Lo que se decide *ahora* para no arrepentirse después

- **Cloud Run como destino previsto**, no GKE: no hay estado local que lo impida
  (ADR-0001) y el costo operativo para 4 personas es mucho menor.
- Las URL firmadas se emiten desde un único paquete (`adapters/objectstore`), de
  modo que pasar a cookies firmadas de CDN toque un solo sitio.
- La migración de esquema corre como **job separado** (ADR-0003), que es
  exactamente la forma en que Cloud Run espera ejecutarla.
- Los binarios se compilan para `linux/amd64` con `CGO_ENABLED=0`, salvo el
  worker de FFmpeg, que necesita la imagen con el binario de FFmpeg.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Usar SDKs de GCP desde ya | Menos trabajo en E3 | La entrega actual debe correr en Compose; añade emuladores y credenciales | Complica lo que hay que entregar ahora |
| Terraform desde E1 | Infra reproducible | No hay infra que provisionar todavía | Se hace en E3. ADR-0015 mantiene esto, pero separa las decisiones que sí hay que tomar ya |
| Abstraer también PostgreSQL y Redis | Portabilidad total | Cloud SQL y Memorystore hablan el mismo protocolo | Abstracción sin beneficio |

## Consecuencias

- E3 será mayormente configuración e infraestructura, no reescritura.
- Se paga una capa de interfaces que en E1 tiene una sola implementación; es
  además lo que permite sustituirla por un doble en las pruebas.
- Hay que resistir la tentación de "probar rápido en GCP" saltándose el puerto.

## Cómo se verifica

- `grep -r "cloud.google.com" backend/internal/modules/` no devuelve nada.
- Los puertos tienen una implementación falsa en pruebas, lo que demuestra que la
  sustitución funciona.
