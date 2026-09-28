#!/usr/bin/env bash
# Script de arranque de las dos máquinas (ADR-0016, D1 y D7). Lo ejecuta el
# agente de GCE como root en cada arranque; Terraform lo entrega por los
# metadatos de la instancia junto con la composición y la configuración.
#
# Es idempotente: instala lo que falte, reescribe la configuración y deja la
# composición levantada. Para desplegar una versión nueva basta cambiar la
# versión en Terraform y volver a ejecutarlo:
#
#   sudo google_metadata_script_runner startup
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
echo "mooc: arranque del rol ${ROL}"

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
# informe de capacidad las exige. Se instala desde el primer arranque: hacerlo
# después obliga a repetir las corridas que ya se hicieron sin él.
if ! systemctl is-active --quiet google-cloud-ops-agent; then
  curl -fsSL https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh \
    -o /tmp/add-ops-agent-repo.sh
  bash /tmp/add-ops-agent-repo.sh --also-install
fi

# Métricas de la aplicación y de Redis al mismo sitio que CPU, memoria y disco.
# En la nube no hay Prometheus: el agente rasca /metrics de cada proceso (solo
# publicado en 127.0.0.1) y lo envía a Cloud Monitoring, donde se consulta con
# PromQL. Solo las series propias: se paga por muestra, y las de runtime de Go
# no contestan ninguna pregunta del informe.
objetivos() {
  case "$ROL" in
    web)    echo "['127.0.0.1:8080']" ;;
    worker) echo "['127.0.0.1:8081', '127.0.0.1:8082']" ;;
  esac
}
cat > /etc/google-cloud-ops-agent/config.yaml.nuevo <<CONFIG
metrics:
  receivers:
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
$( [ "$ROL" = worker ] && cat <<REDIS
    redis:
      type: redis
      address: $(md mooc-ip-privada):6379
      collection_interval: 30s
REDIS
)
  service:
    pipelines:
      mooc:
        receivers: [mooc$( [ "$ROL" = worker ] && echo ", redis")]
CONFIG
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
esac

# El .env lleva los secretos: se escribe con permisos 0600, fuera de cualquier
# imagen, leyendo Secret Manager con la identidad de la instancia. Se escribe
# aparte y se mueve, para que nunca exista a medias con otros permisos.
secreto() {
  gcloud secrets versions access latest --secret="$1" --quiet 2>/dev/null || true
}
umask 077
{
  md mooc-env
  echo "DATABASE_URL=$(secreto database-url)"
  if [ "$ROL" = worker ]; then
    echo "SMTP_PASSWORD=$(secreto smtp-password)"
  fi
} > "$DIR/.env.nuevo"
chmod 600 "$DIR/.env.nuevo"
mv "$DIR/.env.nuevo" "$DIR/.env"
umask 022

if ! grep -q '^DATABASE_URL=.' "$DIR/.env"; then
  echo "mooc: no se pudo leer database-url de Secret Manager" >&2
  exit 1
fi

# --- Imágenes y arranque ----------------------------------------------------
# El ayudante de credenciales de gcloud pide un token a la identidad de la
# instancia en cada descarga: no hay clave ni contraseña guardada.
gcloud auth configure-docker "$REGISTRO_HOST" --quiet

cd "$DIR"
docker compose pull --quiet

# La API depende del Redis de la OTRA máquina, y Compose solo sabe esperar a
# servicios de la suya: si el Web Server arranca primero, la API no pasa su
# healthcheck y `up` aborta dejando el proxy sin arrancar. Se reintenta hasta
# que el Worker Server esté listo; diez minutos cubren su arranque en frío.
for intento in $(seq 1 60); do
  if docker compose up -d --remove-orphans --wait --wait-timeout 120; then
    # Caddy corre con la API de administración apagada y no recarga solo: si
    # su configuración cambió, se reinicia el proxy.
    if [ "$ROL" = web ] && [ "$caddy_antes" != "$(sha256sum "$DIR/Caddyfile")" ]; then
      docker compose restart proxy
    fi
    echo "mooc: ${ROL} levantado"
    exit 0
  fi
  echo "mooc: intento ${intento} sin éxito; se reintenta en 10 s"
  sleep 10
done
echo "mooc: ${ROL} no levantó" >&2
exit 1
