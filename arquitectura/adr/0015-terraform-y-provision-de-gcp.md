# ADR-0015 — Terraform y provisión de GCP

- **Estado:** **Parcialmente aceptado** — D2 y D3 aceptados el 2026-09-05.
  D1 y D4–D7 siguen Propuestos.
- **Fecha:** 2026-09-05
- **Requisitos:** `RT-01`, `RT-06`, `CE-01`
- **Refina:** ADR-0010 (portabilidad a GCP). Puede reemplazar parte de ADR-0005.

## Contexto

ADR-0010 difirió la infraestructura a la Entrega 3 y esa parte sigue en pie: la
Entrega 1 se entrega con Docker Compose y hoy no hay nada que aprovisionar.
Escribir HCL ahora no acredita ningún criterio de esta entrega.

Pero diferir *el código de infraestructura* no es lo mismo que diferir *las
decisiones*. Dos de ellas condicionan lo que se escribe esta semana:

1. Cómo se entrega el HLS detrás de un CDN. Cambia el **contrato de la API**, y
   el OpenAPI está a punto de congelarse (`pendientes.md`, tarea 1).
2. Cómo se autentica el backend contra el almacén de objetos en GCP. Determina
   si el adaptador S3 del ADR-0005 sobrevive o hay que escribir uno nativo.

Las demás decisiones de GCP no tocan el código y pueden esperar sin coste. Este
ADR separa unas de otras, para no pagar por adelantado lo que no hace falta.

## Decisión

### Cuándo

**Terraform se escribe en la Entrega 3, no antes.** Lo que se decide ahora es
solo aquello cuyo aplazamiento obligaría a reescribir código o contratos.

### D1 — Almacenamiento de objetos en GCP: SDK nativo, no interoperabilidad S3 · Propuesto

ADR-0005 apostó por hablar el protocolo S3 para tener un solo adaptador en local
(MinIO) y en la nube (GCS por su endpoint XML). El problema es la credencial: el
endpoint de interoperabilidad de GCS exige **claves HMAC**, que son un secreto
de larga vida asociado a una cuenta de servicio. Eso choca de frente con D3.

Se propone **mantener el adaptador S3 para MinIO** y **añadir un adaptador GCS
nativo** para la nube, ambos detrás del mismo puerto `objectstore.Store`. El
puerto ya existe y el dominio no se entera: es exactamente el escenario para el
que se diseñó (ADR-0001, ADR-0010).

Coste: un adaptador más, unas 200 líneas, en la Entrega 3.
Beneficio: cero credenciales de larga vida, y firmas de URL con la identidad de
la carga de trabajo.

### D2 — Entrega de HLS: cookie firmada de CDN, con endpoint propio · **ACEPTADO**

El diseño actual (`disenos/api-v1.md`) firma **cada segmento** por separado.
Detrás de Cloud CDN eso no funciona bien: un video de 40 minutos son ~400
segmentos, cada uno pidiendo una firma a la API, que deja de ser un CDN para
convertirse en un cuello de botella.

Se propone **cookie firmada de Cloud CDN**, acotada por prefijo de ruta, y un
endpoint nuevo que la emita tras verificar el derecho de acceso:

```
POST /api/v1/enrollments/{id}/media-sessions
  -> 201, Set-Cookie: Cloud-CDN-Cookie=...; Path=/media/{course_version_id}/; ...
  -> { "expires_at": "...", "manifest_url": "https://cdn.../master.m3u8" }
```

Esto **debe entrar en el OpenAPI ahora**, aunque su implementación en la Entrega
1 sea una URL firmada de MinIO en lugar de una cookie de CDN. El contrato es el
mismo; cambia quién emite la credencial.

En local, la Entrega 1 puede seguir firmando por segmento sin coste: son pocos
recursos y no hay CDN. Lo que no se puede es **descubrir en la Entrega 3 que el
contrato era otro**.

### D3 — Autenticación: Workload Identity Federation, sin claves JSON · **ACEPTADO**

**No se crean claves JSON de cuenta de servicio. Nunca, ni para probar.**

- CI hacia GCP: **Workload Identity Federation** con el OIDC de GitHub Actions.
- Cargas en Cloud Run: la **identidad de la propia carga**, sin credencial
  explícita.
