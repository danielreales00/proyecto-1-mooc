# syntax=docker/dockerfile:1
#
# Una sola construcción, tres imágenes (ADR-0001). Los binarios comparten el
# árbol internal/ completo, así que comparten también las capas de compilación.

FROM golang:1.26.10-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

# Dependencias congeladas en el repositorio: la construcción es reproducible y
# esta capa se reutiliza mientras go.mod y go.sum no cambien.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

# --- base común -------------------------------------------------------------
FROM alpine:3.20 AS base
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 -h /app app
WORKDIR /app
USER app

# --- api --------------------------------------------------------------------
FROM base AS api
COPY --from=builder /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["api"]

# --- worker -----------------------------------------------------------------
FROM base AS worker
COPY --from=builder /out/worker /usr/local/bin/worker
EXPOSE 8081
ENTRYPOINT ["worker"]

# --- worker-media -----------------------------------------------------------
# El mismo binario que `worker`, en una imagen que además trae FFmpeg.
#
# Va aparte porque FFmpeg pesa más que todo lo demás junto, y el worker normal
# no lo necesita: la transcodificación viaja por la cola `bulk`, que solo
# consume este servicio (ADR-0011). Así el worker de correo e insignias sigue
# arrancando en segundos.
FROM base AS worker-media
USER root
RUN apk add --no-cache ffmpeg
USER app
COPY --from=builder /out/worker /usr/local/bin/worker
EXPOSE 8081
ENTRYPOINT ["worker"]

# --- workers-cloudrun -------------------------------------------------------
# El servicio de workers de Cloud Run (ADR-0020, D4): un solo contenedor con el
# worker, FFmpeg y ClamAV. Parte de la imagen oficial de ClamAV porque trae las
# firmas: el mismo tag da el mismo veredicto en local y en la nube (ADR-0011).
# clamd corre como el usuario clamav y el worker como root del contenedor, que
# en Cloud Run no tiene privilegios sobre la máquina.
FROM clamav/clamav:1.4 AS workers-cloudrun
RUN apk add --no-cache ffmpeg ca-certificates tzdata
COPY cloudrun/clamd.conf /etc/clamav/clamd.conf
COPY cloudrun/arranque-worker.sh /usr/local/bin/arranque-worker.sh
COPY --from=builder /out/worker /usr/local/bin/worker
ENV QUEUE_BACKEND=pubsub CLAMAV_ADDR=127.0.0.1:3310
EXPOSE 8080
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/arranque-worker.sh"]

# --- migrate ----------------------------------------------------------------
FROM base AS migrate
COPY --from=builder /out/migrate /usr/local/bin/migrate
ENTRYPOINT ["migrate"]

# --- buckets ----------------------------------------------------------------
# Deja el almacén de objetos listo. Sustituye al contenedor de `mc`, que dejó de
# poder descargarse cuando MinIO cerró el acceso anónimo a sus imágenes.
FROM base AS buckets
COPY --from=builder /out/buckets /usr/local/bin/buckets
ENTRYPOINT ["buckets"]

# --- seed -------------------------------------------------------------------
# Datos sintéticos para la demostración (§10.1). Solo desarrollo.
FROM base AS seed
COPY --from=builder /out/seed /usr/local/bin/seed
ENTRYPOINT ["seed"]
