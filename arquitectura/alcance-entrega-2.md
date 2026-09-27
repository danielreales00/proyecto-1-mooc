# Alcance de la Entrega 2 — Despliegue básico en la nube

Fuente: `2026-20 Entrega 2 - Despliegue Básico en la Nube.pdf`. Este documento
es nuestra lectura del enunciado: qué entra, qué no entra y qué cambia respecto
a lo que teníamos planeado. Ante discrepancia, manda el PDF.

**Duración:** una semana y media. Los cuatro integrantes participan en
implementación, pruebas, análisis y sustentación.

---

## La frase que lo ordena todo

> En esta entrega no se espera integrar nuevas funcionalidades ni desarrollar el
> frontend. El objetivo es avanzar en un primer despliegue cloud de la solución
> existente.

**No se escribe funcionalidad nueva de producto.** Todo el trabajo es
infraestructura, despliegue, verificación y medición. Lo único que se escribe de
backend es lo que el despliegue exige y hoy no existe (ver «Trabajo de código»).

---

## Qué se evalúa

| Componente | Peso |
| --- | --- |
| Despliegue e integración de componentes | **50 %** |
| Configuración del servicio administrado de bases de datos | 10 % |
| Funcionamiento de la plataforma y configuración de red | 10 % |
| Análisis de capacidad: escenario 1 | 10 % |
| Análisis de capacidad: escenario 2 | 10 % |
| Documentación de arquitectura | 10 % |

Leído de otra forma: **70 % es que funcione desplegado, 20 % es medirlo y 10 %
es contarlo.** Cada criterio se respalda con evidencia reproducible del entorno
desplegado, no con capturas sueltas.

---

## La arquitectura obligatoria

Esto no se elige. Lo fija el enunciado.

| Recurso | Qué aloja |
| --- | --- |
| **Web Server** (máquina virtual) | API en Go y proxy inverso |
| **Worker Server** (máquina virtual) | Workers en Go, asynq y su cola de mensajería en contenedor |
| **Servicio administrado de PostgreSQL** | Toda la persistencia transaccional. Una zona, sin réplicas de lectura |
| **Servicio administrado de almacenamiento de objetos** | Originales, derivados HLS, documentos, miniaturas e imágenes |

**2 vCPU, 2 GiB de RAM y 30 GiB de disco persistente por máquina.** Si el
catálogo del proveedor no ofrece la combinación exacta, se elige la más cercana
y se justifica. La configuración efectiva **permanece fija durante cada corrida**;
cambiarla obliga a reportar los resultados como una configuración distinta.

### Lo prohibido

| No se usa | Por qué importa |
| --- | --- |
| CDN | El multimedia se sirve directo desde el almacenamiento de objetos |
| Plataformas administradas de contenedores (Cloud Run, GKE) | Los componentes van en máquinas virtuales, en contenedores Docker |
| Funciones serverless | Igual |
| Escalado automático | El número de máquinas permanece fijo durante las pruebas |
| Balanceadores de carga | TLS y entrada terminan en el proxy del Web Server |
| Réplicas de aplicación entre máquinas | Una instancia de cada cosa |
| Alta disponibilidad | Ni en las máquinas ni en la base de datos |
| Caché o mensajería administradas | Redis va en contenedor en el Worker Server |

### Red

Red virtual privada con subredes, enrutamiento y reglas de firewall que separen
el acceso público de la comunicación interna. **Web Server es el único punto
público.** Worker Server, la cola y la base administrada no exponen servicios a
Internet. Hay que documentar cómo se administran las máquinas y qué
conectividad saliente necesitan para instalar dependencias.

---

## Qué cambia respecto al plan que teníamos

El plan anterior (`despliegue-gcp.md`, versión del 26 de septiembre) apuntaba a
Cloud Run, Memorystore, Cloud CDN y balanceador con certificado gestionado. Los
cuatro están prohibidos aquí. El detalle de la corrección está en
[`adr/0016`](adr/0016-despliegue-iaas-en-dos-maquinas.md).

