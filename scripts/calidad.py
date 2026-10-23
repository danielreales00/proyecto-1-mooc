#!/usr/bin/env python3
"""Prepara SonarQube, exporta el análisis y verifica los umbrales (ADR-0018).

El mismo script corre en local (`make calidad`, dentro de mooc-herramientas) y
en el CI (job «Calidad»). Solo usa la biblioteca estándar.

  calidad.py preparar              # espera al servidor e imprime un token nuevo
  calidad.py verificar <salida>    # espera el análisis, exporta y verifica

`verificar` escribe en <salida>:
  medidas.json        los indicadores del proyecto completo
  hallazgos.json      todos los hallazgos abiertos, sin recortar
  puntos-calientes.json  los security hotspots por revisar
  resumen.md          las cifras que cita docs/entrega3/deuda-tecnica.md

Los umbrales se miden sobre el código total, no sobre el código nuevo, y por
eso no se confían a la quality gate: la de SonarQube mira el código nuevo.
"""
import base64
import json
import os
import sys
import time
import urllib.parse
import urllib.request
from collections import Counter
from pathlib import Path

SERVIDOR = os.environ.get("SONAR_HOST_URL", "http://sonarqube:9000").rstrip("/")
PROYECTO = os.environ.get("SONAR_PROJECT_KEY", "mooc")
# El servidor es efímero en el CI y solo escucha en 127.0.0.1 en local: la
# contraseña de fábrica no protege nada que no esté ya en el repositorio.
ADMIN = os.environ.get("SONAR_ADMIN", "admin:admin")
PERFIL = "Sonar way comprehensive"

# Umbrales del proyecto para la Entrega 3.
MAX_DUPLICACION = 20.0  # % de líneas duplicadas
MAX_MANTENIBILIDAD = 3.0  # calificación: 1=A, 2=B, 3=C, 4=D, 5=E

MEDIDAS = [
    "ncloc", "ncloc_language_distribution", "files", "functions",
    "duplicated_lines_density", "duplicated_lines", "duplicated_blocks",
    "sqale_rating", "sqale_index", "sqale_debt_ratio", "code_smells",
    "reliability_rating", "bugs", "security_rating", "vulnerabilities",
    "security_hotspots", "security_review_rating",
    "coverage", "lines_to_cover", "uncovered_lines",
    "cognitive_complexity", "complexity",
]
LETRA = {1: "A", 2: "B", 3: "C", 4: "D", 5: "E"}
SEVERIDADES = ["BLOCKER", "CRITICAL", "MAJOR", "MINOR", "INFO"]


def api(ruta, datos=None, metodo=None, **params):
    url = f"{SERVIDOR}/api/{ruta}"
    if params:
        url += "?" + urllib.parse.urlencode(params)
    cuerpo = urllib.parse.urlencode(datos).encode() if datos is not None else None
    req = urllib.request.Request(url, data=cuerpo, method=metodo)
    if metodo == "PATCH":
        # La API v2 recibe JSON, no formulario.
        req.data = json.dumps(datos).encode()
        req.add_header("Content-Type", "application/merge-patch+json")
    req.add_header("Authorization", "Basic " + base64.b64encode(ADMIN.encode()).decode())
    with urllib.request.urlopen(req, timeout=30) as r:
        texto = r.read()
    return json.loads(texto) if texto else {}


def esperar(condicion, que, limite=600):
    fin = time.time() + limite
    while time.time() < fin:
        try:
            if condicion():
                return
        except OSError:
            pass
        time.sleep(3)
    sys.exit(f"SonarQube no {que} en {limite} s")


