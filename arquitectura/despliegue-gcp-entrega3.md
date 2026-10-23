# Despliegue en GCP — plan ejecutable de la Entrega 3

Esto es el plan de trabajo del despliegue elástico, no un diseño.

- **Qué exige la entrega:** [`alcance-entrega-3.md`](alcance-entrega-3.md)
- **Qué se decidió y por qué:** [`adr/0019`](adr/0019-arquitectura-elastica-iaas.md),
  que corrige el [`adr/0016`](adr/0016-despliegue-iaas-en-dos-maquinas.md)
- **El plan de la Entrega 2**, para la línea base: [`despliegue-gcp.md`](despliegue-gcp.md)

Todo se opera desde contenedores, como en la Entrega 2: `make tf`, `make
gcloud`, `make ssh`. En Windows con Git Bash, anteponer `MSYS_NO_PATHCONV=1`.

---

## La forma del despliegue

```
Internet ─443─► Balanceador externo global (certificado gestionado, <ip>.sslip.io; 80 → 443)
                    │ servicio de backend, modo RATE, preparación /readyz
          ┌─────────▼──────────┐          ┌──────────────────────┐
          │ MIG mooc-api 1..3  │          │ MIG mooc-worker 1..3 │  señal: cola_bulk_trabajos
          │ e2-highcpu-2       │          │ e2-highcpu-2         │  (pendientes + en curso)
          │ caddy:80 api       │          │ worker worker-media  │
          │ frontend swagger   │          │ clamav               │
          └───┬───────────┬────┘          └───┬───────────┬──────┘
              │ sin IP externa; salida por Cloud NAT      │
          ┌───▼───────────▼───────────────────▼──┐   ┌────▼──────────┐
          │ VM mooc-cola e2-small 10.10.0.30     │   │ Cloud SQL     │
          │ redis (AOF) · mailpit · publicador   │   │ Cloud Storage │
          └──────────────────────────────────────┘   └───────────────┘
```

| Archivo | Qué es |
| --- | --- |
| `infra/environments/entrega3/` | El entorno. `grupos.tf`, `cola.tf`, `balanceador.tf` y `tablero.tf` son lo nuevo |
| `infra/modules/grupos/` | Plantilla, MIG, verificación de vida y autoscaler. Se usa dos veces |
| `infra/modules/balanceador/` | IP, certificado, servicio de backend, verificación de preparación y reglas |
| `infra/modules/cola/` | La máquina de Redis y Mailpit |
| `deploy/arranque.sh` | Arranque de todas las máquinas, por papel: `web`, `worker`, `cola`, `hornear` |
| `deploy/compose.{web,worker,cola}.yml` | Qué corre en cada papel |
| `deploy/publicar-cola.sh` | La señal del autoscaler de workers |
| `deploy/apagado-worker.sh` | El retiro ordenado de un worker |
| `scripts/hornear-imagen.sh` | La imagen base de las plantillas |

---

## Fase 0 — Verificaciones previas

Tres comprobaciones que el ADR-0019 (D9) deja abiertas. Se hacen al principio
porque cambian la configuración si fallan.

1. **Métrica de grupo en el autoscaler.** Con la cola encendida, comprobar que
   llega la serie: `make gcloud ARGS="monitoring time-series list ..."` o el
   explorador de métricas, `custom.googleapis.com/mooc/cola_bulk_trabajos`. El
   `apply` de la fase 4 falla si el autoscaler no acepta el filtro
   `resource.type = "global"`.
2. **Margen del apagado.** Retirar un worker en mitad de una transcodificación
   larga y mirar en su puerto serie si `apagado-worker.sh` terminó o lo cortó
   GCE.
3. **Certificado gestionado.** Tras la fase 3,
   `make gcloud ARGS="compute ssl-certificates list"` debe pasar a `ACTIVE`.

## Fase 1 — Estado: de `entrega2` a `entrega3`

Una sola vez, y nunca más. **Con el entorno de la Entrega 2 detenido** y nadie
más aplicando:

```bash
make gcloud ARGS="storage cp gs://mooc-tfstate-mooc-509602/entrega2/default.tfstate \
  gs://mooc-tfstate-mooc-509602/entrega3/default.tfstate"
make tf ENTORNO=entrega3 ARGS=init
make tf ENTORNO=entrega3 ARGS=plan
```

**El plan se lee entero antes de aplicar.** Lo esperado:

- Destruye `module.maquinas` (mooc-web, mooc-worker), las direcciones
  `mooc-web`, `mooc-web-interna`, `mooc-worker-interna` y la regla
  `mooc-web-publico`.
- Crea la cola, los dos grupos, el balanceador, el tablero y las reglas nuevas.
- Cambia **en sitio** el CORS de los buckets de `module.gcs`: el origen pasa
  del nombre del Web Server al del balanceador. Nada más de los buckets.
- **No toca** `module.cloudsql`, `module.secretos`, `module.registry` ni el
  presupuesto, y no destruye ni recrea ningún bucket. Si aparece cualquiera de
  esas cosas, se para.

Desde aquí `make tf ENTORNO=entrega2` se niega a correr. La configuración de la
línea base sigue en el tag `entrega-2`.

