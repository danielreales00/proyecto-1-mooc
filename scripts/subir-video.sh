#!/usr/bin/env bash
# Sube un archivo a la plataforma, de principio a fin.
#
# La carga es multipart directa al almacén: la API solo firma las URLs y los
# bytes nunca pasan por ella. Eso es lo correcto, pero hace incómodo probarlo a
# mano —hay que partir el archivo, subir cada parte, recoger su ETag y
# completar—, así que este guion lo hace por ti.
#
#   ./scripts/subir-video.sh mi-video.mp4
#   ./scripts/subir-video.sh documento.pdf pdf
#
# El segundo argumento es el tipo: video, audio, image, pdf, slides o file.
# Si no se indica, se deduce de la extensión.
set -euo pipefail

ARCHIVO="${1:-}"
[ -n "$ARCHIVO" ] && [ -f "$ARCHIVO" ] || {
  echo "uso: $0 <archivo> [video|audio|image|pdf|slides|file]" >&2; exit 1; }

API="${API:-http://localhost:8090}"
CORREO="${CORREO:-profesor@mooc.local}"
CLAVE="${CLAVE:-Contrasena-Demo-2026}"

VERDE=$'\033[32m'; ROJO=$'\033[31m'; DIM=$'\033[2m'; NEG=$'\033[1m'; OFF=$'\033[0m'
jq(){ python3 -c "import json,sys;d=json.load(sys.stdin);print(eval(sys.argv[1],{'d':d}))" "$1"; }
idem(){ echo "Idempotency-Key: $(python3 -c 'import uuid; print(uuid.uuid4())')"; }
paso(){ echo; echo "${NEG}$*${OFF}"; }
ok(){ echo "  ${VERDE}✓${OFF} $*"; }

# Tipo por extensión, si no se indicó.
TIPO="${2:-}"
if [ -z "$TIPO" ]; then
  case "${ARCHIVO,,}" in
    *.mp4|*.mov|*.mkv|*.webm) TIPO=video ;;
    *.mp3|*.m4a|*.wav|*.ogg)  TIPO=audio ;;
    *.jpg|*.jpeg|*.png|*.gif|*.webp) TIPO=image ;;
    *.pdf)   TIPO=pdf ;;
    *.pptx|*.odp) TIPO=slides ;;
    *) TIPO=file ;;
  esac
fi

TAM=$(stat -c%s "$ARCHIVO")
SHA=$(sha256sum "$ARCHIVO" | cut -d' ' -f1)
NOMBRE=$(basename "$ARCHIVO")

paso "1 · Iniciando sesión como $CORREO"
TOKEN=$(curl -fsS -X POST "$API/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$CORREO\",\"password\":\"$CLAVE\"}" | jq "d['token']")
A="Authorization: Bearer $TOKEN"; J="Content-Type: application/json"
ok "sesión abierta"

paso "2 · Declarando la carga"
echo "  ${DIM}$NOMBRE · $((TAM/1024/1024)) MiB · tipo $TIPO${OFF}"
echo "  ${DIM}sha256 declarado: ${SHA:0:32}…${OFF}"
R=$(curl -fsS -X POST "$API/api/v1/assets/init" -H "$A" -H "$J" -H "$(idem)" \
  -d "{\"filename\":\"$NOMBRE\",\"content_type\":\"application/octet-stream\",\"size_bytes\":$TAM,\"sha256\":\"$SHA\",\"kind\":\"$TIPO\"}")
ASSET=$(echo "$R" | jq "d['asset_id']")
PARTES=$(echo "$R" | jq "d['total_parts']")
TAM_PARTE=$(echo "$R" | jq "d['part_size']")
ok "asset $ASSET · $PARTES parte(s) de $((TAM_PARTE/1024/1024)) MiB"
ok "URLs prefirmadas recibidas; los bytes NO pasan por la API"

paso "3 · Subiendo las partes directamente al almacén"
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
split -b "$TAM_PARTE" -d -a 3 "$ARCHIVO" "$TMP/p-"
ETAGS="[]"
i=0
for f in "$TMP"/p-*; do
  n=$((i+1))
  URL=$(echo "$R" | python3 -c "import json,sys;print([p['url'] for p in json.load(sys.stdin)['parts'] if p['part_number']==$n][0])")
  ET=$(curl -fsS -X PUT --data-binary "@$f" -D- "$URL" -o /dev/null \
       | grep -i '^etag' | tr -d '\r"' | awk '{print $2}')
  [ -n "$ET" ] || { echo "${ROJO}✗ la parte $n no se subió${OFF}" >&2; exit 1; }
  ETAGS=$(python3 -c "
import json;e=json.loads('$ETAGS');e.append({'part_number':$n,'etag':'$ET'});print(json.dumps(e))")
  printf '  parte %d/%d  %s\n' "$n" "$PARTES" "${DIM}etag ${ET:0:12}…${OFF}"
  i=$n
done
ok "$PARTES parte(s) en el almacén"

paso "4 · Estado de la carga (esto es lo que permite REANUDAR)"
curl -fsS "$API/api/v1/assets/$ASSET/upload" -H "$A" | python3 -c "
import json,sys;d=json.load(sys.stdin)
print('  recibidas:',[p['part_number'] for p in d['received_parts']])
print('  faltan:   ',d['missing_parts'])
print('  la ventana expira:',d['expires_at'][:19].replace('T',' '))"

paso "5 · Completando"
curl -fsS -X POST "$API/api/v1/assets/$ASSET/complete" -H "$A" -H "$J" -H "$(idem)" \
  -d "{\"parts\":$ETAGS}" | jq "'  estado: '+d['status']+'   (202: la API no espera al worker)'"

paso "6 · El worker verifica checksum y tipo real"
for _ in $(seq 1 30); do
  S=$(curl -fsS "$API/api/v1/assets/$ASSET" -H "$A")
  EST=$(echo "$S" | jq "d['status']")
  [ "$EST" != "uploaded" ] && break
  sleep 2
done
echo "$S" | python3 -c "
import json,sys;d=json.load(sys.stdin)
print('  estado:              ',d['status'])
print('  tipo declarado:      ',d['declared_mime'])
print('  tipo REAL detectado: ',d['detected_mime'],' (por los bytes, no por la extensión)')
print('  sha256 recalculado:  ',(d['sha256'] or '')[:32]+'…')
if d['last_error']: print('  motivo del rechazo:  ',d['last_error'])"

case "$EST" in
  clean|ready)
    ok "el archivo pasó la verificación"
    echo
    echo "${DIM}Para adjuntarlo a un curso, al crear el recurso:${OFF}"
    echo "${DIM}  {\"title\":\"Mi vídeo\",\"type\":\"$TIPO\",\"asset_id\":\"$ASSET\"}${OFF}"
    echo
    echo "${DIM}Descargarlo (302 a una URL firmada de 15 min):${OFF}"
    echo "${DIM}  curl -L -H \"Authorization: Bearer \\\$TOKEN\" $API/api/v1/assets/$ASSET/content${OFF}"
    echo
    echo "${DIM}Reproducirlo NO se puede: la transcodificación a HLS aún no está.${OFF}"
    echo "${DIM}Ver arquitectura/media-plan.md${OFF}"
    ;;
  rejected)
    echo "  ${ROJO}el archivo fue rechazado y está en cuarentena${OFF}" ;;
  *)
    echo "  ${ROJO}quedó en estado $EST${OFF}" ;;
esac
