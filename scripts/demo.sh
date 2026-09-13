#!/usr/bin/env bash
# Recorrido completo de la demostración de aceptación.
#
# Es el guion del video: cada bloque corresponde a un segmento de la §10.2 del
# enunciado y comprueba la condición verificable asociada. Todo lo que imprime
# es una aserción real contra el sistema en ejecución, no una narración.
set -euo pipefail

API="${API:-http://localhost:8090}"
MAILPIT="${MAILPIT:-http://localhost:8026}"
CLAVE="${CLAVE:-Contrasena-Demo-2026}"

RED=$'\033[31m'; VERDE=$'\033[32m'; DIM=$'\033[2m'; NEG=$'\033[1m'; OFF=$'\033[0m'
fallo() { echo "${RED}✗ $*${OFF}" >&2; exit 1; }
ok()    { echo "  ${VERDE}✓${OFF} $*"; }
seg()   { echo; echo "${NEG}▐ $*${OFF}"; }
nota()  { echo "    ${DIM}$*${OFF}"; }

jq()   { python3 -c "import json,sys;d=json.load(sys.stdin);print(eval(sys.argv[1],{'d':d}))" "$1"; }
login(){ curl -s -X POST "$API/api/v1/auth/login" -H 'Content-Type: application/json' \
           -d "{\"email\":\"$1\",\"password\":\"$CLAVE\"}" | jq "d['token']"; }
J="Content-Type: application/json"
# Las operaciones no repetibles exigen `Idempotency-Key` (ADR-0008). Cada
# llamada genera la suya; repetir una petición con la misma clave devuelve la
# respuesta ya calculada en lugar de ejecutarla otra vez.
# Solo hace falta que sea única y de 8 a 255 caracteres (ADR-0008). Se genera
# con $RANDOM en vez de /proc ni python3: lo primero no existe en macOS y lo
# segundo cuesta un proceso por petición.
idem(){ echo "Idempotency-Key: idem-$(date +%s)-$RANDOM$RANDOM$RANDOM"; }

# ───────────────────────────────────────────────────────── SEG-1 ────────────
seg "SEG-1 · Identidad"

curl -fsS "$API/readyz" >/dev/null || fallo "la API no está lista"
ok "postgres, redis y el almacén de objetos responden"

CUERPO=$(curl -s -X POST "$API/api/v1/auth/register" -H "$J" \
  -d '{"email":"no-es-correo","password":"corta","full_name":""}')
N=$(echo "$CUERPO" | jq "len(d['errors'])")
[ "$N" -ge 3 ] || fallo "se esperaban al menos 3 errores acumulados, llegaron $N"
ok "la validación devuelve TODOS los errores en un solo 422 ($N), no el primero"

NUEVO="estudiante+$(date +%s)@mooc.local"
CODIGO=$(curl -s -o /tmp/reg.json -w '%{http_code}' -X POST "$API/api/v1/auth/register" -H "$J" \
  -d "{\"email\":\"$NUEVO\",\"password\":\"$CLAVE\",\"full_name\":\"Estudiante Nuevo\"}")
[ "$CODIGO" = 202 ] || fallo "el registro devolvió $CODIGO"
ok "registro: 202, la API no espera al worker (CA-02)"

for i in $(seq 1 30); do
  MSG=$(curl -fsS "$MAILPIT/api/v1/search?query=to:$NUEVO" 2>/dev/null \
        | jq "d['messages'][0]['ID'] if d.get('messages') else ''" || echo "")
  [ -n "$MSG" ] && break; [ "$i" = 30 ] && fallo "no llegó el correo"; sleep 1
done
TOKEN=$(curl -fsS "$MAILPIT/api/v1/message/$MSG" | python3 -c \
  "import json,sys,re;print(re.search(r'^([A-Za-z0-9_-]{40,})\r?$',json.load(sys.stdin)['Text'],re.M).group(1))")
ok "el worker consumió el trabajo y entregó el correo de verificación"

curl -fsS -X POST "$API/api/v1/auth/verify-email" -H "$J" -d "{\"token\":\"$TOKEN\"}" >/dev/null
ok "correo verificado"
[ "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/auth/verify-email" -H "$J" \
     -d "{\"token\":\"$TOKEN\"}")" = 422 ] || fallo "el token se pudo reutilizar"
ok "el token de un solo uso no se puede reutilizar"

