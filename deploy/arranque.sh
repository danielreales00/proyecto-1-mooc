#!/usr/bin/env bash
# Script de arranque de todas las máquinas de la aplicación (ADR-0016, D7, y
# ADR-0019, D7). Lo ejecuta el agente de GCE como root en cada arranque;
# Terraform lo entrega por los metadatos de la plantilla, o de la instancia en
# el caso de la cola, junto con la composición y la configuración.
#
# Papeles (metadato mooc-rol):
#   web      instancia del grupo de API: Caddy, API, frontend, Swagger
#   worker   instancia del grupo de workers: worker, worker-media, ClamAV
#   cola     la máquina de la cola: Redis, Mailpit y el publicador de la señal
#   hornear  máquina temporal de make hornear-imagen: instala, descarga las
#            imágenes y se apaga, para que su disco sea la imagen base
#
# Es idempotente: instala lo que falte, reescribe la configuración y deja la
# composición levantada. Un reinicio no duplica contenedores ni temporizadores.
# Sobre la imagen horneada, Docker y el agente ya están y las imágenes ya están
# descargadas: el arranque solo escribe la configuración y levanta.
#
# No corre en la máquina de nadie del equipo, así que no va en la imagen de
# herramientas de scripts/: su único entorno es Debian 12 en GCE.
set -euo pipefail

DIR=/opt/mooc
REGISTRO_HOST=us-central1-docker.pkg.dev

md() {
  curl -fsS -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1"
}

ROL=$(md mooc-rol)
inicio=$(date +%s)
echo "mooc: arranque del rol ${ROL} en $(hostname)"

# --- Docker -----------------------------------------------------------------
# Del repositorio de Docker y no del de Debian: el de Debian trae Compose v1,
# que no entiende `service_completed_successfully`.
if ! command -v docker >/dev/null || ! docker compose version >/dev/null 2>&1; then
  apt-get update -q
  apt-get install -y -q ca-certificates curl
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
  . /etc/os-release
  echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian ${VERSION_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -q
  apt-get install -y -q docker-ce docker-ce-cli containerd.io docker-compose-plugin
fi

# --- Agente de operaciones --------------------------------------------------
# Memoria y disco no están en las métricas por defecto de Compute Engine, y el
# informe de capacidad las exige.
if ! systemctl is-active --quiet google-cloud-ops-agent; then
  curl -fsSL https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh \
    -o /tmp/add-ops-agent-repo.sh
  bash /tmp/add-ops-agent-repo.sh --also-install
fi

# El ayudante de credenciales de gcloud pide un token a la identidad de la
# instancia en cada descarga: no hay clave ni contraseña guardada.
gcloud auth configure-docker "$REGISTRO_HOST" --quiet

# --- Horneado de la imagen base ---------------------------------------------
# Las imágenes de contenedor quedan en el disco: una instancia nueva no las
# descarga, y su arranque deja de depender de Docker Hub en mitad de una
# prueba. La lista la pasa scripts/hornear-imagen.sh.
if [ "$ROL" = hornear ]; then
  for img in $(md mooc-imagenes); do
    echo "mooc: descargando ${img}"
    docker pull --quiet "$img"
  done
  # Que la máquina horneada no arrastre configuración de nadie.
  rm -rf "$DIR"
  echo "mooc: imagen lista; se apaga para crearla"
  poweroff
  exit 0
fi

# Métricas de la aplicación y de Redis al mismo sitio que CPU, memoria y disco.
# En la nube no hay Prometheus: el agente rasca /metrics de cada proceso (solo
# publicado en la máquina) y lo envía a Cloud Monitoring, donde se consulta con
# PromQL. Solo las series propias: se paga por muestra.
objetivos() {
  case "$ROL" in
    web)    echo "['127.0.0.1:8080']" ;;
    worker) echo "['127.0.0.1:8081', '127.0.0.1:8082']" ;;
  esac
}
{
  echo "metrics:"
  echo "  receivers:"
  if [ "$ROL" = cola ]; then
    cat <<REDIS
    redis:
      type: redis
      address: $(md mooc-ip-privada):6379
      collection_interval: 30s
  service:
    pipelines:
      mooc:
        receivers: [redis]
REDIS
  else
    cat <<PROM
    mooc:
      type: prometheus
      config:
        scrape_configs:
          - job_name: mooc-${ROL}
            scrape_interval: 30s
            static_configs:
              - targets: $(objetivos)
            metric_relabel_configs:
              - source_labels: [__name__]
                regex: '(http_request|jobs?_|queue_|rate_limited|progress_rejected).*'
                action: keep
  service:
    pipelines:
      mooc:
        receivers: [mooc]
PROM
  fi
} > /etc/google-cloud-ops-agent/config.yaml.nuevo
if ! cmp -s /etc/google-cloud-ops-agent/config.yaml.nuevo /etc/google-cloud-ops-agent/config.yaml; then
  mv /etc/google-cloud-ops-agent/config.yaml.nuevo /etc/google-cloud-ops-agent/config.yaml
  systemctl restart google-cloud-ops-agent
