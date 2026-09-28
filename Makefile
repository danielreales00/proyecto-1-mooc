# Todo se ejecuta en contenedores: no hay toolchain de Go en la máquina, y los
# scripts de scripts/ tampoco corren en ella. HERRAMIENTAS los mete en una
# imagen fija (ver scripts/Dockerfile) para que den lo mismo en Linux, en macOS
# y en Windows; las órdenes se escriben y se ven igual que siempre.
SHELL := /bin/bash
COMPOSE := docker compose
GO_IMAGE := golang:1.26-alpine

# Herramientas de seguridad con versión fija, por lo mismo que las imágenes: el
# CI y la máquina de cada quien deben ejecutar exactamente lo mismo. Subirlas es
# una decisión, no algo que pase solo un martes.
GOVULN_VERSION := v1.8.0
GOSEC_VERSION  := v2.29.0

# Cachés de Go. La de módulos ya estaba; faltaba LA DE COMPILACIÓN, que es la
# que de verdad pesa: sin ella cada contenedor recompila el árbol entero de
# dependencias desde cero, y por eso `vet`, `test` y `sec` tardaban lo mismo la
# primera vez que la décima. La de binarios evita reinstalar govulncheck y gosec
# en cada ejecución.
GO_CACHE := -v mooc-gomodcache:/go/pkg/mod -v mooc-gobuildcache:/root/.cache/go-build
EN_GO    := docker run --rm -v "$(PWD)/backend":/src -w /src $(GO_CACHE)

# El PDF de los informes de arquitectura sale de estas dos imágenes públicas.
# Versiones fijas por lo mismo que la imagen de herramientas: un clon nuevo debe
# producir el mismo PDF. Se descargan la primera vez que se usa `make informe`.
# La nube también se opera desde contenedores: en esta máquina no hay gcloud ni
# terraform instalados, y no van a hacer falta. La credencial vive en el volumen
# mooc-gcloud, nunca en el repositorio (ADR-0015, D3).
GCLOUD_IMAGE    := google/cloud-sdk:586.0.0-slim
TERRAFORM_IMAGE := hashicorp/terraform:1.16.4
GCP_PROYECTO    := mooc-509602
# -it solo cuando hay terminal: `gcloud auth login` la necesita y el CI no la
# tiene.
TTY := $(shell test -t 0 && printf -- '-it')
EN_NUBE := docker run --rm $(TTY) -v mooc-gcloud:/root/.config/gcloud -v "$(CURDIR)":/repo

MERMAID_IMAGE := minlag/mermaid-cli:11.4.2
PANDOC_IMAGE := pandoc/latex:3.5
EN_REPO := docker run --rm -u "$$(id -u):$$(id -g)" -v "$(CURDIR)":/data
HERRAMIENTAS := $(COMPOSE) run --rm herramientas

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
	$(COMPOSE) --profile tools build

.PHONY: migrate
migrate: ## Aplica las migraciones pendientes
	$(COMPOSE) run --rm migrate

.PHONY: tidy
tidy: ## Resuelve go.mod y go.sum dentro de un contenedor
	$(EN_GO) $(GO_IMAGE) go mod tidy

.PHONY: fmt
fmt: ## Formatea el código
	docker run --rm -v "$(PWD)/backend":/src -w /src $(GO_IMAGE) gofmt -w .

.PHONY: vet
vet: ## Análisis estático
	$(EN_GO) $(GO_IMAGE) go vet ./...

.PHONY: openapi
openapi: ## Valida el contrato OpenAPI
	docker run --rm -v "$(PWD)/backend/openapi":/spec -w /spec \
		redocly/cli:latest lint openapi.yaml

.PHONY: contrato
contrato: env ## Comprueba que el contrato y la API no se hayan separado
	@$(HERRAMIENTAS) ./scripts/contrato.sh

.PHONY: sec
sec: ## Análisis de seguridad: govulncheck y gosec (lo mismo que el CI)
	$(EN_GO) -v mooc-gobin:/go/bin $(GO_IMAGE) sh -c "\
		command -v govulncheck >/dev/null || go install golang.org/x/vuln/cmd/govulncheck@$(GOVULN_VERSION); \
		command -v gosec >/dev/null || go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION); \
		govulncheck ./... && gosec -exclude-generated -quiet ./... && \
		echo 'sin hallazgos'"
	@$(HERRAMIENTAS) ./scripts/sin-credenciales.sh

