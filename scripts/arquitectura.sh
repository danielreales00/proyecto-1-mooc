#!/usr/bin/env bash
# Comprueba las reglas del ADR-0001 y del ADR-0002 sobre el propio código.
#
# El ADR-0001 promete un `make arch-test` que falle si el dominio importa
# infraestructura, y el ADR-0002 promete que el CI verifique las dependencias
# contra la lista aprobada. Ninguna de las dos existía: un ADR que anuncia una
# comprobación y no la tiene es una promesa, no una regla.
set -euo pipefail
cd "$(dirname "$0")/../backend"

ROJO=$'\033[31m'; VERDE=$'\033[32m'; DIM=$'\033[2m'; OFF=$'\033[0m'
fallos=0
mal() { echo "${ROJO}✗ $*${OFF}" >&2; fallos=$((fallos+1)); }
ok()  { echo "  ${VERDE}✓${OFF} $*"; }

echo "── ADR-0001 · el dominio no importa infraestructura ──"
# domain.go es dominio puro: entidades y reglas. No puede importar adaptadores,
# ni el framework HTTP, ni el cliente de base de datos.
prohibidos='mooc/backend/internal/adapters|net/http|github.com/jackc/pgx|github.com/redis|github.com/minio|github.com/hibiken'
for f in internal/modules/*/domain.go; do
    [ -e "$f" ] || continue
    imports=$(awk '/^import \(/{f=1;next}/^\)/{f=0}f' "$f")
    if echo "$imports" | grep -qE "$prohibidos"; then
        mal "$f importa infraestructura:"
        echo "$imports" | grep -E "$prohibidos" | sed 's/^/      /'
    else
        ok "$(basename "$(dirname "$f")")/domain.go es dominio puro"
    fi
done

echo
echo "── ADR-0001 · ningún módulo ESCRIBE en el esquema de otro ──"
# La regla se enmendó el 2026-09-07: las lecturas cruzadas se permiten en la
# capa de adaptadores —un JOIN es más simple y más rápido que reunir en memoria
# lo que la base sabe reunir—, pero las escrituras no: escribir en la tabla de
# otro módulo se salta sus reglas de negocio.
declare -A duenio=(
  [identity]=identity [authoring]=authoring [media]=media
  [assessment]=assessment [progress]=learning [badges]=badges [audit]=audit
)
# Excepción documentada en el ADR-0001: admin e identity son el mismo contexto
# con dos caras, la del usuario y la administrativa, y comparten el agregado.
excepciones='postgres/admin.go:.*identity\.'

for esquema in "${!duenio[@]}"; do
    propietario="${duenio[$esquema]}"
    escrituras=$(grep -rnE "(INSERT INTO|UPDATE|DELETE FROM)[[:space:]]+${esquema}\.[a-z_]+" \
        internal/adapters/ internal/modules/ 2>/dev/null \
        | grep -v '_test.go' | grep -vE '^[^:]+:[0-9]+:[[:space:]]*//' \
        | grep -v "/${propietario}\.go:" | grep -v "/${propietario}/" \
        | grep -vE "$excepciones" || true)
    if [ -n "$escrituras" ]; then
        mal "el esquema '$esquema' (de $propietario) se ESCRIBE desde otro módulo:"
        echo "$escrituras" | sed 's/^/      /' | cut -c1-110
    fi
done
[ $fallos -eq 0 ] && ok "ningún módulo escribe en el esquema de otro"

# Las lecturas cruzadas se cuentan, no se prohíben: son el precio de extraer un
# módulo algún día, y conviene tenerlo a la vista.
lecturas=0
for esquema in "${!duenio[@]}"; do
    propietario="${duenio[$esquema]}"
    n=$(grep -rncE "(FROM|JOIN)[[:space:]]+${esquema}\.[a-z_]+" internal/adapters/ 2>/dev/null \
        | grep -v "/${propietario}\.go:" | awk -F: '{s+=$2} END {print s+0}')
    lecturas=$((lecturas+n))
done
echo "  ${DIM}lecturas cruzadas: $lecturas (permitidas; es lo que costaría extraer un módulo)${OFF}"

echo "── ADR-0002 · dependencias contra la lista aprobada ──"
aprobadas=(
  github.com/jackc/pgx/v5
  github.com/hibiken/asynq
  github.com/redis/go-redis/v9
  github.com/minio/minio-go/v7
  github.com/google/uuid
  github.com/golang-jwt/jwt/v5
  golang.org/x/crypto
  github.com/yuin/goldmark
  github.com/microcosm-cc/bluemonday
  github.com/prometheus/client_golang
)
directas=$(awk '/^require \(/{f=1;next}/^\)/{f=0}f && !/indirect/{print $1}' go.mod | grep -v '^$')
for d in $directas; do
    encontrada=0
    for a in "${aprobadas[@]}"; do [ "$d" = "$a" ] && encontrada=1; done
    if [ $encontrada -eq 0 ]; then
        mal "dependencia directa NO aprobada en el ADR-0002: $d"
    fi
done
echo "  ${DIM}$(echo "$directas" | wc -l) dependencias directas${OFF}"
[ $fallos -eq 0 ] && ok "todas están en la lista del ADR-0002"

echo
echo "── ADR-0010 · nada específico de un proveedor en el dominio ──"
if grep -rn "cloud.google.com\|aws-sdk" internal/modules/ 2>/dev/null | grep -v '_test.go'; then
    mal "hay código específico de un proveedor dentro de los módulos"
else
    ok "los módulos no conocen a ningún proveedor cloud"
fi

echo
if [ $fallos -gt 0 ]; then
    echo "${ROJO}$fallos regla(s) de arquitectura incumplida(s).${OFF}"
    exit 1
fi
echo "${VERDE}Las reglas de arquitectura se cumplen.${OFF}"
