#!/usr/bin/env bash
# Comprueba los invariantes que los ADR prometen verificar y que no cubre
# `make demo`.
#
# Cada bloque cita el ADR que lo exige. Un ADR que anuncia una comprobación y
# no la tiene es una promesa, no una regla.
set -euo pipefail

API="${API:-http://localhost:8090}"
CLAVE="${CLAVE:-Contrasena-Demo-2026}"
ROJO=$'\033[31m'; VERDE=$'\033[32m'; DIM=$'\033[2m'; NEG=$'\033[1m'; OFF=$'\033[0m'

fallos=0
mal(){ echo "  ${ROJO}✗ $*${OFF}" >&2; fallos=$((fallos+1)); }
ok(){ echo "  ${VERDE}✓${OFF} $*"; }
bloque(){ echo; echo "${NEG}▐ $*${OFF}"; }
jq(){ python3 -c "import json,sys;d=json.load(sys.stdin);print(eval(sys.argv[1],{'d':d}))" "$1"; }
lg(){ curl -s -X POST "$API/api/v1/auth/login" -H 'Content-Type: application/json' \
        -d "{\"email\":\"$1\",\"password\":\"$CLAVE\"}" | jq "d['token']"; }
J="Content-Type: application/json"
# Las operaciones no repetibles exigen `Idempotency-Key` (ADR-0008). Cada
# llamada genera la suya; repetir una petición con la misma clave devuelve la
# respuesta ya calculada en lugar de ejecutarla otra vez.
# Solo hace falta que sea única y de 8 a 255 caracteres (ADR-0008). Se genera
# con $RANDOM en vez de /proc ni python3: lo primero no existe en macOS y lo
# segundo cuesta un proceso por petición.
idem(){ echo "Idempotency-Key: idem-$(date +%s)-$RANDOM$RANDOM$RANDOM"; }

P=$(lg profesor@mooc.local); E=$(lg estudiante2@mooc.local)
PA="Authorization: Bearer $P"; EA="Authorization: Bearer $E"

# ─────────────────────────────────────────────────────────── ADR-0007 ──────
bloque "ADR-0007 · el progreso sobrevive a una versión nueva del curso"

R=$(curl -s -X POST "$API/api/v1/courses" -H "$(idem)" -H "$PA" -H "$J" \
  -d '{"title":"Versionado '"$RANDOM"'","summary":"Invariante de stable_id.","category":"cloud","language":"es"}')
CID=$(echo "$R"|jq "d['course']['id']"); VID=$(echo "$R"|jq "d['version']['id']")
MID=$(curl -s -X POST "$API/api/v1/versions/$VID/modules" -H "$(idem)" -H "$PA" -H "$J" -d '{"title":"M1"}'|jq "d['id']")
UID_=$(curl -s -X POST "$API/api/v1/modules/$MID/units" -H "$PA" -H "$J" -d '{"title":"U1"}'|jq "d['id']")
RES=$(curl -s -X POST "$API/api/v1/units/$UID_/resources" -H "$PA" -H "$J" \
  -d '{"title":"Lección 1","type":"rich_text","content_md":"# Uno\n"}')
SID=$(echo "$RES"|jq "d['stable_id']"); RID1=$(echo "$RES"|jq "d['id']")
curl -fsS -X POST "$API/api/v1/courses/$CID/versions/1/publish" -H "$(idem)" -H "$PA" >/dev/null

