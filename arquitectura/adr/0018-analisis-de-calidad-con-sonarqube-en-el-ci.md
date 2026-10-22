# ADR-0018 — Análisis de calidad con SonarQube Community en el CI

- **Estado:** Aceptado
- **Fecha:** 2026-10-21
- **Requisitos:** calidad del código y deuda técnica de la Entrega 3
- **Reemplaza / reemplazado por:** corrige la fila «Calidad» de `alcance-entrega-3.md`, que proponía SonarQube Cloud

## Contexto

La Entrega 3 exige ejecutar SonarQube sobre el código del proyecto desde el
pipeline de integración continua, con dos umbrales sobre el código total:
duplicación de a lo sumo 20 % y mantenibilidad C o mejor. La deuda estimada
por la herramienta se registra como primera medición de referencia. El reporte
del commit final de la entrega es la línea base de las siguientes.

El proyecto ya sostiene una regla para sus herramientas: el CI y la máquina de
cada integrante ejecutan exactamente la misma versión (`Makefile`, versiones de
`govulncheck` y `gosec`). Un diagnóstico hecho en local que no coincide con el
del pipeline obliga a explicar diferencias que no vienen del código.

La quality gate de SonarQube evalúa por defecto el código nuevo. Los umbrales
del proyecto se miden sobre el código total.

## Decisión

SonarQube Community Build 26.10.0.133078 corre como servicio efímero dentro
del job «Calidad» de GitHub Actions, y la misma imagen corre en local con
`make calidad`.

- `sonar-project.properties` fija el alcance. Analiza el repositorio entero
  (Go, interfaz, scripts, despliegue, Terraform, guiones de carga) y excluye
  solo código de terceros y salidas de las pruebas, cada exclusión con su razón.
- El scanner es la CLI 8.1.0.6389, en la imagen
  `sonarsource/sonar-scanner-cli:12.2.0.4256_8.1.0` en local y con
  `scannerVersion` en `SonarSource/sonarqube-scan-action@v8.3.0` en el CI.
- `scripts/calidad.py` prepara el servidor, espera a que termine el análisis,
  exporta medidas, hallazgos y hotspots en JSON, escribe un resumen en Markdown
  y falla si algún umbral no se cumple. No usa la quality gate porque esta mira
  el código nuevo.
- El perfil de reglas es «Sonar way comprehensive» en todos los lenguajes. El
  de fábrica, «Sonar way core», activa 6 de las 37 reglas de Go, y Go es el
  73 % del código analizado. Con él el backend salía sin un solo hallazgo, y
  eso decía más del perfil que del código. La única regla desactivada es
  `go:S1135`, que lee la palabra española «todo» como un `TODO`.
- El servidor trabaja con las severidades clásicas (Blocker, Critical, Major,
  Minor, Info), que son las que usa el documento de deuda.
- La exportación de cada ejecución del CI queda en el artefacto
  `calidad-<sha>`. La del commit final se copia en `docs/entrega3/calidad/`.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| SonarQube Cloud | Gratis para repositorios públicos. El reporte queda navegable en la web | Exige crear una organización y un secreto `SONAR_TOKEN` fuera del repositorio. Su versión de reglas avanza sin aviso y no se puede reproducir en local. Los umbrales sobre código total requieren una quality gate propia | El diagnóstico local y el del pipeline podrían diferir sin que cambie el código |
| SonarQube en una VM del proyecto | Reporte navegable y persistente | Una máquina más que pagar, parchear y apagar, dentro del presupuesto de 50 USD | El costo no aporta nada que el artefacto del CI no dé |
| golangci-lint y ESLint sin SonarQube | Ya conocidos, rápidos | La entrega pide SonarQube o una equivalencia documentada de indicadores y severidades. Ninguno estima deuda ni califica mantenibilidad | Habría que construir a mano la equivalencia |

## Consecuencias

- El análisis no depende de cuentas ni secretos externos. Un clon nuevo lo
  reproduce con `make calidad`.
- El reporte de cada commit es un artefacto descargable del run, no una página
  web. El enlace al run y la exportación versionada sustituyen la vista
  navegable.
- El job tarda unos minutos más porque arranca el servidor en cada ejecución.
- El servidor de SonarQube usa la contraseña de fábrica. En el CI es efímero y
  en local solo escucha en `127.0.0.1`.
- Subir la versión de SonarQube o del scanner cambia las reglas y por tanto los
  números. Se trata como un cambio de línea base y se anota en
  `docs/entrega3/deuda-tecnica.md`.

## Cómo se verifica

- `make calidad` imprime los dos umbrales con `ok` y escribe
  `docs/entrega3/calidad/`.
- El job «Calidad (SonarQube)» del CI termina en verde y publica el artefacto
  `calidad-<sha>` con `medidas.json`, `hallazgos.json`,
  `puntos-calientes.json` y `resumen.md`.
