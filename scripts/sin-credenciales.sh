#!/usr/bin/env bash
# Comprueba que no haya credenciales estáticas de GCP en el repositorio.
#
# ADR-0015 (D3): la autenticación hacia GCP va por Workload Identity
# Federation. Una clave JSON creada "temporalmente" para probar acaba
# versionada, y entonces ya es tarde.
#
# El script se excluye a sí mismo del resultado: contiene los patrones que
# busca. Excluirlo por nombre es preferible a excluir .github entero, que es
# justo uno de los sitios donde alguien pegaría una credencial.
set -euo pipefail

cd "$(dirname "$0")/.."

PATRONES='service_account.*key|credentials\.json|"type":[[:space:]]*"service_account"|BEGIN [A-Z ]*PRIVATE KEY'

hallazgos=$(grep -rInE "$PATRONES" \
  --include='*.go' --include='*.yml' --include='*.yaml' \
  --include='*.json' --include='*.tf' --include='*.tfvars' \
  --include='*.env*' --include='*.sh' \
  --exclude-dir=.git --exclude-dir=node_modules \
  . 2>/dev/null | grep -v '/sin-credenciales\.sh:' || true)

if [ -n "$hallazgos" ]; then
  echo "posible credencial de GCP en el repositorio; ADR-0015 (D3) las prohíbe:" >&2
  echo "$hallazgos" >&2
  exit 1
fi

echo "sin credenciales estáticas"