| Antes | Ahora | Efecto |
| --- | --- | --- |
| `api` en Cloud Run | `api` en contenedor en **Web Server** | Se reutiliza el Compose y el Caddy que ya funcionan |
| `worker` y `worker-media` en Cloud Run | Contenedores en **Worker Server** | D4 de ADR-0015 queda resuelto por decreto |
| Memorystore con réplica | **Contenedor Redis** en Worker Server | Un salto de red por petición autenticada. Es la primera hipótesis de cuello de botella |
| Cloud SQL con HA regional y PITR | **Zonal, sin réplicas**, copias diarias | Punto único de falla, documentado |
| Balanceador + certificado gestionado | **Caddy con Let's Encrypt** sobre IP estática | Hace falta un nombre: dominio propio o `sslip.io` |
| Cloud CDN + cookie firmada | **URL firmada directa a GCS** | `media-sessions` no cambia. El contrato de la Entrega 1 absorbe el cambio sin tocar nada |
| Conector de VPC / salida directa | Las máquinas **están dentro** de la VPC | Desaparece un riesgo entero del plan anterior |
| Entornos `dev` y `prod` | **Un entorno** | La mitad de factura |
| `/docs` sin ruta (hueco 4 del 26-09) | **Resuelto**: Caddy viaja con nosotros | Deja de ser una decisión pendiente |
| `TRUSTED_PROXY_HOPS` por medir | **0**, porque Caddy sobrescribe `X-Forwarded-For` | Deja de ser una incógnita |
| `REDIS_TLS=true` | **`false`**: no sale de la subred privada | El código queda listo para cuando vuelva Memorystore |

**Lo que no cambia:** el adaptador de GCS con multipart emulado por `compose`
(ADR-0015, D1) sigue siendo el trabajo de código más grande de la entrega, y la
prohibición de claves JSON de cuenta de servicio (D3) sigue intacta.

### Numeración

Los documentos anteriores llaman «Entrega 3» al despliegue en GCP. **Es la
Entrega 2.** Las referencias se corrigen a medida que se tocan los documentos;
donde se lea «Entrega 3» junto a GCP, léase esta.

---

## Trabajo de código que sí hay que hacer

Nada de esto es funcionalidad nueva: es lo que el despliegue y la medición
exigen y hoy no existe.

| # | Qué | Por qué | Bloquea |
| --- | --- | --- | --- |
| 1 | **Adaptador `internal/adapters/gcs`** con multipart emulado por `compose` y firma V4 sin clave descargada | Los objetos se migran al servicio administrado. Es el 50 % de la nota | Todo |
| 2 | **Métricas de cola**: profundidad, antigüedad del trabajo más viejo y tasa de procesamiento | El enunciado las exige como evidencia y hoy **no se exportan**. Hay contadores de trabajos, no de cola | Escenario 2 (10 %) |
| 3 | **Generador de datos sintéticos** a escala: estudiantes, cursos, recursos, inscripciones e intentos | El `seed` actual crea cuatro cuentas de demostración. Los escenarios necesitan cientos | Ambos escenarios (20 %) |
| 4 | **Composiciones por máquina** (`deploy/compose.web.yml`, `deploy/compose.worker.yml`) y *scripts* de arranque | Cada máquina levanta su parte, leyendo secretos con su identidad | Despliegue |
| 5 | **Caddyfile de producción** con TLS automático y el nombre público | HTTPS es requisito del enunciado | Despliegue |
| 6 | **Guiones de carga k6** de los dos escenarios | 20 % de la nota | Escenarios |
| 7 | **Agente de métricas de sistema** en las dos máquinas | Memoria y disco no están en las métricas por defecto de Compute Engine, y el enunciado las pide | Ambos escenarios |

El punto 2 es el que más fácil se pasa por alto: se descubre cuando ya se está
corriendo la prueba y no hay forma de contestar «cómo evolucionó la profundidad
de la cola».

**Y lo que no se hace:** Workload Identity Federation, despliegue desde CI,
tablero de Cloud Monitoring, entorno `prod` y alertas en la nube. Ninguno lo
evalúa un criterio de esta entrega. El razonamiento completo, y el orden en que
se recuperarían si sobra tiempo, está en la sección «Lo que se deja fuera a
propósito» de [`despliegue-gcp.md`](despliegue-gcp.md).

---

## Entregables

