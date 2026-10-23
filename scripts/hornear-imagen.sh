#!/usr/bin/env bash
# Hornea la imagen base de las plantillas (ADR-0019, D7): Debian 12 con Docker,
# el agente de operaciones y todas las imágenes de contenedor ya descargadas.
# Una instancia creada por el autoscaler solo escribe su configuración y
# levanta: no instala paquetes ni depende de Docker Hub en mitad de una prueba.
#
#   make hornear-imagen
#
# Corre en el contenedor de gcloud (GCLOUD_IMAGE), con el repositorio en /repo.
# Usa deploy/arranque.sh en modo `hornear`: el mismo script que arranca las
# instancias, así lo que se hornea es exactamente lo que el arranque instalaría.
#
# Uso: hornear-imagen.sh <proyecto> <zona> <registro> <version>
set -euo pipefail

PROYECTO=$1
ZONA=$2
REGISTRO=$3
VERSION=$4

MAQUINA=mooc-horno-${VERSION}
IMAGEN=mooc-base-${VERSION}
FAMILIA=mooc-base

# Las de la aplicación con el SHA de la corrida y las de terceros con la
# etiqueta que fijan las composiciones de deploy/. Si cambia una etiqueta allí,
# cambia aquí: la imagen horneada no debe quedarse con una versión vieja.
IMAGENES="${REGISTRO}/api:${VERSION} ${REGISTRO}/migrate:${VERSION}
${REGISTRO}/frontend:${VERSION} ${REGISTRO}/worker:${VERSION}
${REGISTRO}/worker-media:${VERSION}
caddy:2.10-alpine swaggerapi/swagger-ui:v5.29.0 clamav/clamav:1.4"

g() { gcloud --project "$PROYECTO" --quiet "$@"; }

if g compute images describe "$IMAGEN" >/dev/null 2>&1; then
  echo "ya existe ${IMAGEN}: nada que hornear"
  exit 0
fi

# Con la cuenta del grupo de workers, que ya puede leer Artifact Registry. En
# la subred de la aplicación, sin IP externa: sale por Cloud NAT.
echo "── creando ${MAQUINA}"
g compute instances create "$MAQUINA" \
  --zone "$ZONA" \
  --machine-type e2-highcpu-2 \
  --image-family debian-12 --image-project debian-cloud \
  --boot-disk-size 30GB --boot-disk-type pd-balanced \
  --subnet mooc-us-central1 --no-address \
  --service-account "mooc-worker@${PROYECTO}.iam.gserviceaccount.com" \
  --scopes cloud-platform \
  --shielded-secure-boot --shielded-vtpm --shielded-integrity-monitoring \
  --metadata mooc-rol=hornear,mooc-imagenes="$(echo $IMAGENES)" \
  --metadata-from-file startup-script=/repo/deploy/arranque.sh

# arranque.sh apaga la máquina al terminar. Veinte minutos de margen.
echo "── esperando a que termine (se apaga sola)"
for _ in $(seq 1 120); do
  estado=$(g compute instances describe "$MAQUINA" --zone "$ZONA" --format 'value(status)')
  [ "$estado" = TERMINATED ] && break
  sleep 10
done
if [ "$estado" != TERMINATED ]; then
  echo "${MAQUINA} no terminó; revisa su puerto serie:" >&2
  echo "  gcloud compute instances get-serial-port-output ${MAQUINA} --zone ${ZONA}" >&2
  exit 1
fi

echo "── creando ${IMAGEN} en la familia ${FAMILIA}"
g compute images create "$IMAGEN" \
  --source-disk "$MAQUINA" --source-disk-zone "$ZONA" \
  --family "$FAMILIA" \
  --labels "version=${VERSION}"

g compute instances delete "$MAQUINA" --zone "$ZONA"
echo "listo: imagen_grupos = \"projects/${PROYECTO}/global/images/${IMAGEN}\""
