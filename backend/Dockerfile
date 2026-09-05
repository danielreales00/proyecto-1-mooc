# syntax=docker/dockerfile:1
#
# Una sola construcción, tres imágenes (ADR-0001). Los binarios comparten el
# árbol internal/ completo, así que comparten también las capas de compilación.

FROM golang:1.26-alpine AS builder
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

# --- migrate ----------------------------------------------------------------
FROM base AS migrate
COPY --from=builder /out/migrate /usr/local/bin/migrate
ENTRYPOINT ["migrate"]