.PHONY: test
test: ## Pruebas unitarias con detector de carreras
	$(EN_GO) $(GO_IMAGE) \
		sh -c "apk add --no-cache gcc musl-dev >/dev/null && go test -race ./..."

.PHONY: cover
cover: ## Pruebas con informe de cobertura
	$(EN_GO) $(GO_IMAGE) \
		sh -c "go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out"

.PHONY: seed
seed: ## Carga datos sintéticos para la demostración (idempotente)
	$(COMPOSE) run --rm seed

.PHONY: smoke
smoke: env ## Prueba de extremo a extremo: registro, verificación, login y /me
	@$(HERRAMIENTAS) ./scripts/smoke.sh

.PHONY: postman
postman: env ## Colección de Postman, segmentos rápidos (lo que corre el CI)
	@source .env; \
	$(HERRAMIENTAS) newman run postman/mooc.postman_collection.json \
		-e postman/mooc.postman_environment.json --working-dir postman \
		--env-var base_url=http://localhost:$${API_PORT:-8090} \
		--env-var mailpit_url=http://localhost:$${MAILPIT_UI_PORT:-8026} \
		--folder "SEG-1 · Identidad" \
		--folder "SEG-1b · Administración" \
		--folder "SEG-2 · Autoría y publicación" \
		--folder "SEG-3 · Carga multimedia" \
		--folder "SEG-5 · Catálogo, inscripción y consumo" \
		--folder "SEG-6 · Quiz" \
		--delay-request 300

.PHONY: postman-completo
postman-completo: obs env ## Colección entera, incluidos progreso e insignia (tarda ~8 min)
	@echo "Los heartbeats exigen 10 s de separación, a propósito: esto tarda."
	@source .env; \
	$(HERRAMIENTAS) newman run postman/mooc.postman_collection.json \
		-e postman/mooc.postman_environment.json --working-dir postman \
		--env-var base_url=http://localhost:$${API_PORT:-8090} \
		--env-var mailpit_url=http://localhost:$${MAILPIT_UI_PORT:-8026} \
		--delay-request 11000

.PHONY: invariantes
invariantes: env ## Comprueba los invariantes de los ADR que no cubre `make demo`
	@$(HERRAMIENTAS) ./scripts/invariantes.sh

.PHONY: arch
arch: env ## Comprueba las reglas de arquitectura de los ADR 0001, 0002 y 0010
	@$(HERRAMIENTAS) ./scripts/arquitectura.sh

.PHONY: gcloud
gcloud: ## gcloud en contenedor: make gcloud ARGS="projects list"
	@$(EN_NUBE) -w /repo $(GCLOUD_IMAGE) gcloud $(ARGS)

.PHONY: prueba-gcs
prueba-gcs: ## Suite del almacén contra Cloud Storage real, firmando como mooc-web
	@# Usa el ADC del volumen mooc-gcloud. Firmar como mooc-web exige estar en
	@# firmantes_desarrollo (infra/environments/entrega2/terraform.tfvars).
	$(EN_GO) -v mooc-gcloud:/root/.config/gcloud:ro \
		-e GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json \
		-e GCS_PRUEBA_BUCKET=$(GCP_PROYECTO)-originals \
		-e GCS_PRUEBA_CUARENTENA=$(GCP_PROYECTO)-quarantine \
		-e GCS_SIGNER=mooc-web@$(GCP_PROYECTO).iam.gserviceaccount.com \
		$(GO_IMAGE) go test -count=1 -v ./internal/adapters/gcs/

REGISTRO := us-central1-docker.pkg.dev/$(GCP_PROYECTO)/mooc
IMAGENES := api worker worker-media migrate buckets seed

