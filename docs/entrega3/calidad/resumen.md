# Resumen del análisis de calidad

- Herramienta: SonarQube 26.10.0.133078
- Commit: `ccf2452c7c4b1d3eddb2375ae8f42bfb74a7c125-sucio`
- Fecha del análisis: 2026-10-22T00:23:06+0000
- Perfiles: Azure Resource Manager (Sonar way comprehensive), C# (Sonar way comprehensive), CSS (Sonar way comprehensive), CloudFormation (Sonar way comprehensive), Docker (Sonar way comprehensive), Flex (Sonar way comprehensive), Go (Sonar way comprehensive), HTML (Sonar way comprehensive), IPython Notebooks (Sonar way comprehensive), JSON (Sonar way comprehensive), JSP (Sonar way comprehensive), Java (Sonar way comprehensive), JavaScript (Sonar way comprehensive), Kotlin (Sonar way comprehensive), Kubernetes (Sonar way comprehensive), PHP (Sonar way comprehensive), Python (Sonar way comprehensive), Ruby (Sonar way comprehensive), Rust (Sonar way comprehensive), Scala (Sonar way comprehensive), Secrets (Sonar way comprehensive), Terraform (Sonar way comprehensive), Text (Sonar way comprehensive), TypeScript (Sonar way comprehensive), VB.NET (Sonar way comprehensive), XML (Sonar way comprehensive), YAML (Sonar way comprehensive)
- Líneas de código: 16160 (css=700;docker=65;go=11762;js=1951;py=566;terraform=1079;web=37)


## Umbrales

| Indicador | Medido | Umbral | Cumple |
| --- | --- | --- | --- |
| Duplicación de código | 1.6 % | ≤ 20 % | sí |
| Mantenibilidad | A | A, B o C | sí |
| Deuda técnica estimada | 1162 min (2.4 días de 8 h) | Referencia | |
| Ratio de deuda | 0.2 % | Referencia | |

## Otros indicadores

| Indicador | Valor |
| --- | --- |
| Fiabilidad | C (1 bugs) |
| Seguridad | E (20 vulnerabilidades) |
| Security hotspots por revisar | 0 |
| Code smells | 135 |
| Cobertura | 9.5 % |
| Bloques duplicados | 10 (325 líneas) |

## Hallazgos abiertos por severidad

| Severidad | Cantidad |
| --- | --- |
| BLOCKER | 2 |
| CRITICAL | 32 |
| MAJOR | 83 |
| MINOR | 39 |
| INFO | 0 |

## Por tipo

| Tipo | Cantidad |
| --- | --- |
| CODE_SMELL | 135 |
| VULNERABILITY | 20 |
| BUG | 1 |

## Por regla

| Regla | Severidad | Cantidad |
| --- | --- | --- |
| `terraform:S6329` | BLOCKER | 2 |
| `go:S3776` | CRITICAL | 12 |
| `go:S1192` | CRITICAL | 6 |
| `javascript:S3776` | CRITICAL | 4 |
| `python:S3776` | CRITICAL | 3 |
| `python:S1192` | CRITICAL | 2 |
| `javascript:S2004` | CRITICAL | 2 |
| `javascript:S1186` | CRITICAL | 2 |
| `javascript:S4123` | CRITICAL | 1 |
| `javascript:S4624` | MAJOR | 49 |
| `javascript:S2245` | MAJOR | 11 |
| `javascript:S3358` | MAJOR | 8 |
| `terraform:S6258` | MAJOR | 3 |
| `go:S978` | MAJOR | 2 |
| `go:S107` | MAJOR | 2 |
| `terraform:S6405` | MAJOR | 1 |
| `javascript:S6544` | MAJOR | 1 |
| `Web:S6819` | MAJOR | 1 |
| `javascript:S7767` | MAJOR | 1 |
| `javascript:S8786` | MAJOR | 1 |
| `terraform:S6404` | MAJOR | 1 |
| `css:S4666` | MAJOR | 1 |
| `javascript:S6661` | MAJOR | 1 |
| `javascript:S6582` | MINOR | 12 |
| `javascript:S9382` | MINOR | 8 |
| `javascript:S7778` | MINOR | 4 |
| `go:S117` | MINOR | 4 |
| `javascript:S7726` | MINOR | 2 |
| `docker:S6471` | MINOR | 1 |
| `javascript:S7763` | MINOR | 1 |
| `go:S100` | MINOR | 1 |
| `python:S1481` | MINOR | 1 |
| `terraform:S6412` | MINOR | 1 |
| `javascript:S7750` | MINOR | 1 |
| `javascript:S7781` | MINOR | 1 |
| `javascript:S7773` | MINOR | 1 |
| `javascript:S7776` | MINOR | 1 |