- El código sigue leyendo solo `os.Getenv` (ADR-0010, regla 4); las bibliotecas
  de GCP resuelven la credencial del entorno.

Una clave creada "temporalmente" acaba en el repositorio. La forma de que eso no
pase es que no exista ninguna.

### D4 — Cómputo

| Componente | Propuesta | Nota |
| --- | --- | --- |
| `api` | Cloud Run, `min-instances: 1` | El arranque en frío se pagaría en el p95 |
| `worker` (colas `critical`, `default`) | Cloud Run | Trabajos cortos, ligados a E/S |
| `worker-media` (cola `bulk`) | **Se decide en la Entrega 3** | Si la transcodificación no cabe en Cloud Run, va a un grupo de instancias gestionado. **No afecta al código**: los workers tiran de la cola, no reciben peticiones |
| `migrate`, `seed` | Cloud Run jobs | Ya están separados (ADR-0003) |

D4 es la razón de que este ADR no bloquee nada más: el cómputo se puede decidir
tarde porque el código no cambia.

### D5 — Estructura de Terraform

```
infra/
  modules/            red · cloudsql · memorystore · gcs · cloudrun · cdn · wif
  environments/
    dev/              main.tf · variables.tf · terraform.tfvars
    prod/
  versions.tf         proveedores con versión fijada
```

- **Estado remoto en un bucket de GCS**, con bloqueo, uno por entorno. El bucket
  se crea a mano una vez; es la única excepción a "todo por Terraform", porque
  Terraform no puede crear el sitio donde guarda su propio estado.
- Versiones de proveedor **fijadas**, como las dependencias de Go (ADR-0002).
- `terraform plan` en cada PR que toque `infra/`; `apply` solo desde `main`.
- Terraform **no gestiona secretos**: crea los contenedores de Secret Manager,
  y los valores se cargan aparte. Un secreto en el estado de Terraform es un
  secreto en texto plano en un bucket.

### D6 — Datos

- **Cloud SQL para PostgreSQL 17**, alta disponibilidad regional, copias
  automáticas y recuperación a un punto en el tiempo (`RNF-03`, `RNF-04`).
- **Memorystore for Redis** con réplica.
- **Pool de conexiones:** Cloud Run escala a muchas instancias y cada una abre
  su pool. Con `MaxConns=10` y 20 instancias son 200 conexiones, por encima de
  lo que admiten las instancias pequeñas de Cloud SQL. Se ajusta por variable de
  entorno, o se pone un intermediario. **No bloquea nada hoy.**

## Decisiones que exige este ADR

Lo que el equipo debe resolver, y cuándo.

| # | Decisión | Opciones | Resultado | Qué bloquea | Cuándo |
| --- | --- | --- | --- | --- | --- |
| **D2** | Entrega de HLS | Firma por segmento · Cookie firmada de CDN | **✅ Aceptado 2026-09-05: cookie firmada.** El endpoint `media-sessions` ya está en el OpenAPI | El OpenAPI y los módulos `media` y `enrollment` | Resuelto |
| **D3** | Credenciales hacia GCP | Claves JSON · Workload Identity Federation | **✅ Aceptado 2026-09-05: WIF. Prohibidas las claves JSON de cuenta de servicio** | La higiene del repositorio | Resuelto |
| **D1** | Cliente de objetos en GCP | Interoperabilidad S3 (claves HMAC) · **Adaptador GCS nativo** | Propuesto. **D3 lo implica casi por completo**: las claves HMAC son también una credencial estática de larga vida | El puerto `objectstore` y, con él, ADR-0005 | Antes de la Entrega 3 |
| **D4** | Cómputo de `worker-media` | Cloud Run · Grupo de instancias gestionado | Decidir en la Entrega 3, midiendo | Nada | Entrega 3 |
| **D5** | Estructura de Terraform y estado remoto | Módulos + entornos · Un solo directorio | Módulos + entornos, estado en GCS | Nada | Entrega 3 |
| **D6** | Tamaño de Cloud SQL y pool | Ajustar `MaxConns` · Intermediario de conexiones | Ajustar por variable; medir antes | Nada | Entrega 3 |
| **D7** | Entornos | Solo `prod` · `dev` + `prod` | `dev` + `prod`: sin un sitio donde equivocarse, se prueba en producción | Coste en GCP | Entrega 3 |