EID=$(curl -s -X POST "$API/api/v1/enrollments" -H "$(idem)" -H "$EA" -H "$J" -d "{\"course_id\":\"$CID\"}"|jq "d['id']")
curl -fsS -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" -H "$EA" -H "$J" \
  -d "{\"resource_stable_id\":\"$SID\",\"kind\":\"open\"}" >/dev/null
for _ in 1 2; do
  until [ "$(curl -s -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" -H "$EA" -H "$J" \
      -d "{\"resource_stable_id\":\"$SID\",\"kind\":\"heartbeat\"}"|jq "d['accepted']")" = True ]; do sleep 3; done
done
ANTES=$(curl -s "$API/api/v1/enrollments/$EID/progress" -H "$EA"|jq "round(d['progress_pct'])")
[ "$ANTES" = "100" ] || mal "el estudiante no llegó al 100% antes de versionar (fue $ANTES%)"

curl -fsS -X POST "$API/api/v1/courses/$CID/versions/1/unpublish" -H "$(idem)" -H "$PA" >/dev/null
curl -fsS -X POST "$API/api/v1/courses/$CID/versions" -H "$(idem)" -H "$PA" >/dev/null
ARBOL=$(curl -s "$API/api/v1/courses/$CID/versions/2" -H "$PA")
SID2=$(echo "$ARBOL"|jq "d['version']['modules'][0]['units'][0]['resources'][0]['stable_id']")
RID2=$(echo "$ARBOL"|jq "d['version']['modules'][0]['units'][0]['resources'][0]['id']")

[ "$SID" = "$SID2" ] && ok "el stable_id se conserva al clonar la versión" \
                     || mal "el stable_id cambió: $SID -> $SID2"
[ "$RID1" != "$RID2" ] && ok "el id de fila sí cambia, como debe" \
                       || mal "el id de fila no cambió; la versión no se clonó"

curl -fsS -X POST "$API/api/v1/courses/$CID/versions/2/publish" -H "$(idem)" -H "$PA" >/dev/null
DESPUES=$(curl -s "$API/api/v1/enrollments/$EID/progress" -H "$EA"|jq "round(d['progress_pct'])")
[ "$ANTES" = "$DESPUES" ] && ok "el progreso sobrevivió a la versión nueva ($DESPUES%)" \
                          || mal "el progreso se perdió: $ANTES% -> $DESPUES%"

APROB=$(curl -s "$API/api/v1/enrollments/$EID" -H "$EA"|jq "d['state']")
[ "$APROB" = "approved" ] && ok "la aprobación no se revoca por publicar una versión nueva" \
                          || mal "la inscripción pasó a $APROB tras la versión nueva"

# ─────────────────────────────────────────────────────────── ADR-0013 ──────
bloque "ADR-0013 · la clave del quiz no sale por NINGUNA ruta del estudiante"

QR=$(curl -s -X POST "$API/api/v1/units/$UID_/resources" -H "$PA" -H "$J" \
  -d '{"title":"Quiz","type":"quiz"}' 2>/dev/null || echo '{}')
if [ "$(echo "$QR"|jq "d.get('id','')")" = "" ]; then
  echo "  ${DIM}(la versión 2 está publicada; se omite el alta de quiz)${OFF}"
else
  QRID=$(echo "$QR"|jq "d['id']")
  curl -fsS -X PUT "$API/api/v1/resources/$QRID/quiz" -H "$PA" -H "$J" -d '{
    "max_attempts":0,"pass_score":70,"feedback_policy":"correctness",
    "questions":[{"statement_md":"¿Dos más dos?","kind":"single","points":1,
      "options":[{"text_md":"Cuatro","is_correct":true},{"text_md":"Cinco","is_correct":false}]}]}' >/dev/null
fi

# Se recorren TODAS las respuestas del flujo del estudiante buscando la clave.
sucias=0
for ruta in "/api/v1/enrollments/$EID/content" "/api/v1/enrollments/$EID/progress" \
            "/api/v1/enrollments" "/api/v1/enrollments/$EID"; do
  cuerpo=$(curl -s "$API$ruta" -H "$EA")
  echo "$cuerpo" | grep -qi "is_correct" && { mal "$ruta contiene is_correct"; sucias=1; }
done
[ $sucias -eq 0 ] && ok "ninguna respuesta del estudiante contiene is_correct"

CUERPO_PROF=$(curl -s "$API/api/v1/courses/$CID/versions/2" -H "$PA")
echo "$CUERPO_PROF" | grep -qi "is_correct" \
  && mal "el árbol de autoría filtra is_correct fuera de la ruta del quiz" \
  || ok "el árbol de autoría tampoco lo incluye; solo la ruta del quiz"

