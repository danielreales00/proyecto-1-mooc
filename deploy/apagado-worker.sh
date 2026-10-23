#!/usr/bin/env bash
# Script de apagado de las instancias de workers (ADR-0019, D6). GCE lo ejecuta
# como root cuando el autoscaler retira la instancia, antes de apagarla.
#
# `docker compose stop` respeta el stop_grace_period de cada servicio: el
# worker deja de tomar trabajos nuevos y el que está en curso tiene ese margen
# para terminar. Si no termina, asynq lo devuelve a la cola y otra instancia
# lo repite; la idempotencia del worker evita derivados duplicados (ADR-0008).
set -uo pipefail

cd /opt/mooc 2>/dev/null || exit 0
echo "mooc: retirando la instancia; se detienen los workers"
docker compose stop
echo "mooc: workers detenidos"