.PHONY: publicar
publicar: ## Construye las imágenes para linux/amd64 y las sube a Artifact Registry con el SHA
	@# La etiqueta es el commit: si hay cambios sin confirmar, mentiría sobre
	@# qué código lleva la imagen, y el informe registra esa versión por corrida.
	@test -z "$$(git status --porcelain)" \
		|| { echo "hay cambios sin confirmar; la etiqueta no correspondería al código"; exit 1; }
	@# Token de una hora del ADC del volumen: ninguna clave queda guardada. Sin
	@# $(TTY): con terminal, docker -t añadiría \r al token al pasar por el tubo.
	@docker run --rm -v mooc-gcloud:/root/.config/gcloud $(GCLOUD_IMAGE) gcloud auth print-access-token \
		| docker login -u oauth2accesstoken --password-stdin https://us-central1-docker.pkg.dev >/dev/null 2>&1
	@sha=$$(git rev-parse --short=12 HEAD); \
	for img in $(IMAGENES); do \
		echo "── $$img:$$sha"; \
		docker build -q --platform linux/amd64 --target $$img \
			-t $(REGISTRO)/$$img:$$sha backend >/dev/null || exit 1; \
		docker push -q $(REGISTRO)/$$img:$$sha \
			|| { docker logout us-central1-docker.pkg.dev >/dev/null; exit 1; }; \
	done; \
	docker logout us-central1-docker.pkg.dev >/dev/null

# La clave vive en el volumen mooc-ssh: sin él, cada contenedor generaría una
# nueva y la registraría en el perfil de OS Login de quien entra.
SSH_NUBE := docker run --rm $(TTY) -v mooc-gcloud:/root/.config/gcloud -v mooc-ssh:/root/.ssh \
	$(GCLOUD_IMAGE) gcloud compute ssh --zone us-central1-a --tunnel-through-iap --quiet

.PHONY: ssh
ssh: ## SSH por IAP: make ssh MAQUINA=mooc-worker [CMD="sudo docker ps"]
	@test -n "$(MAQUINA)" || { echo 'uso: make ssh MAQUINA=<mooc-web|mooc-worker> [CMD="..."]'; exit 1; }
	@$(SSH_NUBE) $(MAQUINA) $(if $(CMD),--command '$(CMD)')

# --- Verificación contra la nube (A4) ---------------------------------------
# Nombre público del Web Server: `make tf ENTORNO=entrega2 ARGS="output nombre_publico"`.
NUBE_HOST := 35-184-146-250.sslip.io
VERSION_IMAGENES := $(shell sed -n 's/^version_imagenes *= *"\(.*\)"/\1/p' infra/environments/entrega2/terraform.tfvars)
NUBE_RED  := mooc-nube
GCLOUD    := docker run --rm -v mooc-gcloud:/root/.config/gcloud $(GCLOUD_IMAGE) gcloud

.PHONY: tunel-mailpit
tunel-mailpit: ## Túnel SSH por IAP al Mailpit del Worker Server (contenedor mooc-tunel)
	@docker network inspect $(NUBE_RED) >/dev/null 2>&1 || docker network create $(NUBE_RED) >/dev/null
	@docker rm -f mooc-tunel >/dev/null 2>&1 || true
	@# Mailpit solo escucha en el 127.0.0.1 del Worker Server. El túnel lo trae
	@# a mooc-tunel:8025 dentro de la red $(NUBE_RED); no se abre ningún puerto.
	@# También en 127.0.0.1:8027 de esta máquina (8026 es el Mailpit local): Postman de escritorio lo lee
	@# con mailpit_url=http://localhost:8027.
	@docker run -d --name mooc-tunel --network $(NUBE_RED) -p 127.0.0.1:8027:8025 \
		-v mooc-gcloud:/root/.config/gcloud -v mooc-ssh:/root/.ssh $(GCLOUD_IMAGE) \
		gcloud compute ssh mooc-worker --zone us-central1-a --tunnel-through-iap --quiet \
		-- -N -o ServerAliveInterval=30 -o ExitOnForwardFailure=yes \
		-L 0.0.0.0:8025:127.0.0.1:8025 >/dev/null
	@for i in $$(seq 1 30); do \
		docker run --rm --network $(NUBE_RED) --entrypoint curl $(GCLOUD_IMAGE) \
			-fsS -o /dev/null http://mooc-tunel:8025/api/v1/info 2>/dev/null && { echo "túnel listo: mooc-tunel:8025"; exit 0; }; \
		sleep 2; done; echo "el túnel no respondió"; docker logs mooc-tunel | tail -5; exit 1

