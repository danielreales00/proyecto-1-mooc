# ADR-0001 — Monolito modular en Go con workers independientes

- **Estado:** Aceptado — **enmendado el 2026-09-07**, ver «Enmienda» al final
- **Fecha:** 2026-09-05
- **Requisitos:** `RT-01`, `CE-01`

## Contexto

El enunciado (§4, §7) impone monolito modular en Go, workers asíncronos
independientes, API y workers sin estado, y dominio desacoplado del framework
HTTP y del proveedor cloud. La decisión pendiente no es *si*, sino *cómo* se
traduce eso a paquetes y binarios.

## Decisión

Un solo módulo Go, dos binarios, un árbol de paquetes por módulo de dominio.

```
backend/
  cmd/api/            binario de la API
  cmd/worker/         binario de los workers
  cmd/migrate/        aplicación de migraciones
  internal/
    platform/         nada de dominio: config, log, otel, httpx, errores, ids
      config/ logging/ observability/ httpx/ problem/ idempotency/
    adapters/         implementaciones de los puertos del dominio
      postgres/ redis/ objectstore/ mailer/ antivirus/ transcoder/ queue/
    modules/          un paquete por módulo de dominio
      identity/ admin/ authoring/ media/ catalog/ enrollment/
      assessment/ progress/ badges/ audit/
  migrations/         SQL numerado, hacia adelante
  openapi/            openapi.yaml
```

Reglas:

1. Cada módulo expone `service.go` (casos de uso), `domain.go` (entidades y
   reglas, sin imports de infraestructura), `store.go` (interfaz de
   persistencia) y `http.go` (handlers).
2. **El dominio no importa `net/http`, ni `pgx`, ni el SDK de S3.** Define
   interfaces; `adapters/` las implementa; `cmd/` las cablea.
3. Los módulos se comunican **por llamada directa a la interfaz de servicio del
   otro módulo**. Ningún módulo **escribe** en el esquema de otro.
   *(Regla enmendada el 2026-09-07; la redacción original prohibía también las
   lecturas. Ver «Enmienda».)*
4. Cuando el acoplamiento sería circular, se rompe con un **evento asíncrono**
   publicado en la cola.
5. Los dos binarios comparten `internal/` completo. El worker no tiene servidor
   HTTP salvo `/healthz` y `/metrics`.
6. Nada de estado en proceso: ni caché local, ni ficheros temporales que deban
   sobrevivir a la petición, ni sesiones en memoria. Los temporales de FFmpeg
   viven dentro de la ejecución de un job y se borran al terminar.

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Microservicios | Escalado independiente, límites duros | Operación mucho más cara para 4 personas y 13 semanas | El enunciado pide monolito modular explícitamente |
| Un solo binario con `MODE=api\|worker` | Una imagen, menos configuración | Un fallo del worker puede tumbar la API; el escalado se confunde | Se pierde la independencia que pide §4 |
| Monolito por capas (`handlers/`, `services/`, `repos/`) | Familiar | El dominio se dispersa; los módulos dejan de tener frontera | No hay forma de comprobar que un módulo no invade a otro |

## Consecuencias

- Se puede extraer un módulo a servicio propio después sin reescribir el dominio.
- Un test de arquitectura en CI puede verificar las reglas 2 y 3 inspeccionando
  imports.
- Cuesta disciplina: es más rápido, a corto plazo, leer la tabla del vecino.
- Las dos imágenes se construyen del mismo `Dockerfile` multi-stage con dos
  targets, así que comparten capas.

## Cómo se verifica

- `make arch` (`scripts/arquitectura.sh`): falla si un `domain.go` importa
  infraestructura, si un módulo **escribe** en el esquema de otro fuera de las
  excepciones documentadas, si entra una dependencia que no está en el
  ADR-0002, o si aparece código específico de un proveedor cloud dentro de los
  módulos. Corre en CI.
- `docker compose up --scale api=3 --scale worker=3` y la suite E2E pasa igual.

## Enmienda · 2026-09-07

La regla 3 decía que un módulo no podía **tocar** las tablas de otro. Al
escribir la comprobación que este mismo ADR prometía, resultó que el código la
incumplía en **39 sitios**: 5 escrituras y 34 lecturas.

Merecía la pena mirar cada caso antes de decidir si arreglar el código o la
regla.

**Las escrituras sí eran el problema.** `assessment` escribía directamente en
`progress.resource_progress` para marcar completado el recurso de un quiz
aprobado, saltándose las reglas de `learning` sobre qué cuenta como completado
y su recálculo del porcentaje. Eso se corrigió: ahora `assessment` declara un
puerto `Progreso` y le pide a `learning` que lo marque; la decisión sigue
viviendo donde vive la regla.

**Las lecturas no lo eran.** En un monolito con una sola base, un `JOIN` en la
capa de adaptadores es más simple, más rápido y más fácil de leer que reunir en
memoria lo que la base sabe reunir. Prohibirlas obligaría a N+1 consultas y a
un mapa de identificadores en cada listado, a cambio de una pureza que las
otras dos reglas —el dominio no importa infraestructura, y la frontera está en
los esquemas— ya sostienen.

La regla queda así:

| | Permitido |
| --- | --- |
| Leer el esquema de otro módulo desde un adaptador | **Sí**, con un `JOIN` explícito y legible |
| Escribir en el esquema de otro módulo | **No**, salvo las excepciones de abajo |
| Que el dominio importe infraestructura | **No** |

### Excepciones documentadas

`admin` escribe en `identity.users`, `identity.sessions` y
`identity.one_time_tokens`. No es una fuga: **`admin` e `identity` son el mismo
contexto con dos caras** —la del propio usuario y la administrativa— y comparten
el agregado. Separarlos obligaría a `identity` a exponer una interfaz de
administración que solo usaría `admin`, sin ganar frontera real.

Está enumerada en `scripts/arquitectura.sh` para que sea una excepción
consciente y no un descuido que se propaga.

### Lo que esto cuesta al extraer un módulo

Las 34 lecturas cruzadas son el precio: extraer `authoring` a un servicio
propio obligaría a sustituirlas por llamadas. El script las cuenta y las
imprime justamente para que ese precio esté a la vista y no se descubra el día
de la extracción.