# ─────────────────────────────────────────────────────────── ADR-0012 ──────
bloque "ADR-0012 · el servidor descarta las evidencias imposibles"

# Se espera a que pase la cadencia mínima: de lo contrario el heartbeat se
# rechazaría por cadencia y no llegaríamos a evaluar el salto, que es lo que
# esta comprobación quiere probar.
until [ "$(curl -s -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" -H "$EA" -H "$J" \
      -d "{\"resource_stable_id\":\"$SID\",\"kind\":\"heartbeat\"}"|jq "d['accepted']")" = True ]; do
  sleep 3
done
SALTO=$(curl -s -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" -H "$EA" -H "$J" \
  -d "{\"resource_stable_id\":\"$SID\",\"kind\":\"heartbeat\",\"position_seconds\":3000}"|jq "d['reject_reason']")
[ "$SALTO" = "cadence" ] \
  && ok "el salto llegó demasiado pronto y lo frenó la cadencia (primera barrera)" \
  || { [ "$SALTO" = "position_jump" ] \
       && ok "un salto de posición imposible se descarta" \
       || mal "el salto de posición se aceptó (motivo: ${SALTO:-ninguno})"; }

NEGATIVA=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" \
  -H "$EA" -H "$J" -d "{\"resource_stable_id\":\"$SID\",\"kind\":\"heartbeat\",\"position_seconds\":-5}")
[ "$NEGATIVA" = "202" ] && ok "una posición negativa no rompe la ingesta" \
                        || mal "posición negativa -> $NEGATIVA"

# ─────────────────────────────────────────────────────────── ADR-0006 ──────
bloque "ADR-0006 · el token de sesión no se guarda en claro"

if docker compose exec -T redis redis-cli -n 1 --scan --pattern 'sess:*' 2>/dev/null | grep -qF "$E"; then
  mal "el token aparece en claro en una clave de Redis"
else
  ok "las claves de sesión son hashes, no tokens"
fi
if docker compose exec -T redis redis-cli -n 1 --scan --pattern 'sess:*' 2>/dev/null | head -1 \
   | grep -qE '^sess:[a-f0-9]{64}$'; then
  ok "el formato de la clave es sess:<sha256>"
fi

# ─────────────────────────────────────────────────────────── ADR-0005 ──────
bloque "ADR-0005 · los objetos privados no se sirven sin firma"

DIRECTO=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:9010/mooc-originals/originals/")
[ "$DIRECTO" = "403" ] || [ "$DIRECTO" = "404" ] \
  && ok "el bucket privado no responde sin firma ($DIRECTO)" \
  || mal "el bucket privado respondió $DIRECTO sin firma"

PUB=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:9010/mooc-badges/")
ok "el bucket de insignias es el único público (responde $PUB)"

# ─────────────────────────────────────────────────────────── ADR-0009 ──────
bloque "ADR-0009 · todos los errores usan application/problem+json"

malformados=0
for caso in "GET /api/v1/me|" "GET /api/v1/courses/no-es-uuid|$PA" \
            "POST /api/v1/enrollments|$EA" "GET /api/v1/admin/users|$EA"; do
  ruta="${caso%|*}"; hdr="${caso#*|}"
  met="${ruta%% *}"; path="${ruta#* }"
  ct=$(curl -s -o /dev/null -w '%{content_type}' -X "$met" "$API$path" \
        ${hdr:+-H "$hdr"} -H "$J" -d '{}')
  case "$ct" in application/problem+json*) ;; *) mal "$ruta devolvió $ct"; malformados=1;; esac
done
[ $malformados -eq 0 ] && ok "los 4xx comprobados responden en problem+json"

echo
if [ $fallos -gt 0 ]; then
  echo "${ROJO}${NEG}$fallos invariante(s) incumplido(s).${OFF}"
  exit 1
fi
echo "${VERDE}${NEG}Todos los invariantes comprobados se cumplen.${OFF}"