## Fase 2 — Puesta en marcha con una instancia por grupo

En `terraform.tfvars`: `cloudsql_encendida = true`, `entorno_encendido = true`,
`autoescalado = false`, `version_imagenes` con el SHA de `make publicar`.

```bash
make publicar                         # si el SHA cambió
make tf ENTORNO=entrega3 ARGS=apply
make tf ENTORNO=entrega3 ARGS="output url_publica"
```

Copiar el nombre público a `NUBE_HOST` del `Makefile` y al `base_url` de la
colección de Postman.

**El certificado tarda de 15 a 60 minutos.** Mientras tanto el 443 responde con
error de TLS; no es un fallo de las instancias. El 80 ya redirige.

Comprobaciones:

```bash
make instancias                       # mooc-cola, una mooc-api-*, una mooc-worker-*
make ssh MAQUINA=mooc-cola CMD="sudo journalctl -u mooc-publicar-cola -n 5"
make ssh MAQUINA=<mooc-api-xxxx> CMD="sudo journalctl -u google-startup-scripts | grep mooc:"
curl -sI https://<nombre>/healthz     # 200, X-Instancia: mooc-api-xxxx
make semilla-nube                     # cuentas sintéticas
make postman-nube                     # la colección entera por el balanceador
```

La línea `mooc: web levantado en N s` del diario es el tiempo de arranque del
script. El tiempo desde la creación de la instancia sale de su `creationTimestamp`.

## Fase 3 — Imagen horneada

Antes de la primera corrida medida, y otra vez cada vez que cambie
`version_imagenes`:

```bash
make hornear-imagen
```

Imprime el valor para `imagen_grupos`. Con él en `terraform.tfvars`, `apply`:
los grupos reemplazan sus instancias una a una con la plantilla nueva.

Medir el arranque con la imagen (tres instancias nuevas por grupo, con
`make gcloud ARGS="compute instance-groups managed recreate-instances ..."`) y
llevar los tiempos a `inicializacion_api_s`, `inicializacion_workers_s` y
`retraso_autorreparacion_s`.

## Fase 4 — Autoscalers

**Requisito:** los pools de conexiones repartidos para tres instancias por
grupo (punto 3 del alcance). Sin eso, el tercer worker o la tercera API agotan
las conexiones de `db-g1-small`.

`autoescalado = true` y `apply`. Comprobar:

```bash
make gcloud ARGS="compute instance-groups managed describe mooc-api --zone us-central1-a"
make gcloud ARGS="compute autoscalers list"
```

Ciclo mínimo de cada grupo, antes de las corridas medidas:

- **API:** carga por el balanceador por encima de 32 peticiones/s; el grupo
  crea una instancia, aparece sana en el servicio de backend y responde
  (`X-Instancia` nuevo). Se quita la carga; a los 10 minutos largos, retira una.
- **Workers:** subir varios videos del perfil C; `cola_bulk_trabajos` pasa de
  2, el grupo crea una instancia y su `worker-media` toma un trabajo. Cuando la
  señal baja, retira una sin cortar ninguna transcodificación.

## Variables por papel

Lo común a los dos grupos está en `local.env_comun` de `grupos.tf`. Lo propio:

| Variable | API | Workers | Cola |
| --- | --- | --- | --- |
| `REDIS_ADDR` | `10.10.0.30:6379` | `10.10.0.30:6379` | — |
| `SMTP_ADDR` | `10.10.0.30:1025` (no envía) | `10.10.0.30:1025` | — |
| `DB_MAX_CONNS` | `db_max_conns_api` | `db_max_conns` | — |
| `TRUSTED_PROXY_HOPS` | `1` | — | — |
| `INSTANCIA` | nombre de la máquina, para `X-Instancia` | ídem | ídem |
| `DATABASE_URL` | Secret Manager | Secret Manager | — |
| `SMTP_PASSWORD` | — | Secret Manager | — |

## Apagar entre sesiones

En `terraform.tfvars`: `entorno_encendido = false` y `cloudsql_encendida =
false`, y `apply`. Los grupos quedan en cero, la cola detenida y las reglas de
reenvío borradas. La IP, el certificado y el disco de la cola se conservan.

## Reconstrucción para la sustentación

Desde cero, en el mismo proyecto:

1. `make tf ENTORNO=entrega3 ARGS=init` y `apply` con `entorno_encendido = true`
   y `autoescalado = false`.
2. Esperar el certificado.
3. `make semilla-nube`.
4. `make hornear-imagen` si la imagen no existe, `imagen_grupos` y `apply`.
5. `autoescalado = true` y `apply`.

## Retiro de recursos

Tras conservar evidencias y respaldos: `proteger_borrado = false`, `apply`, y
`destroy`. Las imágenes horneadas no las gestiona Terraform: se borran con
`make gcloud ARGS="compute images delete mooc-base-<sha>"`.

## Lo que se deja fuera a propósito

- **Alta disponibilidad:** grupos zonales, una sola cola, Cloud SQL zonal. El
  enunciado no la pide.
- **Despliegue continuo:** una versión nueva es `make publicar`, cambiar
  `version_imagenes` y `apply`.
- **CDN:** HLS sigue saliendo directo de Cloud Storage con URL firmada.
