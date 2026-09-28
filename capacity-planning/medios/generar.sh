#!/bin/sh
# Genera los tres perfiles del escenario 2 (pruebas_de_carga_entrega2.md, §6.1).
# Corre dentro de la imagen worker-media, que trae FFmpeg: make medios-carga.
#
# Video de patrón de prueba en movimiento (testsrc2) y un tono de audio: no es
# ruido incompresible, que falsearía el tiempo de FFmpeg, ni una grabación
# real. Se declara así en el informe. La tasa de bits apunta al tamaño del plan.
set -eu
cd "$(dirname "$0")"
perfil() { # nombre segundos WxH kbps
  [ -f "$1.mp4" ] && return 0
  ffmpeg -hide_banner -loglevel error -y \
    -f lavfi -i "testsrc2=size=$3:rate=30" -f lavfi -i "sine=frequency=440:sample_rate=48000" \
    -t "$2" -c:v libx264 -preset veryfast -b:v "$4k" -maxrate "$4k" -bufsize "$(( $4 * 2 ))k" \
    -pix_fmt yuv420p -c:a aac -b:a 96k -movflags +faststart "$1.mp4"
}
# Duraciones cortas a propósito (decidido el 27-09): tres perfiles distintos
# en duración, tamaño y resolución, con la escalera completa de rendiciones en
# C, sin que un solo archivo tarde decenas de minutos en transcodificarse.
perfil a 20 640x360 450
perfil b 60 1280x720 1050
perfil c 120 1920x1080 2400
for f in a b c; do
  printf '{"perfil":"%s","bytes":%s,"sha256":"%s"}\n' "$f" "$(wc -c < "$f.mp4")" "$(sha256sum "$f.mp4" | cut -d' ' -f1)"
done > manifiesto.jsonl
cat manifiesto.jsonl
