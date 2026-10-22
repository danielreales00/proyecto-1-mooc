#!/usr/bin/env bash
# Publicador de la señal de escalado de los workers (ADR-0018, D5). Corre en la
# máquina de la cola cada 30 s, con un temporizador de systemd que instala
# deploy/arranque.sh.
#
# Lee de Redis cuántos trabajos de `bulk` esperan y cuántos están en curso, y
# escribe la suma como un punto de custom.googleapis.com/mooc/cola_bulk_trabajos
# en Cloud Monitoring, sobre el recurso `global`. El autoscaler de workers la
# divide por los trabajos asignados a cada instancia.
#
# Por qué aquí y no en los workers: cada worker exporta la misma profundidad
# (la cola es una sola), y con tres instancias habría tres series idénticas.
# Esta es una sola serie, producida fuera de los grupos que escala.
#
# Por qué pendientes MÁS en curso: una cola sin pendientes puede tener
# transcodificaciones a medias. Si solo contaran los pendientes, el grupo
# encogería mientras una instancia todavía trabaja.
#
# Sin credenciales guardadas: el token sale del servidor de metadatos con la
# identidad de la máquina (mooc-cola, con monitoring.metricWriter).
set -euo pipefail

METADATOS=http://metadata.google.internal/computeMetadata/v1
METRICA=custom.googleapis.com/mooc/cola_bulk_trabajos
COLA=bulk

redis() {
  docker compose -f /opt/mooc/compose.yml exec -T redis redis-cli -n 0 "$@"
}

pendientes=$(redis LLEN "asynq:{${COLA}}:pending")
en_curso=$(redis LLEN "asynq:{${COLA}}:active")
total=$((pendientes + en_curso))

proyecto=$(curl -fsS -H "Metadata-Flavor: Google" "$METADATOS/project/project-id")
token=$(curl -fsS -H "Metadata-Flavor: Google" \
  "$METADATOS/instance/service-accounts/default/token" \
  | sed -n 's/.*"access_token" *: *"\([^"]*\)".*/\1/p')
ahora=$(date -u +%Y-%m-%dT%H:%M:%SZ)

curl -fsS -o /dev/null -X POST \
  -H "Authorization: Bearer ${token}" \
  -H "Content-Type: application/json" \
  "https://monitoring.googleapis.com/v3/projects/${proyecto}/timeSeries" \
  -d @- <<JSON
{
  "timeSeries": [{
    "metric": {"type": "${METRICA}"},
    "resource": {"type": "global", "labels": {"project_id": "${proyecto}"}},
    "points": [{
      "interval": {"endTime": "${ahora}"},
      "value": {"int64Value": "${total}"}
    }]
  }]
}
JSON

echo "mooc: ${COLA} pendientes=${pendientes} en_curso=${en_curso} total=${total}"