**D2 y D3 quedaron resueltas.** Las demás están aquí para que no se decidan por
omisión; ninguna bloquea la Entrega 1.

### Consecuencia abierta de D2: alcance de la credencial

Aceptar la cookie firmada deja una sub-decisión que **no afecta al contrato de
la API** —la respuesta es la misma en ambos casos— pero sí al diseño de claves
de objeto:

- **Por asset** (`derived/{asset_id}/…`, como hoy en ADR-0005): una credencial
  por video. Simple, y no duplica derivados cuando una versión nueva del curso
  reutiliza el mismo asset (ADR-0007).
- **Por versión de curso**: una sola credencial para todo el curso, menos idas y
  vueltas. Pero obligaría a duplicar los derivados HLS por versión, que es justo
  lo que ADR-0007 evita.

La respuesta de `media-sessions` incluye el campo `scope` precisamente para que
esto se pueda cambiar sin romper al cliente. Se decide al implementar `media`.

## Estado de la implementación

| Decisión | Qué se hizo al aceptarla |
| --- | --- |
| D2 | `POST /api/v1/enrollments/{enrollmentId}/media-sessions` añadido a `backend/openapi/openapi.yaml` y a `disenos/api-v1.md`. En la Entrega 1 devolverá `delivery: "signed_url"` con una URL firmada de MinIO; en la Entrega 3 pasará a `delivery: "signed_cookie"` **sin cambiar el contrato** |
| D3 | Regla registrada en `CLAUDE.md`. El CI ya falla si aparece una credencial en el repositorio |

## Alternativas consideradas

| Opción | A favor | En contra | Por qué no |
| --- | --- | --- | --- |
| Escribir Terraform ya, en la Entrega 1 | Adelanta trabajo de la Entrega 3 | No hay nada que aprovisionar; no acredita ningún criterio de esta entrega; el diseño cambiaría antes de usarse | Sería trabajo desechable |
| No decidir nada de GCP hasta la Entrega 3 | Foco total en la entrega actual | D2 obliga a rehacer el contrato de la API y la colección de Postman | Dos decisiones sí tienen coste diferido |
| `gcloud` y consola en vez de Terraform | Rápido para empezar | Nada reproducible; el criterio de arquitectura evalúa despliegue repetible | Contradice `CE-01` |
| Pulumi o Terraform CDK | El equipo escribiría Go | Menos material de referencia; el estado se complica igual | Terraform es lo estándar y lo que se espera de un curso cloud |
| Mantener la interoperabilidad S3 en GCP | Un solo adaptador | Obliga a claves HMAC de larga vida, que es justo lo que D3 quiere evitar | La credencial pesa más que el adaptador |

## Consecuencias

- La Entrega 1 no cambia de alcance: se sigue entregando con Docker Compose.
- El OpenAPI incorpora `media-sessions` antes de congelarse. En la Entrega 1 ese
  endpoint devuelve una URL firmada de MinIO; en la Entrega 3 emite la cookie.
  **El cliente no se entera del cambio**, que es el objetivo.
- ADR-0005 queda matizado: el puerto se mantiene, la implementación única no.
  Cuando D1 se acepte, se añade la nota de reemplazo parcial allí.
- Habrá dos adaptadores de objetos que probar. El de MinIO ya está; el de GCS se
  prueba contra GCS real en la Entrega 3.
- Prohibir las claves JSON obliga a montar WIF antes del primer despliegue, lo
  que hace la primera subida más lenta y todas las siguientes más seguras.

## Cómo se verifica

- `grep -rn "credentials.json\|service_account.*key" .` no devuelve nada, y el
  CI falla si aparece.
- El OpenAPI de la Entrega 1 incluye `POST /enrollments/{id}/media-sessions`.
- En la Entrega 3: `terraform plan` sin cambios pendientes tras un `apply`, y el
  despliegue desde CI sin ninguna credencial almacenada en GitHub.
