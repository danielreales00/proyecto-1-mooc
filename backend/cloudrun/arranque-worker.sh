#!/bin/sh
# Arranque del worker en Cloud Run (ADR-0020, D4): primero clamd, que carga las
# firmas de la imagen, y después el worker. El escaneo de un archivo falla si
# clamd no responde, así que el worker no empieza a recibir mensajes hasta que
# clamd contesta PONG.
set -eu

clamd --config-file=/etc/clamav/clamd.conf &

intentos=0
until [ "$(printf 'PING\n' | nc -w 2 127.0.0.1 3310 2>/dev/null)" = "PONG" ]; do
	intentos=$((intentos + 1))
	if [ "$intentos" -gt 120 ]; then
		echo "clamd no respondió en 120 s" >&2
		exit 1
	fi
	sleep 1
done
echo "clamd listo en ${intentos} s"

exec worker