## Archivos con más hallazgos

| Archivo | Cantidad |
| --- | --- |
| `frontend/app.js` | 63 |
| `capacity-planning/k6/escenario1.js` | 15 |
| `capacity-planning/k6/escenario2.js` | 10 |
| `capacity-planning/k6/comun.js` | 6 |
| `backend/internal/modules/media/domain.go` | 5 |
| `capacity-planning/k6/medios.js` | 5 |
| `capacity-planning/k6/preparar.js` | 4 |
| `capacity-planning/k6/preparar-medios.js` | 4 |
| `capacity-planning/metricas/tabla.py` | 4 |
| `frontend/api.js` | 4 |
| `infra/modules/gcs/main.tf` | 3 |
| `infra/modules/generador/main.tf` | 2 |
| `backend/internal/modules/learning/service.go` | 2 |
| `backend/internal/modules/assessment/domain.go` | 2 |
| `capacity-planning/metricas/graficas.py` | 2 |

## Hallazgos Major o superior

| Severidad | Regla | Ubicación | Mensaje |
| --- | --- | --- | --- |
| BLOCKER | `terraform:S6329` | `infra/modules/maquinas/main.tf:61` | Make sure allowing public network access is safe here. |
| BLOCKER | `terraform:S6329` | `infra/modules/generador/main.tf:48` | Make sure allowing public network access is safe here. |
| CRITICAL | `go:S3776` | `backend/internal/modules/learning/service.go:190` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. |
| CRITICAL | `go:S3776` | `backend/internal/modules/assessment/domain.go:160` | Refactor this method to reduce its Cognitive Complexity from 23 to the 15 allowed. |
| CRITICAL | `javascript:S3776` | `capacity-planning/k6/preparar.js:38` | Refactor this function to reduce its Cognitive Complexity from 32 to the 15 allowed. |
| CRITICAL | `python:S1192` | `capacity-planning/metricas/graficas.py:90` | Define a constant instead of duplicating this literal "CPU Worker Server" 3 times. |
| CRITICAL | `go:S1192` | `backend/internal/modules/media/domain.go:123` | Define a constant instead of duplicating this literal "audio/mpeg" 3 times. |
| CRITICAL | `javascript:S2004` | `frontend/app.js:410` | Refactor this code to not nest functions more than 5 levels deep. |
| CRITICAL | `javascript:S1186` | `capacity-planning/k6/preparar-medios.js:59` | Unexpected empty function. |
| CRITICAL | `go:S1192` | `backend/internal/modules/authoring/domain.go:322` | Define a constant instead of duplicating this literal "resource.not_available" 3 times. |
| CRITICAL | `python:S3776` | `capacity-planning/metricas/tabla.py:128` | Refactor this function to reduce its Cognitive Complexity from 16 to the 15 allowed. |
| CRITICAL | `go:S1192` | `backend/internal/platform/httpx/middleware.go:141` | Define a constant instead of duplicating this literal "petición" 3 times. |
| CRITICAL | `go:S3776` | `backend/internal/modules/media/service.go:379` | Refactor this method to reduce its Cognitive Complexity from 29 to the 15 allowed. |
| CRITICAL | `go:S3776` | `backend/internal/adapters/postgres/authoring.go:206` | Refactor this method to reduce its Cognitive Complexity from 19 to the 15 allowed. |
| CRITICAL | `python:S3776` | `capacity-planning/metricas/tabla.py:45` | Refactor this function to reduce its Cognitive Complexity from 18 to the 15 allowed. |
| CRITICAL | `go:S3776` | `backend/cmd/seed/main.go:56` | Refactor this method to reduce its Cognitive Complexity from 25 to the 15 allowed. |
| CRITICAL | `python:S1192` | `capacity-planning/metricas/tabla.py:76` | Define a constant instead of duplicating this literal "/ --- / --- / --- / --- / --- / --- /" 4 times. |
| CRITICAL | `go:S3776` | `backend/internal/modules/authoring/domain.go:276` | Refactor this method to reduce its Cognitive Complexity from 56 to the 15 allowed. |
| CRITICAL | `javascript:S4123` | `frontend/app.js:154` | Unexpected `await` of a non-Promise (non-"Thenable") value. |
| CRITICAL | `python:S3776` | `capacity-planning/metricas/tabla.py:91` | Refactor this function to reduce its Cognitive Complexity from 18 to the 15 allowed. |
| CRITICAL | `go:S3776` | `backend/internal/modules/assessment/http.go:75` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. |
| CRITICAL | `go:S1192` | `backend/internal/modules/identity/http.go:168` | Define a constant instead of duplicating this literal "Se requiere una sesión activa." 3 times. |
| CRITICAL | `javascript:S2004` | `frontend/app.js:418` | Refactor this code to not nest functions more than 5 levels deep. |
| CRITICAL | `go:S3776` | `backend/internal/adapters/objectstore/contrato/contrato.go:39` | Refactor this method to reduce its Cognitive Complexity from 49 to the 15 allowed. |
| CRITICAL | `javascript:S3776` | `frontend/api.js:26` | Refactor this function to reduce its Cognitive Complexity from 20 to the 15 allowed. |
| CRITICAL | `javascript:S3776` | `frontend/app.js:641` | Refactor this function to reduce its Cognitive Complexity from 16 to the 15 allowed. |
| CRITICAL | `go:S3776` | `backend/internal/modules/media/domain.go:166` | Refactor this method to reduce its Cognitive Complexity from 20 to the 15 allowed. |
| CRITICAL | `go:S3776` | `backend/internal/modules/learning/service.go:296` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. |
| CRITICAL | `go:S3776` | `backend/internal/modules/media/sesion_test.go:41` | Refactor this method to reduce its Cognitive Complexity from 24 to the 15 allowed. |
| CRITICAL | `javascript:S1186` | `capacity-planning/k6/preparar.js:143` | Unexpected empty function. |
| CRITICAL | `go:S1192` | `backend/internal/modules/media/domain.go:124` | Define a constant instead of duplicating this literal "image/gif" 3 times. |
| CRITICAL | `go:S3776` | `backend/internal/adapters/postgres/assessment.go:51` | Refactor this method to reduce its Cognitive Complexity from 23 to the 15 allowed. |
| CRITICAL | `go:S1192` | `backend/internal/platform/httpx/middleware.go:64` | Define a constant instead of duplicating this literal "X-Request-Id" 3 times. |
| CRITICAL | `javascript:S3776` | `frontend/app.js:328` | Refactor this function to reduce its Cognitive Complexity from 23 to the 15 allowed. |
| MAJOR | `javascript:S4624` | `frontend/app.js:550` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:538` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S3358` | `frontend/app.js:650` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/escenario1.js:151` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:506` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S3358` | `frontend/app.js:650` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `javascript:S4624` | `frontend/app.js:152` | Refactor this code to not use nested template literals. |
| MAJOR | `terraform:S6405` | `infra/modules/generador/main.tf:62` | Omitting "metadata.block-project-ssh-keys" enables project-wide SSH keys. Make sure it is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:347` | Refactor this code to not use nested template literals. |
| MAJOR | `go:S978` | `backend/internal/modules/assessment/domain.go:250` | Rename this identifier, "max" shadows a predeclared identifier. |
| MAJOR | `javascript:S4624` | `frontend/app.js:568` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S6544` | `frontend/app.js:319` | Promise-returning function provided to variable where a void return was expected. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/escenario1.js:160` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S3358` | `frontend/app.js:243` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `terraform:S6258` | `infra/modules/gcs/main.tf:18` | Omitting "logging" makes logs incomplete. Make sure it is safe here. |
| MAJOR | `Web:S6819` | `frontend/index.html:31` | Use <output> instead of the status role to ensure accessibility across all devices. |
| MAJOR | `javascript:S4624` | `frontend/app.js:210` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:967` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:894` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:592` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:1042` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:538` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:81` | Refactor this code to not use nested template literals. |
| MAJOR | `go:S107` | `backend/internal/adapters/postgres/learning.go:273` | This function has 9 parameters, which is greater than the 7 authorized. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/comun.js:37` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S7767` | `capacity-planning/k6/comun.js:37` | Use `Math.trunc` instead of `/ 0`. |
| MAJOR | `javascript:S3358` | `frontend/app.js:243` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `javascript:S4624` | `frontend/app.js:179` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:13` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/escenario1.js:185` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:280` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:774` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S3358` | `frontend/app.js:247` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/escenario1.js:170` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:179` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/preparar.js:154` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:538` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:874` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:615` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:243` | Refactor this code to not use nested template literals. |
| MAJOR | `go:S107` | `backend/internal/adapters/objectstore/contrato/contrato.go:169` | This function has 8 parameters, which is greater than the 7 authorized. |
| MAJOR | `go:S978` | `backend/internal/modules/badges/imagen.go:86` | Rename this identifier, "max" shadows a predeclared identifier. |
| MAJOR | `javascript:S3358` | `capacity-planning/k6/escenario2.js:112` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `javascript:S8786` | `capacity-planning/k6/medios.js:97` | Simplify this regular expression to reduce its runtime, as it has super-linear performance due to backtracking. |
| MAJOR | `javascript:S4624` | `frontend/app.js:20` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:568` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S3358` | `frontend/app.js:243` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `terraform:S6404` | `infra/modules/gcs/main.tf:67` | Ensure that granting public access to this resource is safe here. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/escenario1.js:184` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:538` | Refactor this code to not use nested template literals. |
| MAJOR | `css:S4666` | `frontend/styles.css:699` | Duplicate selector "h1,
h2,
h3", first used at line 114 |
| MAJOR | `javascript:S4624` | `frontend/app.js:292` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/api.js:76` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/escenario1.js:122` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `terraform:S6258` | `infra/modules/cloudsql/main.tf:17` | Omitting "database_flags.log_connections" makes logs incomplete. Make sure it is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:232` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `capacity-planning/k6/comun.js:110` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:232` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:538` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/comun.js:91` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:550` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S3358` | `frontend/api.js:76` | Extract this nested ternary operation into an independent statement. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:81` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/escenario1.js:121` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `terraform:S6258` | `infra/modules/red/main.tf:11` | Omitting "log_config" makes logs incomplete. Make sure it is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:774` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:347` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:506` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:650` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S6661` | `capacity-planning/k6/escenario1.js:50` | Use an object spread instead of `Object.assign` eg: `{ ...foo }`. |
| MAJOR | `javascript:S2245` | `capacity-planning/k6/escenario2.js:79` | Make sure that using this pseudorandom number generator is safe here. |
| MAJOR | `javascript:S4624` | `frontend/app.js:152` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:538` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:81` | Refactor this code to not use nested template literals. |
| MAJOR | `javascript:S4624` | `frontend/app.js:550` | Refactor this code to not use nested template literals. |