| # | Qué | Dónde |
| --- | --- | --- |
| 1 | Plataforma desplegada y operativa | La URL pública |
| 2 | *Release* del código y la configuración | Tag `entrega-2` en el repositorio |
| 3 | **Documento de arquitectura** | `docs/entrega2/`, referenciado desde `README.md` |
| 4 | **Informe de capacidad** | `capacity-planning/pruebas_de_carga_entrega2.md` |
| 5 | Video de sustentación, máximo 20 minutos | Enlazado desde el `README.md` |

Las rutas 3 y 4 **las fija el enunciado**; no se negocian ni se mueven a
`arquitectura/`. Lo que vive en `arquitectura/` es el material de trabajo; lo
que va en esas dos rutas es el entregable.

### El documento de arquitectura debe cubrir

1. **Modelo de componentes** — módulos de la API, workers, cola, base,
   almacenamiento e integraciones, con sus comunicaciones síncronas y
   asíncronas.
2. **Modelo de despliegue** — región, red, subredes, máquinas, contenedores,
   base administrada, almacenamiento, volúmenes y reglas de acceso.
3. **Decisiones y adaptaciones** — ubicación de Redis, migración al
   almacenamiento del proveedor, carga directa y URL firmadas, configuración de
   los workers y **diferencias frente a la arquitectura objetivo**.
4. **Operación y recuperación** — despliegue, migración, verificación,
   reinicio, respaldo y reconstrucción; dónde vive la configuración y cómo se
   manejan los secretos.
5. **Capacidad, costo y limitaciones** — configuración exacta, estimación y
   consumo observado, puntos únicos de falla y qué cambios permitirían
   evolucionar hacia una aplicación elástica.

### El video debe mostrar

- La arquitectura desplegada y su correspondencia con los servicios usados.
- Un recorrido de la funcionalidad de la Entrega 1 **sobre el entorno cloud**.
- Evidencia de: carga directa al almacenamiento, acceso autorizado,
  procesamiento asíncrono, persistencia en la base administrada, **idempotencia
  y un fallo con reintento**.
- Los resultados principales de los dos escenarios, el cuello de botella y las
  propuestas de evolución.

---

## Costos: lo que el enunciado obliga a hacer

No es una recomendación, es parte del enunciado.

- **Registrar** región, tipos de instancia, almacenamiento aprovisionado y horas
  de uso previstas. Estimar volumen de objetos, operaciones y transferencia.
  Presentar los costos **con fecha y supuestos**, y contrastarlos con el consumo
  observado durante las pruebas.
- **Activar presupuesto y alertas** de consumo. Documentar las limitaciones de
  la cuenta que afecten esta configuración.
- **Encender los recursos solo cuando hagan falta.** Detener una instancia no
  elimina todos sus costos: hay que revisar almacenamiento, respaldos y recursos
  de red asociados.
- **Después de registrar las evidencias y cargar la entrega, eliminar la
  instancia de base de datos administrada**, conservando antes los respaldos o
  los datos sintéticos y los *scripts* necesarios para reconstruirla. Documentar
  qué recursos se conservan, sus costos y cómo recrear el entorno para una
  sustentación.
- Si el servicio de bases de datos permite **suspender** una instancia,
  consultar las condiciones del proveedor sobre duración, reactivación y cargos
  que continúan durante la suspensión, e incorporarlas al plan de uso.

Lo último importa más de lo que parece: el equipo docente puede pedir una
**sustentación síncrona** y repetir una prueba. Si los recursos se borraron por
control de costos, es responsabilidad del equipo recrearlos con el
procedimiento documentado. El procedimiento de reconstrucción es un entregable
de facto.

---

## Qué sigue fuera de alcance

- **Frontend.** Si el equipo ya dispone de uno, puede reutilizarlo, pero
  desarrollarlo o ampliarlo no es criterio de evaluación.
- **Funcionalidad nueva de producto.** Las 25 operaciones que siguen en
  `x-estado: planificado` se quedan como están.
- **Microservicios.** La separación en máquinas es del modelo de despliegue; no
  exige partir los módulos de negocio.
- **CDN, autoescalado, alta disponibilidad, redimensionar la infraestructura y
  corregir todos los cuellos de botella encontrados.** El análisis termina en
  caracterizar el límite y proponer la evolución con medición detrás.
