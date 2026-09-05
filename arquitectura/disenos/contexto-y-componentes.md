# Contexto y componentes

## Contexto

```mermaid
graph LR
  EST[Estudiante]
  PROF[Profesor]
  ADM[Administrador]
  PUB[Visitante<br/>verifica una insignia]

  subgraph plataforma[Plataforma MOOC]
    API[API REST /api/v1]
    W[Workers]
  end

  CORREO[(Servidor de correo<br/>Mailpit en local)]

  EST --> API
  PROF --> API
  ADM --> API
  PUB --> API
  W --> CORREO
```

En la Entrega 1 los tres roles interactúan mediante **Postman**; el frontend
llega en la Entrega 2.

## Componentes

```mermaid
graph TB
  CLI[Cliente<br/>Postman en E1]

  subgraph borde[Borde]
    PROXY[Proxy / TLS<br/>Caddy]
  end

  subgraph app[Aplicación sin estado]
    API["api (N instancias)<br/>cmd/api"]
    WG["worker (N instancias)<br/>cmd/worker"]
    WM["worker-media (N instancias)<br/>cmd/worker + ffmpeg"]
    MIG["migrate<br/>job de arranque"]
  end

  subgraph datos[Estado]
    PG[(PostgreSQL 17<br/>fuente de verdad)]
    RD[(Redis<br/>0 cola · 1 sesiones<br/>2 caché · 3 rate limit)]
    S3[(MinIO<br/>originals · derived<br/>badges · quarantine)]
  end

  subgraph apoyo[Servicios de apoyo]
    CLAM[ClamAV]
    MAIL[Mailpit]
  end

  subgraph obs[Observabilidad]
    PROM[Prometheus]
    GRAF[Grafana]
    JAEG[Jaeger]
  end

  CLI --> PROXY --> API
  CLI -. "PUT/GET prefirmado<br/>los bytes no pasan por la API" .-> S3

  API --> PG
  API --> RD
  API --> S3
  MIG --> PG

  RD --> WG
  RD --> WM
  WG --> PG
  WG --> S3
  WG --> MAIL
  WM --> PG
  WM --> S3
  WM --> CLAM

  API -.métricas.-> PROM
  WG -.métricas.-> PROM
  WM -.métricas.-> PROM
  API -.trazas.-> JAEG
  WG -.trazas.-> JAEG
  WM -.trazas.-> JAEG
  PROM --> GRAF
```

Comparar con `../recursos/diagrama-enunciado.png`: es el mismo esquema, con los
servicios de apoyo y observabilidad que el enunciado exige en otras secciones.

## Módulos del monolito

Frontera lógica del ADR-0001. Un módulo solo llama a otro por su interfaz de
servicio; nunca por sus tablas.

| Módulo | Responsabilidad | Depende de |
| --- | --- | --- |
| `identity` | Registro, verificación, login, sesiones, contraseñas | `audit`, cola (correo) |
| `admin` | Usuarios, roles, estados, sesiones ajenas, configuración | `identity`, `audit` |
| `authoring` | Cursos, versiones, módulos, unidades, recursos, publicación | `media`, `assessment`, `audit` |
| `media` | Assets, carga multipart, escaneo, transcodificación, URLs firmadas | cola, `audit` |
| `assessment` | Quizzes, intentos, calificación | `authoring` (solo lectura vía servicio) |
| `catalog` | Búsqueda, filtros, ficha pública del curso | `authoring` (lectura) |
| `enrollment` | Inscripción, retiro, reinscripción | `catalog`, `progress` |
| `progress` | Evidencias, cálculo, estados de la inscripción | `authoring`, `assessment`, cola (insignia) |
| `badges` | Emisión, verificación pública, revocación | `progress`, `media`, cola |
| `audit` | Registro inmutable de eventos | — |

Dependencias que serían circulares se rompen con un evento en la cola. Por
ejemplo, `progress` no llama a `badges`: encola `badge.issue`.

## Cómo se escala

La API **no publica puerto propio**: la única entrada es el proxy, que resuelve
`api` por DNS de Docker y reparte por turnos entre las instancias. Sin eso,
`--scale api=3` chocaría por el puerto del host.

| Componente | Cómo | Límite |
| --- | --- | --- |
| `api` | `--scale api=N` tras el proxy | El pool de conexiones a PostgreSQL |
| `worker` | `--scale worker=N` | Contención en `job_runs`, despreciable |
| `worker-media` | `--scale worker-media=N` | CPU de la máquina |
| PostgreSQL | Vertical; réplica de lectura en GCP | Fuente de verdad, no se reparte |
| Redis | Una instancia con AOF | Suficiente a esta escala |

Nada guarda estado en el proceso: ni caché local, ni sesiones, ni ficheros que
sobrevivan a la petición o al job (ADR-0001, ADR-0006).

## Servicios de Docker Compose

| Servicio | Imagen | Perfil |
| --- | --- | --- |
| `proxy` | `caddy:2-alpine` | por defecto |

| `api` | construida (`target: api`) | por defecto |
| `worker` | construida (`target: worker`) | por defecto |
| `worker-media` | construida (`target: worker-media`, con ffmpeg) | por defecto |
| `migrate` | construida (`target: migrate`) | por defecto, `restart: no` |
| `postgres` | `postgres:17-alpine` | por defecto |
| `redis` | `redis:7-alpine` (`--appendonly yes`) | por defecto |
| `minio` | `minio/minio` | por defecto |
| `minio-init` | `minio/mc` (crea buckets y políticas) | por defecto, `restart: no` |
| `mailpit` | `axllent/mailpit` | por defecto |
| `clamav` | `clamav/clamav` | por defecto |
| `prometheus` | `prom/prometheus` | `observability` |
| `grafana` | `grafana/grafana-oss` | `observability` |
| `jaeger` | `jaegertracing/all-in-one` | `observability` |
| `k6` | `grafana/k6` | `carga` |

Orden de arranque por healthcheck: `postgres`+`redis`+`minio` → `migrate` →
`api`/`worker`. `clamav` bloquea solo a `worker-media`.