.PHONY: psql-nube
psql-nube: ## psql interactivo contra Cloud SQL, a través del Web Server
	@# La base solo tiene IP privada: se entra desde la máquina que la usa. La
	@# URL con la clave la lee allí mismo, del .env, y no pasa por aquí.
	@$(SSH_NUBE) mooc-web -- -t 'cd /opt/mooc && sudo docker run --rm -it postgres:17-alpine \
		psql "$$(sudo grep ^DATABASE_URL= .env | cut -d= -f2-)"'

.PHONY: semilla-nube
semilla-nube: ## Cuentas sintéticas en Cloud SQL, con la contraseña de Secret Manager
	@# Corre en el Web Server, que es quien llega a la IP privada de la base. La
	@# contraseña la lee la propia máquina con su identidad: no pasa por aquí.
	@# El .env es de root con 0600: se lee con sudo, no se carga en el shell.
	@# Sin pasar por `make ssh`: un make recursivo expandiría dos veces los $$.
	@# CARGA=1 crea además las cuentas de las pruebas de carga (600 y 10).
	@$(SSH_NUBE) mooc-web --command 'set -e; cd /opt/mooc; \
		img=$$(sudo grep ^REGISTRO= .env | cut -d= -f2)/seed:$$(sudo grep ^VERSION= .env | cut -d= -f2); \
		sudo docker run --rm --env-file .env -e SEED_FORCE=1 \
		$(if $(CARGA),-e SEED_CARGA_ESTUDIANTES=600 -e SEED_CARGA_PROFESORES=10) \
		-e SEED_PASSWORD="$$(sudo gcloud secrets versions access latest --secret=seed-password)" $$img'

# --- Pruebas de carga (C1) -----------------------------------------------------
# Todo corre en el generador (mooc-generador), fuera de las dos máquinas de la
# aplicación. k6 corre en segundo plano en la máquina: una sesión SSH de
# quince minutos por IAP se puede cortar, y la corrida no debe morir con ella.
K6_IMAGE  := grafana/k6:2.3.0
CARGA_DIR := /opt/carga

.PHONY: carga-sincronizar
carga-sincronizar: ## Copia los guiones de k6 y el generador de videos al generador
	@docker run --rm -v mooc-gcloud:/root/.config/gcloud -v mooc-ssh:/root/.ssh -v "$(CURDIR)":/repo:ro \
		$(GCLOUD_IMAGE) gcloud compute scp --zone us-central1-a --tunnel-through-iap --quiet \
		/repo/capacity-planning/k6/comun.js /repo/capacity-planning/k6/preparar.js \
		/repo/capacity-planning/k6/preparar-medios.js /repo/capacity-planning/k6/escenario1.js \
		/repo/capacity-planning/k6/escenario2.js /repo/capacity-planning/k6/rafaga-login.js \
		/repo/capacity-planning/k6/medios.js /repo/capacity-planning/medios/generar.sh \
		mooc-generador:/tmp/ >/dev/null
	@$(SSH_NUBE) mooc-generador --command 'sudo mv /tmp/generar.sh $(CARGA_DIR)/medios/ && sudo mv /tmp/*.js $(CARGA_DIR)/k6/ && ls $(CARGA_DIR)/k6'

.PHONY: carga-medios
carga-medios: ## Genera los tres videos en el generador, con el FFmpeg de worker-media
	@$(SSH_NUBE) mooc-generador --command 'sudo docker run --rm --user root -v $(CARGA_DIR)/medios:/m \
		--entrypoint sh $(REGISTRO)/worker-media:$(VERSION_IMAGENES) /m/generar.sh'

