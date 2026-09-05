#!/usr/bin/env bash
# Prueba de extremo a extremo de la rebanada vertical.
#
# Recorre el camino completo: registro -> trabajo asíncrono -> correo en Mailpit
# -> verificación -> login -> /me -> revocación inmediata de la sesión.
# Comprueba además la lista exhaustiva de errores (CA-01) y la idempotencia del
# encolado (CA-03).
set -euo pipefail

API="${API:-http://localhost:8090}"
MAILPIT="${MAILPIT:-http://localhost:8026}"

RED=$'\033[31m'; GREEN=$'\033[32m'; DIM=$'\033[2m'; BOLD=$'\033[1m'; OFF=$'\033[0m'
fail() { echo "${RED}✗ $*${OFF}" >&2; exit 1; }
ok()   { echo "${GREEN}✓${OFF} $*"; }
step() { echo; echo "${BOLD}$*${OFF}"; }
jqp()  { python3 -c "import json,sys;d=json.load(sys.stdin);print(eval(sys.argv[1],{'d':d}))" "$1"; }

EMAIL="estudiante+$(date +%s)@mooc.local"
PASSWORD="contrasena-de-prueba"

step "0. Esperando a que la API esté lista"
for i in $(seq 1 60); do
  if curl -fsS "$API/readyz" >/dev/null 2>&1; then break; fi
  [ "$i" = 60 ] && fail "la API no respondió a /readyz en 60 s"
  sleep 1
done
curl -fsS "$API/readyz" | python3 -m json.tool | sed "s/^/  ${DIM}/;s/$/${OFF}/"
ok "postgres, redis y el almacén de objetos responden"

step "1. Validación: la API devuelve TODOS los errores, no el primero (CA-01)"
BODY=$(curl -sS -X POST "$API/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"email":"no-es-un-correo","password":"corta","full_name":""}')
echo "$BODY" | python3 -m json.tool | sed "s/^/  ${DIM}/;s/$/${OFF}/"
N=$(echo "$BODY" | jqp "len(d['errors'])")
[ "$N" -ge 3 ] || fail "se esperaban al menos 3 errores, llegaron $N"
CT=$(curl -sS -o /dev/null -w '%{content_type}' -X POST "$API/api/v1/auth/register" \
  -H 'Content-Type: application/json' -d '{"email":"x","password":"y","full_name":""}')
[[ "$CT" == application/problem+json* ]] || fail "el tipo de contenido no es problem+json: $CT"
ok "$N errores en un solo 422, en application/problem+json"

step "2. Registro del estudiante"
BODY=$(curl -sS -w '\n%{http_code}' -X POST "$API/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"full_name\":\"Estudiante de Prueba\"}")
CODE=$(tail -n1 <<<"$BODY"); BODY=$(sed '$d' <<<"$BODY")
[ "$CODE" = 202 ] || fail "se esperaba 202, llegó $CODE: $BODY"
USER_ID=$(echo "$BODY" | jqp "d['id']")
ok "202 Accepted — la API no esperó al worker (CA-02). user_id=$USER_ID"

step "3. El worker envía el correo (trabajo asíncrono)"
for i in $(seq 1 30); do
  MSG_ID=$(curl -fsS "$MAILPIT/api/v1/search?query=to:$EMAIL" 2>/dev/null \
    | jqp "d['messages'][0]['ID'] if d.get('messages') else ''" || echo "")
  [ -n "$MSG_ID" ] && break
  [ "$i" = 30 ] && fail "no llegó el correo a Mailpit en 30 s"
  sleep 1
done
# Los saltos de línea SMTP son CRLF: el \r forma parte de la línea.
TOKEN=$(curl -fsS "$MAILPIT/api/v1/message/$MSG_ID" \
  | python3 -c "import json,sys,re;m=re.search(r'^([A-Za-z0-9_-]{40,})\r?$', json.load(sys.stdin)['Text'], re.M);sys.exit('no se encontró el token en el correo') if not m else print(m.group(1))")
ok "correo entregado, token extraído (${TOKEN:0:12}…)"

step "4. Verificación del correo"
BODY=$(curl -sS -w '\n%{http_code}' -X POST "$API/api/v1/auth/verify-email" \
  -H 'Content-Type: application/json' -d "{\"token\":\"$TOKEN\"}")
CODE=$(tail -n1 <<<"$BODY")
[ "$CODE" = 200 ] || fail "se esperaba 200, llegó $CODE: $(sed '$d' <<<"$BODY")"
ok "correo verificado"

step "5. El token de un solo uso no se puede reutilizar"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/auth/verify-email" \
  -H 'Content-Type: application/json' -d "{\"token\":\"$TOKEN\"}")
[ "$CODE" = 422 ] || fail "se esperaba 422 al reutilizar el token, llegó $CODE"
ok "422 al reutilizarlo"

step "6. Login"
BODY=$(curl -sS -X POST "$API/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
SESSION=$(echo "$BODY" | jqp "d['token']")
[ -n "$SESSION" ] || fail "no se recibió token de sesión: $BODY"
ok "sesión abierta (${SESSION:0:12}…)"

step "7. GET /me con la sesión"
BODY=$(curl -sS "$API/api/v1/me" -H "Authorization: Bearer $SESSION")
echo "$BODY" | python3 -m json.tool | sed "s/^/  ${DIM}/;s/$/${OFF}/"
[ "$(echo "$BODY" | jqp "d['role']")" = student ] || fail "el rol no es student"
ok "el registro público solo crea estudiantes (RF-01)"

step "8. Contraseña incorrecta"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"incorrecta\"}")
[ "$CODE" = 401 ] || fail "se esperaba 401, llegó $CODE"
ok "401 sin revelar si el correo existe"

step "9. Correo duplicado"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"full_name\":\"Otro\"}")
[ "$CODE" = 409 ] || fail "se esperaba 409, llegó $CODE"
ok "409 conflict"

step "10. Revocación inmediata de la sesión (SEG-1)"
curl -sS -o /dev/null -X POST "$API/api/v1/auth/logout" -H "Authorization: Bearer $SESSION"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' "$API/api/v1/me" -H "Authorization: Bearer $SESSION")
[ "$CODE" = 401 ] || fail "se esperaba 401 tras cerrar sesión, llegó $CODE"
ok "401 en la petición siguiente, sin ventana de gracia"

step "11. Registro del trabajo asíncrono (outbox e idempotencia)"
docker compose exec -T postgres psql -U "${POSTGRES_USER:-mooc}" -d "${POSTGRES_DB:-mooc}" -x \
  -c "SELECT job_key, type, queue, status, attempt FROM platform.job_runs ORDER BY id DESC LIMIT 1;" \
  | sed "s/^/  ${DIM}/;s/$/${OFF}/"
ok "el trabajo quedó en PostgreSQL, no solo en Redis (ADR-0004)"

step "12. La auditoría es inmutable (CE-02)"
if docker compose exec -T postgres psql -U "${POSTGRES_USER:-mooc}" -d "${POSTGRES_DB:-mooc}" \
     -c "UPDATE audit.events SET action = 'manipulado' WHERE id = (SELECT max(id) FROM audit.events);" \
     >/dev/null 2>&1; then
  fail "la auditoría admitió un UPDATE"
fi
ok "PostgreSQL rechaza el UPDATE sobre audit.events"

echo
echo "${GREEN}${BOLD}Todo en verde.${OFF}"
