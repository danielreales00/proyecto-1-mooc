#!/usr/bin/env bash
# Comprueba que el contrato y el código no se separen.
#
# El OpenAPI marca con `x-estado: planificado` lo que todavía no responde. Este
# script toma todo lo que NO lleva esa marca y comprueba que la ruta exista de
# verdad en la API en ejecución: un 404 significa que el contrato promete algo
# que el router no sirve.
#
# También comprueba lo contrario: que el contrato servido por la API sea
# exactamente el del repositorio.
set -euo pipefail

API="${API:-http://localhost:8090}"
SPEC="${SPEC:-backend/openapi/openapi.yaml}"

cd "$(dirname "$0")/.."

echo "1. La API sirve el mismo contrato que hay en el repositorio"
curl -fsS "$API/openapi.yaml" -o /tmp/contrato-servido.yaml
if ! cmp -s /tmp/contrato-servido.yaml "$SPEC"; then
  echo "   el contrato servido no coincide con $SPEC" >&2
  echo "   (¿se reconstruyó la imagen tras editarlo?)" >&2
  exit 1
fi
echo "   ok"

echo "2. Las rutas declaradas como implementadas existen"
python3 - "$API" "$SPEC" <<'PY'
import subprocess, sys, yaml

api, ruta_spec = sys.argv[1], sys.argv[2]
spec = yaml.safe_load(open(ruta_spec))

METODOS = ("get", "post", "patch", "put", "delete")
fallos = []
comprobadas = 0

for ruta, operaciones in spec["paths"].items():
    for metodo, op in operaciones.items():
        if metodo not in METODOS or not isinstance(op, dict):
            continue
        if op.get("x-estado") == "planificado":
            continue
        if "{" in ruta:
            continue  # necesitaría datos; lo cubre la colección de Postman

        cmd = ["curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
               "-X", metodo.upper(), f"{api}{ruta}"]
        if metodo in ("post", "patch", "put"):
            cmd += ["-H", "Content-Type: application/json", "-d", "{}"]

        codigo = subprocess.run(cmd, capture_output=True, text=True).stdout.strip()
        comprobadas += 1
        # Cualquier código sirve menos 404: lo que se comprueba es que la ruta
        # esté enrutada, no que la petición vacía tenga sentido.
        if codigo == "404":
            fallos.append(f"{metodo.upper()} {ruta} -> 404, el contrato la declara implementada")

if fallos:
    print("   rutas del contrato que la API no sirve:", file=sys.stderr)
    for f in fallos:
        print(f"     {f}", file=sys.stderr)
    sys.exit(1)

print(f"   ok, {comprobadas} rutas comprobadas")
PY

echo
echo "Contrato y API coinciden."