# Uso: make carga K6=escenario1.js ETIQUETA=e1-L2 ARGS="-e TASA=8 -e DURACION=8m"
.PHONY: carga
carga: ## Lanza una corrida de k6 en el generador, en segundo plano
	@test -n "$(K6)" -a -n "$(ETIQUETA)" || { echo 'uso: make carga K6=<guion.js> ETIQUETA=<nombre> [ARGS="-e ..."]'; exit 1; }
	@$(SSH_NUBE) mooc-generador --command 'set -e; \
		clave=$$(gcloud secrets versions access latest --secret=seed-password); \
		sudo docker rm -f k6-$(ETIQUETA) >/dev/null 2>&1 || true; \
		sudo docker run -d --name k6-$(ETIQUETA) --network host \
		-v $(CARGA_DIR)/k6:/k6:ro -v $(CARGA_DIR)/resultados:/resultados -v $(CARGA_DIR)/medios:/medios:ro \
		-e BASE_URL=https://$(NUBE_HOST) -e CLAVE="$$clave" -e ETIQUETA=$(ETIQUETA) $(ARGS) \
		$(K6_IMAGE) run -q --no-color /k6/$(K6) >/dev/null && echo "k6-$(ETIQUETA) en marcha desde $$(date -u +%FT%TZ)"'

.PHONY: carga-esperar
carga-esperar: ## Espera a que termine una corrida y muestra su resumen: make carga-esperar ETIQUETA=e1-L2
	@$(SSH_NUBE) mooc-generador --command 'sudo docker wait k6-$(ETIQUETA) >/dev/null; sudo docker logs k6-$(ETIQUETA) 2>&1 | grep -v level=info | tail -40'

.PHONY: carga-traer
carga-traer: ## Trae los resúmenes al repositorio, SIN los archivos con sesiones
	@mkdir -p capacity-planning/resultados
	@$(SSH_NUBE) mooc-generador --command 'cd $(CARGA_DIR)/resultados && sudo tar cz --exclude=datos.json --exclude=medios.json .' \
		| tar xz -C capacity-planning/resultados
	@ls capacity-planning/resultados

# Un nivel completo: corrida, espera, resumen y métricas de su misma ventana.
# Uso: make carga-nivel K6=escenario1.js ETIQUETA=e1-L2 ARGS="-e TASA=8 -e INTEGRIDAD=1"
.PHONY: carga-nivel
carga-nivel: ## Corre un nivel de carga y exporta sus métricas de la misma ventana
	@desde=$$(date -u +%FT%TZ); \
	$(MAKE) --no-print-directory carga K6=$(K6) ETIQUETA=$(ETIQUETA) ARGS='$(ARGS)' || exit 1; \
	$(MAKE) --no-print-directory carga-esperar ETIQUETA=$(ETIQUETA) 2>&1 | grep -v -E 'NumPy|increasing_the_tcp|^Existing|^$$'; \
	hasta=$$(date -u +%FT%TZ); echo "ventana: $$desde → $$hasta"; \
	$(MAKE) --no-print-directory carga-metricas ETIQUETA=$(ETIQUETA) DESDE=$$desde HASTA=$$hasta; \
	$(MAKE) --no-print-directory carga-traer >/dev/null

# Uso: make carga-metricas ETIQUETA=e1-L2 DESDE=2026-09-27T21:00:00Z HASTA=2026-09-27T21:10:00Z
.PHONY: carga-metricas
carga-metricas: ## Exporta de Cloud Monitoring las métricas de una corrida
	@test -n "$(ETIQUETA)" -a -n "$(DESDE)" -a -n "$(HASTA)" || { echo 'uso: make carga-metricas ETIQUETA=.. DESDE=.. HASTA=..'; exit 1; }
	@mkdir -p capacity-planning/resultados
	@docker run --rm -v mooc-gcloud:/root/.config/gcloud -v "$(CURDIR)/capacity-planning/metricas":/m:ro \
		--entrypoint python3 $(GCLOUD_IMAGE) /m/exportar.py $(ETIQUETA) $(DESDE) $(HASTA) \
		> capacity-planning/resultados/$(ETIQUETA)-metricas.json
	@echo "capacity-planning/resultados/$(ETIQUETA)-metricas.json"

# Uso: make carga-tabla ARGS="e1 e1-L0 e1-L1"   o   ARGS="detalle e1-L2"
.PHONY: carga-tabla
carga-tabla: ## Filas de las tablas de resultados a partir de los archivos de cada corrida
	@docker run --rm -v "$(CURDIR)/capacity-planning":/c:ro --entrypoint python3 $(GCLOUD_IMAGE) \
		/c/metricas/tabla.py $(ARGS)