SES=$(login "$NUEVO")
curl -fsS -X POST "$API/api/v1/auth/logout" -H "Authorization: Bearer $SES" >/dev/null
[ "$(curl -s -o /dev/null -w '%{http_code}' "$API/api/v1/me" -H "Authorization: Bearer $SES")" = 401 ] \
  || fallo "la sesión seguía viva tras cerrarla"
ok "revocación de sesión con efecto inmediato, sin ventana de gracia"

PROF=$(login profesor@mooc.local); EST=$(login estudiante1@mooc.local)
PA="Authorization: Bearer $PROF"; EA="Authorization: Bearer $EST"

[ "$(curl -s -o /dev/null -w '%{http_code}' "$API/api/v1/courses" -H "$EA")" = 403 ] \
  || fallo "un estudiante pudo entrar a la autoría"
ok "control por rol: un estudiante no accede a la autoría (CA-06)"

# ───────────────────────────────────────────────────────── SEG-2 ────────────
seg "SEG-2 · Autoría y publicación"

R=$(curl -s -X POST "$API/api/v1/courses" -H "$(idem)" -H "$PA" -H "$J" \
  -d '{"title":"Fundamentos de Cloud '"$RANDOM"'","summary":"Curso de la demostración.","category":"cloud","language":"es"}')
CID=$(echo "$R" | jq "d['course']['id']"); VID=$(echo "$R" | jq "d['version']['id']")
ok "curso creado con su versión 1 en borrador"

ERRS=$(curl -s -X POST "$API/api/v1/courses/$CID/versions/1/publish" -H "$(idem)" -H "$PA" | jq "len(d['errors'])")
[ "$ERRS" -ge 1 ] || fallo "un curso vacío se pudo publicar"
ok "publicar un curso vacío falla con la lista exhaustiva de errores (CA-01)"

MID=$(curl -s -X POST "$API/api/v1/versions/$VID/modules" -H "$(idem)" -H "$PA" -H "$J" -d '{"title":"Módulo 1"}' | jq "d['id']")
UID_=$(curl -s -X POST "$API/api/v1/modules/$MID/units" -H "$PA" -H "$J" -d '{"title":"Unidad 1"}' | jq "d['id']")
TXT=$(curl -s -X POST "$API/api/v1/units/$UID_/resources" -H "$PA" -H "$J" \
  -d '{"title":"Introducción","type":"rich_text","content_md":"# Contenedores\n\n*  Aislamiento\n\n\n*  Portabilidad   \n"}')
TSID=$(echo "$TXT" | jq "d['stable_id']")
ok "jerarquía curso → módulo → unidad → recurso"

MD=$(curl -s "$API/api/v1/courses/$CID/versions/1" -H "$PA" \
     | jq "d['version']['modules'][0]['units'][0]['resources'][0]['content_md']")
case "$MD" in *"*  Aislamiento"*) fallo "el Markdown no se normalizó";; esac
ok "el Markdown se guarda en forma canónica (viñetas y líneas en blanco)"

QR=$(curl -s -X POST "$API/api/v1/units/$UID_/resources" -H "$PA" -H "$J" \
  -d '{"title":"Evaluación","type":"quiz"}')
QRID=$(echo "$QR" | jq "d['id']"); QSID=$(echo "$QR" | jq "d['stable_id']")
curl -fsS -X PUT "$API/api/v1/resources/$QRID/quiz" -H "$PA" -H "$J" -d '{
  "max_attempts":2,"pass_score":70,"feedback_policy":"correctness",
  "questions":[{"statement_md":"¿Qué aísla un contenedor?","kind":"single","points":1,
    "options":[{"text_md":"Procesos y sistema de archivos","is_correct":true},
               {"text_md":"El hardware físico","is_correct":false}]}]}' >/dev/null
ok "quiz configurado por el profesor"

curl -fsS -X POST "$API/api/v1/courses/$CID/versions/1/publish" -H "$(idem)" -H "$PA" >/dev/null
ok "versión publicada"

CODIGO=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/versions/$VID/modules" -H "$(idem)" \
         -H "$PA" -H "$J" -d '{"title":"No debería entrar"}')
[ "$CODIGO" = 409 ] || fallo "se pudo mutar una versión publicada (llegó $CODIGO)"
ok "la versión publicada es INMUTABLE: 409 al intentar mutarla (ADR-0007)"

# ───────────────────────────────────────────────────────── SEG-5 ────────────
seg "SEG-5 · Catálogo y consumo de contenido"

curl -fsS "$API/api/v1/catalog/courses" | jq "len(d['items'])" >/dev/null
ok "el catálogo público responde sin sesión"

