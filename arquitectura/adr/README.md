# Decisiones de arquitectura (ADR)

Un archivo por decisión, numerado y no reutilizable. Un ADR no se borra ni se
reescribe en el fondo: se marca **Reemplazado por ADR-NNNN** y se escribe uno
nuevo. Así queda la historia de por qué el proyecto es como es, que es lo que se
pregunta en la sustentación.

**Estados:** Propuesto · Aceptado · Reemplazado · Rechazado.

## Índice

| # | Decisión | Estado | Requisitos |
| --- | --- | --- | --- |
| [0001](0001-monolito-modular-en-go-con-workers-independientes.md) | Monolito modular en Go con workers independientes | Aceptado | `RT-01`, `CE-01` |
| [0002](0002-dependencias-minimas-y-stdlib-para-http.md) | Dependencias mínimas y `net/http` para el ruteo | Aceptado | `RT-01`, `RT-05` |
| [0003](0003-postgresql-con-migraciones-sql-versionadas.md) | PostgreSQL como fuente de verdad, migraciones SQL versionadas | Aceptado | `RT-03`, `RNF-03`, `RNF-04` |
| [0004](0004-cola-asincrona-con-redis-y-asynq.md) | Cola asíncrona con Redis y asynq | Aceptado | `CA-02`, `CA-03`, `RNF-06` |
| [0005](0005-almacenamiento-de-objetos-compatible-con-s3.md) | Almacenamiento de objetos compatible con S3 tras un puerto propio | Aceptado | `RF-05`, `RF-06`, `CA-06` |
| [0006](0006-sesiones-opacas-en-redis.md) | Sesiones opacas en Redis, no JWT de sesión | Aceptado | `RF-01`, `RF-02`, `CE-02` |
| [0007](0007-versiones-inmutables-e-identificadores-estables.md) | Versiones de curso inmutables e identificadores estables | Aceptado | `RF-03`, `CA-01`, `CE-03` |
| [0008](0008-idempotencia-en-la-api-y-en-los-workers.md) | Idempotencia en la API y en los workers | Aceptado | `CA-03`, `CA-04`, `CA-07` |
| [0009](0009-observabilidad-y-errores-uniformes.md) | Observabilidad y formato uniforme de errores | Aceptado | `RT-05`, `RT-06`, `CE-07` |
| [0010](0010-portabilidad-a-gcp.md) | Portabilidad a GCP sin acoplarse a GCP | Aceptado | `RT-01`, `CE-01` |
| [0011](0011-pipeline-de-medios-con-ffmpeg-y-clamav.md) | Pipeline de medios: multipart, ClamAV y HLS con FFmpeg | Aceptado | `RF-05`, `RF-06`, `CA-02` |
| [0012](0012-progreso-verificado-en-el-servidor.md) | Progreso verificado en el servidor | Aceptado | `RF-09`, `CA-05`, `CE-06` |
| [0013](0013-integridad-del-quiz.md) | Integridad del quiz: snapshot y clave exclusiva del servidor | Aceptado | `RF-08`, `CA-04`, `CE-05` |
| [0014](0014-markdown-extendido-canonico.md) | Markdown extendido canónico | Aceptado | `RF-04`, `CE-03` |

## Decisiones aún no tomadas

Están en `../pendientes.md`. Cuando una se resuelva, se convierte en ADR.

## Cómo escribir uno

Copiar `0000-plantilla.md` con el número siguiente. El nombre del archivo
describe la decisión, no el área: `0015-cache-de-catalogo-en-redis.md`, no
`0015-catalogo.md`.

Un ADR bueno responde a "¿por qué no lo hicieron de la otra forma?". Si la
sección de alternativas está vacía, probablemente no había decisión que tomar y
no hacía falta el ADR.
