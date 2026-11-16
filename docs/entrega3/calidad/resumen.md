# Resumen del análisis de calidad

- Herramienta: SonarQube 26.10.0.133078
- Commit: `5d79cbd6b29d51dca0f780399f47b663013eae82`
- Fecha del análisis: 2026-11-16T02:53:57+0000
- Perfiles: Azure Resource Manager (Sonar way comprehensive), C# (Sonar way comprehensive), CSS (Sonar way comprehensive), CloudFormation (Sonar way comprehensive), Docker (Sonar way comprehensive), Flex (Sonar way comprehensive), Go (Sonar way comprehensive), HTML (Sonar way comprehensive), IPython Notebooks (Sonar way comprehensive), JSON (Sonar way comprehensive), JSP (Sonar way comprehensive), Java (Sonar way comprehensive), JavaScript (Sonar way comprehensive), Kotlin (Sonar way comprehensive), Kubernetes (Sonar way comprehensive), PHP (Sonar way comprehensive), Python (Sonar way comprehensive), Ruby (Sonar way comprehensive), Rust (Sonar way comprehensive), Scala (Sonar way comprehensive), Secrets (Sonar way comprehensive), Terraform (Sonar way comprehensive), Text (Sonar way comprehensive), TypeScript (Sonar way comprehensive), VB.NET (Sonar way comprehensive), XML (Sonar way comprehensive), YAML (Sonar way comprehensive)
- Líneas de código: 19196 (css=878;docker=76;go=12373;js=2446;py=724;terraform=2646;web=53)


## Umbrales

| Indicador | Medido | Umbral | Cumple |
| --- | --- | --- | --- |
| Duplicación de código | 1.3 % | ≤ 20 % | sí |
| Mantenibilidad | A | A, B o C | sí |
| Deuda técnica estimada | 37 min (0.1 días de 8 h) | Referencia | |
| Ratio de deuda | 0.0 % | Referencia | |

## Otros indicadores

| Indicador | Valor |
| --- | --- |
| Fiabilidad | D (1 bugs) |
| Seguridad | A (0 vulnerabilidades) |
| Security hotspots por revisar | 0 |
| Code smells | 6 |
| Cobertura | 9.7 % |
| Bloques duplicados | 10 (329 líneas) |

## Hallazgos abiertos por severidad

| Severidad | Cantidad |
| --- | --- |
| BLOCKER | 0 |
| CRITICAL | 1 |
| MAJOR | 5 |
| MINOR | 1 |
| INFO | 0 |

## Por tipo

| Tipo | Cantidad |
| --- | --- |
| CODE_SMELL | 6 |
| BUG | 1 |

## Por regla

| Regla | Severidad | Cantidad |
| --- | --- | --- |
| `css:S4657` | CRITICAL | 1 |
| `css:S4666` | MAJOR | 2 |
| `go:S107` | MAJOR | 1 |
| `javascript:S3358` | MAJOR | 1 |
| `javascript:S7721` | MAJOR | 1 |
| `javascript:S9382` | MINOR | 1 |

## Archivos con más hallazgos

| Archivo | Cantidad |
| --- | --- |
| `frontend/styles.css` | 3 |
| `frontend/app.js` | 3 |
| `backend/cmd/worker/push.go` | 1 |

## Hallazgos Major o superior

| Severidad | Regla | Ubicación | Mensaje |
| --- | --- | --- | --- |
| CRITICAL | `css:S4657` | `frontend/styles.css:483` | Overridden property "text-decoration-color" by shorthand "text-decoration" |
| MAJOR | `css:S4666` | `frontend/styles.css:882` | Duplicate selector ".progress span", first used at line 512 |
| MAJOR | `css:S4666` | `frontend/styles.css:879` | Duplicate selector ".progress", first used at line 505 |
| MAJOR | `go:S107` | `backend/cmd/worker/push.go:26` | This function has 8 parameters, which is greater than the 7 authorized. |
| MAJOR | `javascript:S3358` | `frontend/app.js:1364` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `javascript:S7721` | `frontend/app.js:1450` | Move function 'bindRecent' to the outer scope. |
