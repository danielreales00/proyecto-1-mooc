#!/usr/bin/env bash
# Reproduce dentro del contenedor los puertos que la máquina publica, y luego
# ejecuta lo que le pidan.
#
# Los scripts y la colección de Postman apuntan a `http://localhost:8090`, que
# es lo que se ve en el video y lo que escribe cualquiera que siga el README.
# Dentro de un contenedor, `localhost` es el propio contenedor.
#
# Reescribir /etc/hosts no funciona: tanto glibc como musl resuelven el nombre
# `localhost` internamente a 127.0.0.1, sin mirar el archivo. Y `--network host`
# tampoco: en macOS y en Windows el demonio corre en una máquina virtual y ese
# modo no da acceso a los puertos publicados.
#
# Lo que sí funciona en los tres sistemas es no salir de la red de Docker:
# socat escucha en 127.0.0.1 en el mismo puerto que publica la máquina y lo
# reenvía al servicio correspondiente. Así `localhost:8090` significa lo mismo
# dentro que fuera, sin tocar una línea de los scripts. Importa además para las
# URL prefirmadas de MinIO: su firma incluye el Host, y reenviar en crudo lo
# deja intacto.
set -euo pipefail

reenviar() {
  socat "TCP-LISTEN:$1,bind=127.0.0.1,fork,reuseaddr" "TCP:$2" &
}

# El puerto de la izquierda sale de .env; el de la derecha es el que el servicio
# escucha dentro de la red de Docker.
reenviar "${API_PORT:-8090}"            proxy:8080
reenviar "${MAILPIT_UI_PORT:-8026}"     mailpit:8025
reenviar "${MINIO_API_PORT:-9010}"      minio:9000
reenviar "${MINIO_CONSOLE_PORT:-9011}"  minio:9001
reenviar "${PROMETHEUS_PORT:-9091}"     prometheus:9090
reenviar "${GRAFANA_PORT:-3002}"        grafana:3000
reenviar "${POSTGRES_PORT:-5433}"       postgres:5432
reenviar "${REDIS_PORT:-6380}"          redis:6379

# socat tarda milisegundos en escuchar, pero el primer `curl` llega antes si no
# se espera. Se comprueba el socket en /proc, sin abrir conexiones: el servicio
# de destino puede estar legítimamente apagado (Prometheus vive en un perfil).
escuchando() { grep -qi "0100007F:$(printf '%04X' "$1")" /proc/net/tcp; }

for puerto in "${API_PORT:-8090}" "${MAILPIT_UI_PORT:-8026}" "${MINIO_API_PORT:-9010}"; do
  intento=0
  until escuchando "$puerto"; do
    intento=$((intento + 1))
    if [ "$intento" -gt 100 ]; then
      echo "el reenvío del puerto $puerto no llegó a escuchar" >&2
      exit 1
    fi
    sleep 0.05
  done
done

exec "$@"