# Uso: make carga-graficas ARGS="e1 e1-L0 e1-L1 e1-L2 e1-L3"  o  ARGS="serie e1-L3"
.PHONY: carga-graficas
carga-graficas: ## Gráficas del informe de capacidad a partir de los resultados
	@docker run --rm -u $$(id -u):$$(id -g) -e HOME=/tmp -e MPLCONFIGDIR=/tmp \
		-v "$(CURDIR)/capacity-planning":/c -w /c python:3.13-slim sh -c \
		"pip install -q --no-warn-script-location --user matplotlib==3.10.7 2>/dev/null && python metricas/graficas.py $(ARGS)"

.PHONY: postman-nube
postman-nube: tunel-mailpit ## Colección entera contra la URL pública (tarda ~8 min)
	@echo "Contra https://$(NUBE_HOST). Los heartbeats exigen 10 s de separación: esto tarda."
	@# La contraseña de las cuentas sintéticas se lee en el momento y viaja como
	@# variable de entorno, no como argumento: no queda en el historial ni en
	@# ningún archivo. prometheus_url vacía salta las tres peticiones de alertas,
	@# que son del stack local.
	@clave=$$($(GCLOUD) secrets versions access latest --secret=seed-password); \
	docker run --rm --network $(NUBE_RED) -e DEMO_PASSWORD="$$clave" \
		-v "$(CURDIR)":/repo -w /repo/postman --entrypoint sh mooc-herramientas -c \
		'newman run mooc-entrega2.postman_collection.json -e mooc-entrega2.postman_environment.json \
			--env-var mailpit_url=http://mooc-tunel:8025 \
			--env-var demo_password="$$DEMO_PASSWORD" \
			--delay-request 11000'; \
	rc=$$?; docker rm -f mooc-tunel >/dev/null 2>&1; exit $$rc

.PHONY: tf
tf: ## terraform de un entorno: make tf ENTORNO=dev ARGS="plan"
	@test -n "$(ENTORNO)" || { echo 'uso: make tf ENTORNO=<dev|prod> ARGS="plan"'; exit 1; }
	@test -d infra/environments/$(ENTORNO) \
		|| { echo "no existe infra/environments/$(ENTORNO)"; exit 1; }
	@# Terraform se autentica con las credenciales por defecto de la aplicación,
	@# las que deja `gcloud auth application-default login` en el mismo volumen.
	@$(EN_NUBE) -w /repo/infra/environments/$(ENTORNO) \
		-e GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json \
		$(TERRAFORM_IMAGE) $(ARGS)

FUENTE_INFORME = $(firstword $(wildcard docs/entrega$(ENTREGA)/informe-entrega-$(ENTREGA).md) arquitectura/informe-entrega-$(ENTREGA).md)

.PHONY: informe
informe: ## Genera el PDF de un informe: make informe ENTREGA=2
	@test -n "$(ENTREGA)" || { echo "uso: make informe ENTREGA=<n>"; exit 1; }
	@# Desde la Entrega 2 el informe vive donde lo pide la entrega,
	@# docs/entregaN/; el de la Entrega 1 sigue en arquitectura/.
	@test -f $(FUENTE_INFORME) || { echo "no existe $(FUENTE_INFORME)"; exit 1; }
	@mkdir -p arquitectura/recursos/informe-$(ENTREGA)
	@# 1. Cada bloque mermaid sale a un SVG y el Markdown intermedio lo enlaza.
	@$(EN_REPO) $(MERMAID_IMAGE) \
		-i $(FUENTE_INFORME) \
		-o arquitectura/recursos/informe-$(ENTREGA)/informe.md \
		-e pdf --pdfFit -b white
	@# 2. pandoc arma el PDF. Los diagramas ya son PDF vectoriales, que es lo
	@#    que LaTeX incrusta sin intermediarios.
	@$(EN_REPO) $(PANDOC_IMAGE) \
		arquitectura/recursos/informe-$(ENTREGA)/informe.md \
		-f markdown-implicit_figures \
		--lua-filter=scripts/informe.lua \
		--shift-heading-level-by=-1 \
		--resource-path=arquitectura/recursos/informe-$(ENTREGA) \
		-V lang=es -V geometry:a4paper,margin=2.5cm -V fontsize=11pt \
		-V colorlinks=true -V linkcolor=black -V urlcolor=black \
		-o $(basename $(FUENTE_INFORME)).pdf
	@echo "$(basename $(FUENTE_INFORME)).pdf"

