# Todo se ejecuta en contenedores: no hay toolchain de Go en la máquina.
SHELL := /bin/bash
COMPOSE := docker compose
GO_IMAGE := golang:1.26-alpine

.DEFAULT_GOAL := help

.PHONY: help
help: ## Muestra esta ayuda
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: env
env: ## Crea .env a partir de .env.example si no existe
	@test -f .env || (cp .env.example .env && echo ".env creado desde .env.example")

.PHONY: up
up: env ## Levanta el stack completo
	$(COMPOSE) up -d --build
	@echo ""
	@source .env; \
	echo "  API      http://localhost:$${API_PORT:-8090}/readyz"; \
	echo "  Mailpit  http://localhost:$${MAILPIT_UI_PORT:-8026}"; \
	echo "  MinIO    http://localhost:$${MINIO_CONSOLE_PORT:-9011}"

.PHONY: down
down: ## Detiene el stack (conserva los volúmenes)
	$(COMPOSE) down

.PHONY: clean
clean: ## Detiene el stack y borra los volúmenes
	$(COMPOSE) down -v

.PHONY: ps
ps: ## Estado de los servicios
	$(COMPOSE) ps

.PHONY: logs
logs: ## Sigue los logs de api y worker
	$(COMPOSE) logs -f api worker

.PHONY: build
build: ## Reconstruye las imágenes
	$(COMPOSE) build

.PHONY: migrate
migrate: ## Aplica las migraciones pendientes
	$(COMPOSE) run --rm migrate

.PHONY: tidy
tidy: ## Resuelve go.mod y go.sum dentro de un contenedor
	docker run --rm -v "$(PWD)/backend":/src -w /src \
		-v mooc-gomodcache:/go/pkg/mod $(GO_IMAGE) go mod tidy

.PHONY: fmt
fmt: ## Formatea el código
	docker run --rm -v "$(PWD)/backend":/src -w /src $(GO_IMAGE) gofmt -w .

.PHONY: vet
vet: ## Análisis estático
	docker run --rm -v "$(PWD)/backend":/src -w /src \
		-v mooc-gomodcache:/go/pkg/mod $(GO_IMAGE) go vet ./...

.PHONY: test
test: ## Pruebas unitarias con detector de carreras
	docker run --rm -v "$(PWD)/backend":/src -w /src \
		-v mooc-gomodcache:/go/pkg/mod $(GO_IMAGE) \
		sh -c "apk add --no-cache gcc musl-dev >/dev/null && go test -race ./..."

.PHONY: cover
cover: ## Pruebas con informe de cobertura
	docker run --rm -v "$(PWD)/backend":/src -w /src \
		-v mooc-gomodcache:/go/pkg/mod $(GO_IMAGE) \
		sh -c "go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out"

.PHONY: seed
seed: ## Carga datos sintéticos para la demostración (idempotente)
	$(COMPOSE) run --rm seed

.PHONY: smoke
smoke: ## Prueba de extremo a extremo: registro, verificación, login y /me
	@source .env; \
	API="http://localhost:$${API_PORT:-8090}" \
	MAILPIT="http://localhost:$${MAILPIT_UI_PORT:-8026}" ./scripts/smoke.sh

.PHONY: postman
postman: ## Ejecuta la colección de la demostración con newman
	@source .env; \
	docker run --rm --network host -v "$(PWD)/postman":/etc/newman \
		postman/newman:alpine run mooc.postman_collection.json \
		-e mooc.postman_environment.json \
		--env-var base_url=http://localhost:$${API_PORT:-8090} \
		--env-var mailpit_url=http://localhost:$${MAILPIT_UI_PORT:-8026} \
		--delay-request 300

.PHONY: scale
scale: ## Levanta 3 instancias de api y 3 de worker (CE-01)
	$(COMPOSE) up -d --scale api=3 --scale worker=3 --no-recreate

.PHONY: psql
psql: ## Consola de PostgreSQL
	$(COMPOSE) exec postgres psql -U $${POSTGRES_USER:-mooc} -d $${POSTGRES_DB:-mooc}

.PHONY: jobs
jobs: ## Muestra el registro de trabajos (outbox, idempotencia y DLQ)
	$(COMPOSE) exec postgres psql -U $${POSTGRES_USER:-mooc} -d $${POSTGRES_DB:-mooc} \
		-c "SELECT job_key, type, queue, status, attempt, last_error FROM platform.job_runs ORDER BY id DESC LIMIT 20;"

.PHONY: audit
audit: ## Muestra los últimos eventos de auditoría
	$(COMPOSE) exec postgres psql -U $${POSTGRES_USER:-mooc} -d $${POSTGRES_DB:-mooc} \
		-c "SELECT occurred_at, actor_role, action, entity_type, metadata FROM audit.events ORDER BY id DESC LIMIT 20;"
