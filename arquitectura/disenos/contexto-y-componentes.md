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

Lo que existe y corre hoy. Cada caja es un servicio de `docker-compose.yml`.

```mermaid
graph TB
  CLI[Cliente<br/>Postman en E1]

  subgraph borde[Borde · único puerto publicado]
    PROXY[proxy<br/>Caddy · :8090]
    SW[swagger<br/>contrato en /docs]
  end

  subgraph app[Aplicación sin estado]
    API["api (N instancias)<br/>cmd/api"]
    WG["worker (N instancias)<br/>cmd/worker"]
    MIG["migrate<br/>job de arranque"]
  end

  subgraph datos[Estado]
    PG[(PostgreSQL 17<br/>fuente de verdad)]
    RD[(Redis<br/>0 cola · 1 sesiones<br/>2 caché · 3 rate limit)]
    S3[(MinIO<br/>originals · derived<br/>badges · quarantine)]
  end

  subgraph apoyo[Servicios de apoyo]
    MAIL[mailpit<br/>SMTP local]
  end

  subgraph obs[Observabilidad]
    PROM[prometheus]
    GRAF[grafana]
  end

  CLI --> PROXY
  PROXY --> API
  PROXY --> SW
  CLI -. "PUT/GET prefirmado<br/>los bytes no pasan por la API" .-> S3

  API --> PG
  API --> RD
  API --> S3
  MIG --> PG

  RD -- "cola 0" --> WG
  WG --> PG
  WG --> S3
  WG --> MAIL

  PROM -. "raspa /metrics" .-> API
  PROM -. "raspa /metrics" .-> WG
  GRAF --> PROM
```

**Cómo leerlo:**

- **El proxy es la única puerta.** Ni la API ni el worker publican puerto en la
  máquina, y por eso `--scale api=3 --scale worker=3` no colisiona (CE-01).
  Caddy resuelve `api` por DNS y reparte en round robin.
- **Los bytes no pasan por la API.** La flecha punteada al almacén es el cliente
  subiendo y bajando con URLs prefirmadas; la API solo las firma.
- **La API nunca llama al worker.** Escribe el trabajo en PostgreSQL, en la misma
  transacción que el cambio de dominio, y lo publica en la cola **después** del
  commit. De ahí el `202` y no el `201`. → `adr/0008`
- **Prometheus raspa**, no recibe. Descubre las instancias por DNS, así que
  escalar no exige tocar su configuración.
- Quedan fuera dos contenedores de arranque que corren una vez y terminan:
  `minio-init`, que crea los cuatro buckets y deja `mooc-badges` con lectura
  pública, y `seed`, que carga las cuentas sintéticas de la demostración.

### Trabajos que ejecuta el worker

| Trabajo | Qué hace | Escribe en |
| --- | --- | --- |
| `email.send` | Verificación de correo e invitaciones de profesor | Mailpit, PostgreSQL |
| `badge.issue` | Emite la insignia al aprobar una inscripción | PostgreSQL, MinIO |
| `media.probe` | Recalcula el SHA-256 leyendo del bucket y detecta el MIME real | PostgreSQL, MinIO |
| `jobs.reaper` | Cada 30 s republica los trabajos huérfanos | PostgreSQL |

### Lo que este diagrama no tiene

Está decidido y documentado, pero **no implementado**. No aparece aquí para que
el diagrama no prometa de más:

| Falta | Dónde está decidido |
| --- | --- |
| `worker-media` con FFmpeg y transcodificación a HLS | `../adr/0011-pipeline-de-medios-con-ffmpeg-y-clamav.md` |
| ClamAV y el trabajo `media.scan` | `../adr/0011-…`, `../media-plan.md` |
| Trazas distribuidas (Jaeger / OpenTelemetry) | `../pendientes.md` |

El esquema con todo eso, que es el del enunciado, está en
`../recursos/diagrama-enunciado.png`.

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
| `minio` | `quay.io/minio/minio` (etiqueta fija) | por defecto |
| `minio-init` | `quay.io/minio/mc` (crea buckets y políticas) | por defecto, `restart: no` |
| `mailpit` | `axllent/mailpit` | por defecto |
| `clamav` | `clamav/clamav` | por defecto |
| `prometheus` | `prom/prometheus` | `observability` |
| `grafana` | `grafana/grafana-oss` | `observability` |
| `jaeger` | `jaegertracing/all-in-one` | `observability` |
| `k6` | `grafana/k6` | `carga` |

Orden de arranque por healthcheck: `postgres`+`redis`+`minio` → `migrate` →
`api`/`worker`. `clamav` bloquea solo a `worker-media`.