.PHONY: rapido
rapido: ## Comprobación de bucle corto: formato, arquitectura, vet y pruebas
	@echo "── formato ──────────────────────────────────────────"
	@$(EN_GO) $(GO_IMAGE) gofmt -l . | tee /tmp/mooc-fmt
	@test ! -s /tmp/mooc-fmt || { echo "hay archivos sin formatear; ejecuta 'make fmt'"; exit 1; }
	@$(MAKE) --no-print-directory arch
	@echo "── go vet ───────────────────────────────────────────"
	@$(EN_GO) $(GO_IMAGE) go vet ./...
	@echo "── pruebas ──────────────────────────────────────────"
	@# Sin -race: cuesta dos o tres veces más y aquí interesa la vuelta rápida.
	@# El detector de carreras sigue corriendo en `make ci` y en el CI.
	@$(EN_GO) $(GO_IMAGE) go test ./...
	@echo
	@echo "Verde en lo rápido. Antes de empujar, 'make ci'."

.PHONY: ci
ci: ## Corre TODO lo que corre el CI, en local. Úsalo antes de empujar.
	@echo "── formato ──────────────────────────────────────────"
	@$(EN_GO) $(GO_IMAGE) gofmt -l . | tee /tmp/mooc-fmt
	@test ! -s /tmp/mooc-fmt || { echo "hay archivos sin formatear; ejecuta 'make fmt'"; exit 1; }
	@echo "── arquitectura ─────────────────────────────────────"
	@$(MAKE) --no-print-directory arch
	@echo "── go vet ───────────────────────────────────────────"
	@$(MAKE) --no-print-directory vet
	@echo "── pruebas ──────────────────────────────────────────"
	@$(MAKE) --no-print-directory test
	@echo "── contrato OpenAPI ─────────────────────────────────"
	@$(MAKE) --no-print-directory openapi
	@echo "── seguridad ────────────────────────────────────────"
	@$(MAKE) --no-print-directory sec
	@echo "── contrato contra la API en marcha ─────────────────"
	@$(MAKE) --no-print-directory contrato
	@echo "── extremo a extremo ────────────────────────────────"
	@$(MAKE) --no-print-directory smoke
	@$(MAKE) --no-print-directory invariantes
	@$(MAKE) --no-print-directory postman
	@echo
	@echo "Todo en verde. Se puede empujar."

.PHONY: obs
obs: ## Levanta Prometheus y Grafana (perfil observability)
	$(COMPOSE) --profile observability up -d
	@source .env; \
	echo "  Prometheus  http://localhost:$${PROMETHEUS_PORT:-9091}"; \
	echo "  Grafana     http://localhost:$${GRAFANA_PORT:-3002}  (panel: MOOC · Operación)"

.PHONY: subir
subir: env ## Sube un archivo de prueba: make subir ARCHIVO=video.mp4
	@test -n "$(ARCHIVO)" || { echo "uso: make subir ARCHIVO=<ruta>"; exit 1; }
	@test -f "$(ARCHIVO)" || { echo "no existe el archivo: $(ARCHIVO)"; exit 1; }
	@# El archivo puede estar en cualquier parte de la máquina; el contenedor
	@# solo ve /repo, así que se le monta además la carpeta que lo contiene.
	@$(COMPOSE) run --rm \
		-v "$$(cd "$$(dirname "$(ARCHIVO)")" && pwd)":/entrada:ro \
		herramientas ./scripts/subir-video.sh "/entrada/$$(basename "$(ARCHIVO)")"

.PHONY: demo
demo: env ## Recorre el flujo completo de la demostración (guion del video)
	@$(HERRAMIENTAS) ./scripts/demo.sh

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