def preparar():
    esperar(lambda: api("system/status")["status"] == "UP", "arrancó")
    # Severidades clásicas (Blocker, Critical, Major...) en vez de las del modo
    # MQR: son las que usan el documento de deuda y los criterios del proyecto.
    api("v2/clean-code-policy/mode", {"mode": "STANDARD_EXPERIENCE"}, metodo="PATCH")
    # Perfil «Sonar way comprehensive» en todos los lenguajes. El de fábrica
    # («core») activa 6 reglas de Go de 37, y Go es casi tres cuartas partes del
    # código: con él, el análisis del backend no diría nada.
    for perfil in api("qualityprofiles/search")["profiles"]:
        if perfil["name"] == PERFIL and not perfil["isDefault"]:
            api("qualityprofiles/set_default", {"language": perfil["language"], "qualityProfile": PERFIL})
    nombre = "analisis"
    api("user_tokens/revoke", {"name": nombre})
    print(api("user_tokens/generate", {"name": nombre})["token"])


def tabla(titulo, *columnas):
    """Encabezado de una sección del resumen con su tabla en Markdown."""
    return ["", f"## {titulo}", "", "| " + " | ".join(columnas) + " |", "|" + " --- |" * len(columnas)]


def todas(ruta, clave, **params):
    filas, pagina = [], 1
    while True:
        r = api(ruta, ps=500, p=pagina, **params)
        filas += r[clave]
        total = r.get("paging", {}).get("total", r.get("total", 0))
        # La API de hallazgos no pagina más allá de 10 000 resultados.
        if len(filas) >= min(total, 10000) or not r[clave]:
            return filas
        pagina += 1


