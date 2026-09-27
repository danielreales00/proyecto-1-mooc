#!/usr/bin/env bash
# Arranque del generador de carga (capacity-planning/). Solo instala lo que
# necesita: Docker, para correr k6 y FFmpeg en contenedor, y el agente de
# operaciones, porque el informe tiene que demostrar que el generador no
# limitó los resultados (CPU por debajo del 60 %).
set -euo pipefail

if ! command -v docker >/dev/null; then
  apt-get update -q
  apt-get install -y -q ca-certificates curl
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
  . /etc/os-release
  echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian ${VERSION_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -q
  apt-get install -y -q docker-ce docker-ce-cli containerd.io
fi

if ! systemctl is-active --quiet google-cloud-ops-agent; then
  curl -fsSL https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh -o /tmp/ops.sh
  bash /tmp/ops.sh --also-install
fi

gcloud auth configure-docker us-central1-docker.pkg.dev --quiet
mkdir -p /opt/carga/resultados /opt/carga/medios /opt/carga/k6
chmod 777 /opt/carga/resultados /opt/carga/medios
echo "mooc: generador listo"