else
  rm /etc/google-cloud-ops-agent/config.yaml.nuevo
fi

# --- Configuración ----------------------------------------------------------
mkdir -p "$DIR"
md mooc-compose > "$DIR/compose.yml"
caddy_antes=$(sha256sum "$DIR/Caddyfile" 2>/dev/null || true)
case "$ROL" in
  web)    md mooc-caddyfile > "$DIR/Caddyfile" ;;
  worker) md mooc-clamd > "$DIR/clamd.conf" ;;
  cola)   md mooc-publicador > "$DIR/publicar-cola.sh"; chmod 755 "$DIR/publicar-cola.sh" ;;
esac

# El .env lleva los secretos: se escribe con permisos 0600, fuera de cualquier
# imagen, leyendo Secret Manager con la identidad de la instancia. Se escribe
# aparte y se mueve, para que nunca exista a medias con otros permisos. La
# cola no lee ningún secreto.
secreto() {
  gcloud secrets versions access latest --secret="$1" --quiet 2>/dev/null || true
}
umask 077
{
  md mooc-env
  # El nombre de la máquina, que Caddy devuelve en X-Instancia.
  echo "INSTANCIA=$(hostname)"
  if [ "$ROL" != cola ]; then
    echo "DATABASE_URL=$(secreto database-url)"
  fi
  if [ "$ROL" = worker ]; then
    echo "SMTP_PASSWORD=$(secreto smtp-password)"
  fi
} > "$DIR/.env.nuevo"
chmod 600 "$DIR/.env.nuevo"
mv "$DIR/.env.nuevo" "$DIR/.env"
umask 022

if [ "$ROL" != cola ] && ! grep -q '^DATABASE_URL=.' "$DIR/.env"; then
  echo "mooc: no se pudo leer database-url de Secret Manager" >&2
  exit 1
fi

# --- Publicador de la señal de escalado (solo la cola) ----------------------
# Un temporizador y no un bucle: systemd lo relanza si falla, no se duplica al
# reiniciar y su salida queda en el diario con fecha.
if [ "$ROL" = cola ]; then
  cat > /etc/systemd/system/mooc-publicar-cola.service <<UNIDAD
[Unit]
Description=Publica la profundidad de la cola bulk en Cloud Monitoring
After=docker.service

[Service]
Type=oneshot
ExecStart=${DIR}/publicar-cola.sh
UNIDAD
  cat > /etc/systemd/system/mooc-publicar-cola.timer <<UNIDAD
[Unit]
Description=Cada 30 s, la señal del autoscaler de workers

[Timer]
OnBootSec=30s
OnUnitActiveSec=30s
AccuracySec=1s

[Install]
WantedBy=timers.target
UNIDAD
  systemctl daemon-reload
  systemctl enable --now mooc-publicar-cola.timer
fi

# --- Imágenes y arranque ----------------------------------------------------
cd "$DIR"
docker compose pull --quiet

# Las instancias dependen de la cola, que vive en otra máquina, y Compose solo
# sabe esperar a servicios de la suya: si la cola arranca después, la API no
# pasa su healthcheck y `up` aborta. Se reintenta hasta que esté lista; diez
# minutos cubren su arranque en frío.
for intento in $(seq 1 60); do
  if docker compose up -d --remove-orphans --wait --wait-timeout 120; then
    # Caddy corre con la API de administración apagada y no recarga solo: si
    # su configuración cambió, se reinicia el proxy.
    if [ "$ROL" = web ] && [ "$caddy_antes" != "$(sha256sum "$DIR/Caddyfile")" ]; then
      docker compose restart proxy
    fi
    # El tiempo de arranque, para el informe: desde que el agente lanzó este
    # script hasta que todos los contenedores pasaron su healthcheck.
    echo "mooc: ${ROL} levantado en $(( $(date +%s) - inicio )) s"
    exit 0
  fi
  echo "mooc: intento ${intento} sin éxito; se reintenta en 10 s"
  sleep 10
done
echo "mooc: ${ROL} no levantó" >&2
exit 1