def verificar(salida):
    salida = Path(salida)
    salida.mkdir(parents=True, exist_ok=True)

    # El scanner deja el identificador de la tarea en el servidor. Hasta que esa
    # tarea termine, las medidas son las del análisis anterior.
    reporte = Path(os.environ.get("SONAR_REPORT_TASK", ".scannerwork/report-task.txt"))
    tarea = {}
    for linea in reporte.read_text().splitlines():
        clave, _, valor = linea.partition("=")
        tarea[clave] = valor
    estado = lambda: api("ce/task", id=tarea["ceTaskId"])["task"]["status"]
    esperar(lambda: estado() in ("SUCCESS", "FAILED", "CANCELED"), "procesó el análisis")
    if estado() != "SUCCESS":
        sys.exit(f"el análisis terminó en {estado()}")
    analisis = api("ce/task", id=tarea["ceTaskId"])["task"]

    servidor = api("system/status")["version"]
    medidas = {m["metric"]: m.get("value")
               for m in api("measures/component", component=PROYECTO,
                            metricKeys=",".join(MEDIDAS))["component"]["measures"]}
    hallazgos = todas("issues/search", "issues", componentKeys=PROYECTO,
                      resolved="false", s="SEVERITY", asc="false")
    calientes = todas("hotspots/search", "hotspots", project=PROYECTO, status="TO_REVIEW")
    perfiles = api("qualityprofiles/search", project=PROYECTO)["profiles"]

    meta = {
        "servidor": f"SonarQube {servidor}",
        "proyecto": PROYECTO,
        "commit": os.environ.get("GITHUB_SHA") or os.environ.get("COMMIT", "desconocido"),
        "fecha_analisis": analisis.get("executedAt"),
        "perfiles": {p["languageName"]: p["name"] for p in perfiles},
    }
    (salida / "medidas.json").write_text(json.dumps({**meta, "medidas": medidas}, indent=2, ensure_ascii=False))
    (salida / "hallazgos.json").write_text(json.dumps(hallazgos, indent=2, ensure_ascii=False))
    (salida / "puntos-calientes.json").write_text(json.dumps(calientes, indent=2, ensure_ascii=False))

    duplicacion = float(medidas["duplicated_lines_density"])
    mantenibilidad = float(medidas["sqale_rating"])
    minutos = int(medidas["sqale_index"])
    checks = [
        ("Duplicación de código", f"{duplicacion:.1f} %", f"≤ {MAX_DUPLICACION:.0f} %", duplicacion <= MAX_DUPLICACION),
        ("Mantenibilidad", LETRA[int(mantenibilidad)], "A, B o C", mantenibilidad <= MAX_MANTENIBILIDAD),
    ]

    por_sev = Counter(h["severity"] for h in hallazgos)
    por_tipo = Counter(h["type"] for h in hallazgos)
    por_regla = Counter((h["rule"], h["severity"]) for h in hallazgos)
    por_archivo = Counter(h["component"].split(":", 1)[-1] for h in hallazgos)
    graves = [h for h in hallazgos if h["severity"] in ("BLOCKER", "CRITICAL", "MAJOR")]

    l = ["# Resumen del análisis de calidad", "",
         f"- Herramienta: {meta['servidor']}",
         f"- Commit: `{meta['commit']}`",
         f"- Fecha del análisis: {meta['fecha_analisis']}",
         "- Perfiles: " + ", ".join(f"{k} ({v})" for k, v in sorted(meta["perfiles"].items())),
         f"- Líneas de código: {medidas['ncloc']} ({medidas.get('ncloc_language_distribution')})", "",
         *tabla("Umbrales", "Indicador", "Medido", "Umbral", "Cumple")]
    l += [f"| {n} | {v} | {u} | {'sí' if ok else 'no'} |" for n, v, u, ok in checks]
    l += [f"| Deuda técnica estimada | {minutos} min ({minutos / 480:.1f} días de 8 h) | Referencia | |",
          f"| Ratio de deuda | {medidas.get('sqale_debt_ratio')} % | Referencia | |",
          *tabla("Otros indicadores", "Indicador", "Valor"),
          f"| Fiabilidad | {LETRA[int(float(medidas['reliability_rating']))]} ({medidas['bugs']} bugs) |",
          f"| Seguridad | {LETRA[int(float(medidas['security_rating']))]} ({medidas['vulnerabilities']} vulnerabilidades) |",
          f"| Security hotspots por revisar | {len(calientes)} |",
          f"| Code smells | {medidas['code_smells']} |",
          f"| Cobertura | {medidas.get('coverage')} % |",
          f"| Bloques duplicados | {medidas['duplicated_blocks']} ({medidas['duplicated_lines']} líneas) |",
          *tabla("Hallazgos abiertos por severidad", "Severidad", "Cantidad")]
    l += [f"| {s} | {por_sev.get(s, 0)} |" for s in SEVERIDADES]
    l += tabla("Por tipo", "Tipo", "Cantidad")
    l += [f"| {t} | {n} |" for t, n in por_tipo.most_common()]
    l += tabla("Por regla", "Regla", "Severidad", "Cantidad")
    l += [f"| `{r}` | {s} | {n} |" for (r, s), n in sorted(por_regla.items(), key=lambda x: (SEVERIDADES.index(x[0][1]), -x[1]))]
    l += tabla("Archivos con más hallazgos", "Archivo", "Cantidad")
    l += [f"| `{a}` | {n} |" for a, n in por_archivo.most_common(15)]
    l += tabla("Hallazgos Major o superior", "Severidad", "Regla", "Ubicación", "Mensaje")
    l += [f"| {h['severity']} | `{h['rule']}` | `{h['component'].split(':', 1)[-1]}:{h.get('line', '-')}` | {h['message'].replace('|', '/')} |"
          for h in graves]
    (salida / "resumen.md").write_text("\n".join(l) + "\n")

    for n, v, u, ok in checks:
        print(f"{'ok   ' if ok else 'FALLA'} {n}: {v} (umbral {u})")
    print(f"      Deuda técnica: {minutos} min. Hallazgos abiertos: {len(hallazgos)} "
          f"({', '.join(f'{s} {por_sev.get(s, 0)}' for s in SEVERIDADES)})")
    print(f"      Exportado en {salida}/")
    if not all(ok for *_, ok in checks):
        sys.exit(1)


if __name__ == "__main__":
    if sys.argv[1:2] == ["preparar"]:
        preparar()
    elif sys.argv[1:2] == ["verificar"] and len(sys.argv) == 3:
        verificar(sys.argv[2])
    else:
        sys.exit(__doc__)