EID=$(curl -s -X POST "$API/api/v1/enrollments" -H "$(idem)" -H "$EA" -H "$J" -d "{\"course_id\":\"$CID\"}" | jq "d['id']")
ok "el estudiante se inscribe"
curl -fsS "$API/api/v1/enrollments/$EID/content" -H "$EA" >/dev/null
ok "recibe el árbol del curso con su progreso"

# ───────────────────────────────────────────────────────── SEG-6 ────────────
seg "SEG-6 · Quiz"

AT=$(curl -s -X POST "$API/api/v1/enrollments/$EID/quizzes/$QSID/attempts" -H "$(idem)" -H "$EA" -H "$J")
AID=$(echo "$AT" | jq "d['id']")
echo "$AT" | grep -qi "is_correct" && fallo "el snapshot contiene las claves correctas"
ok "el snapshot del intento NO contiene ninguna clave correcta (CA-04)"
nota "campos por opción: $(echo "$AT" | jq "list(d['questions'][0]['options'][0].keys())")"

Q=$(echo "$AT" | jq "d['questions'][0]['stable_id']")
O=$(echo "$AT" | jq "[o['stable_id'] for o in d['questions'][0]['options'] if 'Procesos' in o['text_md']][0]")
curl -fsS -X PATCH "$API/api/v1/attempts/$AID/answers" -H "$EA" -H "$J" \
  -d "{\"answers\":[{\"question_stable_id\":\"$Q\",\"selected_option_stable_ids\":[\"$O\"]}]}" >/dev/null
ok "guardado parcial de respuestas"

NOTA=$(curl -s -X POST "$API/api/v1/attempts/$AID/submit" -H "$(idem)" -H "$EA" | jq "d['score']")
ok "calificación en servidor: $NOTA"
NOTA2=$(curl -s -X POST "$API/api/v1/attempts/$AID/submit" -H "$(idem)" -H "$EA" | jq "d['score']")
[ "$NOTA" = "$NOTA2" ] || fallo "reenviar cambió la nota ($NOTA vs $NOTA2)"
ok "reenviar el intento es idempotente: misma nota"

# ───────────────────────────────────────────────────────── SEG-7 ────────────
seg "SEG-7 · Progreso y aprobación"

