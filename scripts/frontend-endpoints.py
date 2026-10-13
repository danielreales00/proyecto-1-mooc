#!/usr/bin/env python3
"""Audita todas las rutas reales frente al contrato y genera el inventario."""
import json
import re
from pathlib import Path
import yaml

root = Path('/repo')
spec = yaml.safe_load((root / 'backend/openapi/openapi.yaml').read_text())
routes = []
for source in (root / 'backend/internal/modules').glob('*/http.go'):
    routes.extend((source.parent.name, method, path) for method, path in re.findall(
        r'mux.Handle(?:Func)?\("(GET|POST|PUT|PATCH|DELETE) ([^"]+)"', source.read_text()))
routes.extend(('platform', method, path) for method, path in re.findall(
    r'mux.Handle(?:Func)?\("(GET|POST|PUT|PATCH|DELETE) ([^"]+)"',
    (root / 'backend/cmd/api/main.go').read_text()))
normalize = lambda path: re.sub(r'\{[^}]+\}', '{}', path)
implemented = {(method.lower(), normalize(path)) for _, method, path in routes}
operations = [(method.upper(), path, operation) for path, methods in spec['paths'].items()
              for method, operation in methods.items() if method in ['get', 'post', 'patch', 'put', 'delete']]
missing = [(method, path) for _, method, path in routes if path.startswith('/api/')
           and not any(method == m and normalize(path) == normalize(p) for m, p, _ in operations)]
assert not missing, f'Rutas sin contrato: {missing}'
labels = {'identity': 'Cuenta y sesión', 'learning': 'Catálogo y aula', 'assessment': 'Evaluación y editor de quiz',
          'authoring': 'Estudio docente', 'admin': 'Administración', 'badges': 'Insignias y verificación',
          'media': 'Biblioteca y reproductor', 'platform': 'Operación y documentación'}
lines = ['# Inventario de endpoints', '',
         'Generado con `docker compose run --rm herramientas python3 scripts/frontend-endpoints.py`.', '',
         f'El router registra **{len(routes)} operaciones disponibles**. El contrato declara **{len(operations)} operaciones**; '
         'las que no están registradas no se presentan como funciones disponibles en la interfaz.', '',
         '| Módulo | Método | Ruta real | Interfaz |', '| --- | --- | --- | --- |']
lines.extend(f'| {module} | {method} | `{path}` | {labels[module]} |' for module, method, path in sorted(routes))
lines.extend(['', '## Operaciones del contrato sin ruta registrada', '', '| Método | Ruta | Estado declarado |', '| --- | --- | --- |'])
lines.extend(f'| {method} | `{path}` | {op.get("x-estado", "sin etiqueta")} |'
             for method, path, op in operations if (method.lower(), normalize(path)) not in implemented)
lines.extend(['', 'Los manifiestos y playlists HLS los consume el reproductor. Las partes de carga se envían por PUT a URLs firmadas del almacén, sin token de usuario. `/metrics` solo se expone dentro de la máquina en producción.'])
(root / 'frontend/ENDPOINTS.md').write_text('\n'.join(lines) + '\n')
print(json.dumps({'available': len(routes), 'contract_operations': len(operations), 'unregistered': len(operations)-sum((m.lower(),normalize(p)) in implemented for m,p,_ in operations), 'uncontracted': missing}))
