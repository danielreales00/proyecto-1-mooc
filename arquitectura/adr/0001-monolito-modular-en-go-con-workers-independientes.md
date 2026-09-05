# ADR-0001 — Monolito modular en Go con workers independientes

- **Estado:** Aceptado
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
   otro módulo**, nunca tocando sus tablas. Prohibido `authoring` leyendo tablas
   de `assessment`.
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

- `make arch-test`: falla si un paquete bajo `modules/*/domain.go` importa algo
  de `adapters/` o de la stdlib de red.
- `docker compose up --scale api=3 --scale worker=3` y la suite E2E pasa igual.