RESP=$(curl -s -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" -H "$EA" -H "$J" \
  -d "{\"resource_stable_id\":\"$TSID\",\"kind\":\"heartbeat\",\"progress_percent\":100}")
echo "$RESP" | grep -q "progress.client_computed_value" || fallo "se aceptó un porcentaje del cliente"
ok "SEÑAL FRAUDULENTA rechazada: el cliente no calcula el progreso (CA-05)"

AUD=$(docker compose exec -T postgres psql -U mooc -d mooc -tAc \
  "SELECT count(*) FROM audit.events WHERE action='progress.tampering_attempt';")
[ "$AUD" -ge 1 ] || fallo "el intento de manipulación no quedó auditado"
ok "el intento quedó auditado ($AUD en total)"

curl -fsS -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" -H "$EA" -H "$J" \
  -d "{\"resource_stable_id\":\"$TSID\",\"kind\":\"open\"}" >/dev/null
MOT=$(curl -s -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" -H "$EA" -H "$J" \
  -d "{\"resource_stable_id\":\"$TSID\",\"kind\":\"heartbeat\"}" | jq "d['reject_reason']")
[ "$MOT" = "cadence" ] || fallo "no se rechazó el heartbeat por cadencia (motivo: $MOT)"
ok "heartbeat inmediato rechazado por cadencia mínima"

nota "acumulando permanencia con heartbeats espaciados…"
for _ in 1 2; do
  for i in $(seq 1 30); do
    RESP=$(curl -fsS -X POST "$API/api/v1/enrollments/$EID/progress" -H "$(idem)" -H "$EA" -H "$J" \
      -d "{\"resource_stable_id\":\"$TSID\",\"kind\":\"heartbeat\"}") \
      || fallo "falló la petición de heartbeat"
    ACEPTADO=$(echo "$RESP" | jq "d['accepted']") || fallo "respuesta de heartbeat inesperada: $RESP"
    [ "$ACEPTADO" = True ] && break
    [ "$i" = 30 ] && fallo "no se aceptó el heartbeat tras 30 intentos: $RESP"
    sleep 3
  done
done
ESTADO=$(curl -s "$API/api/v1/enrollments/$EID" -H "$EA" | jq "d['state']")
PCT=$(curl -s "$API/api/v1/enrollments/$EID" -H "$EA" | jq "round(d['progress_pct'])")
[ "$ESTADO" = "approved" ] || fallo "la inscripción quedó en $ESTADO con $PCT%"
ok "estado calculado por el servidor: $ESTADO ($PCT% de los obligatorios)"

# ───────────────────────────────────────────────────────── SEG-8 ────────────
seg "SEG-8 · Insignia"

# Se busca la insignia DE ESTA inscripción: el estudiante puede tener otras de
# ejecuciones anteriores, y quedarse con items[0] daría un falso verde.
for i in $(seq 1 20); do
  CODE=$(curl -s "$API/api/v1/me/badges" -H "$EA" \
    | jq "([b['public_code'] for b in d['items'] if b['enrollment_id']=='$EID'] or [''])[0]")
  [ -n "$CODE" ] && break
  [ "$i" = 20 ] && fallo "el worker no emitió la insignia de esta inscripción"
  sleep 2
done
ok "el worker emitió la insignia: $CODE"

PUB=$(curl -s "$API/api/v1/verify/$CODE")
echo "$PUB" | grep -qiE '@|"email"' && fallo "la verificación pública expone el correo"
ok "la verificación pública NO expone el correo del estudiante (CA-07)"
nota "$(echo "$PUB" | jq "d['student_name']+' · '+d['course_title']+' · '+d['status']")"

# ───────────────────────────────────────────────────────── SEG-4 ────────────
seg "SEG-4 · Tolerancia a fallos e idempotencia"

# Se devuelve el trabajo a 'queued', como haría un reencolado administrativo.
# El reaper lo republica en la cola y el worker vuelve a ejecutarlo de verdad:
# sin eso, la prueba pasaría porque nadie lo ejecutó, no por idempotencia.
ANTES=$(docker compose exec -T postgres psql -U mooc -d mooc -tAc \
  "SELECT count(*) FROM badges.badges WHERE enrollment_id='$EID';")
[ "$ANTES" = 1 ] || fallo "se esperaba exactamente una insignia antes del reencolado (hay $ANTES)"

docker compose exec -T postgres psql -U mooc -d mooc -q -c \
  "UPDATE platform.job_runs SET status='queued', created_at = now() - interval '1 hour'
    WHERE job_key='badge.issue:$EID';" >/dev/null
nota "trabajo devuelto a la cola; esperando a que el reaper lo recupere…"

for i in $(seq 1 20); do
  ESTADO=$(docker compose exec -T postgres psql -U mooc -d mooc -tAc \
    "SELECT status FROM platform.job_runs WHERE job_key='badge.issue:$EID';")
  [ "$ESTADO" != "queued" ] && break
  [ "$i" = 20 ] && fallo "el reaper no recuperó el trabajo (sigue en queued)"
  sleep 3
done
ok "el reaper republicó el trabajo huérfano y el worker lo reejecutó ($ESTADO)"

DESPUES=$(docker compose exec -T postgres psql -U mooc -d mooc -tAc \
  "SELECT count(*) FROM badges.badges WHERE enrollment_id='$EID';")
[ "$DESPUES" = 1 ] || fallo "la reejecución duplicó la insignia ($ANTES → $DESPUES)"
ok "la reejecución NO produjo una segunda salida: sigue habiendo 1 insignia (CA-03)"

docker compose exec -T postgres psql -U mooc -d mooc -tAc \
  "SELECT job_key||' · '||status||' · intento '||attempt FROM platform.job_runs ORDER BY id DESC LIMIT 3;" \
  | sed "s/^/    ${DIM}/;s/$/${OFF}/"
ok "los trabajos viven en PostgreSQL, no solo en Redis (ADR-0004)"

# ───────────────────────────────────────────────────────── SEG-9 ────────────
seg "SEG-9 · Operación"

if docker compose exec -T postgres psql -U mooc -d mooc -q -c \
     "UPDATE audit.events SET action='manipulado' WHERE id=(SELECT max(id) FROM audit.events);" >/dev/null 2>&1; then
  fallo "la auditoría admitió un UPDATE"
fi
ok "la auditoría es inmutable: PostgreSQL rechaza el UPDATE"

INSTANCIAS=$(docker compose ps --format '{{.Name}}' | grep -c '^mooc-api-' || true)
ok "instancias de API en ejecución: $INSTANCIAS"

echo
echo "${VERDE}${NEG}Flujo completo verificado de punta a punta.${OFF}"
echo "${DIM}curso=$CID  inscripción=$EID  insignia=$CODE${OFF}"
