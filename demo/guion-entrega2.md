# Guion del video. Entrega 2

Despliegue básico en la nube. **Máximo 20 minutos.** Los bloques suman **19**,
así que queda un minuto de margen. Si un bloque se va de tiempo, se recorta
ahí mismo, no se le roba al siguiente.

El enunciado pide que el video muestre cuatro cosas. Cada bloque dice cuál
acredita:

| Lo que se pide | Bloques |
| --- | --- |
| Arquitectura desplegada y su correspondencia con los servicios de GCP | 1, 2 |
| Recorrido de la funcionalidad de la Entrega 1 sobre la nube | 3, 4 |
| Carga directa, acceso autorizado, procesamiento asíncrono, persistencia en la base administrada, idempotencia y un fallo con reintento | 4, 5 |
| Resultados de los dos escenarios, el cuello de botella y la evolución | 6, 7, 8 |

**Se puede mostrar resultados de corridas previas** para los procesos largos,
siempre que los registros completos estén en el repositorio. Lo están, en
`capacity-planning/resultados/`. Las pruebas de carga no se corren en vivo:
duran de 10 a 15 minutos por nivel.

---

## Tiempos y reparto

| # | Bloque | Min | Quién |
| --- | --- | --- | --- |
| 0 | Apertura | 0,5 | |
| 1 | La arquitectura y el mapa a GCP | 2,5 | |
| 2 | Infraestructura como código y operación | 1,5 | |
| 3 | Recorrido funcional sobre la nube | 4 | |
| 4 | Multimedia: carga directa y acceso autorizado | 2 | |
| 5 | Persistencia, idempotencia y fallo con reintento | 2,5 | |
| 6 | Escenario 1: actividad académica | 2,5 | |
| 7 | Escenario 2: multimedia | 2 | |
| 8 | Evolución, costo y cierre | 1,5 | |
| | **Total** | **19** | |

Cuatro personas, unos cinco minutos cada una. Los bloques 3, 4 y 5 van
seguidos y con la misma persona al teclado si se puede, porque Postman
encadena variables entre ellos.

---

## Antes de grabar

### El día anterior

- La base encendida: `cloudsql_encendida = true` en `terraform.tfvars`.
- `make postman-nube` en verde. Si falla, no se graba.
- Revisar que los resultados y las gráficas estén en `capacity-planning/`.

### Media hora antes

```bash
make tf ENTORNO=entrega2 ARGS=plan     # "No changes": se enseña en el bloque 2
make tunel-mailpit                     # Mailpit en localhost:8026 para Postman
curl -s https://35-184-146-250.sslip.io/readyz
```

**Postman de escritorio**, entorno `mooc`:

| Variable | Valor |
| --- | --- |
| `base_url` | `https://35-184-146-250.sslip.io` |
| `mailpit_url` | `http://localhost:8026` (el túnel) |
| `demo_password` | La de Secret Manager, pegada **fuera de cámara** |

```bash
# fuera de cámara: copia la contraseña de las cuentas sintéticas
make gcloud ARGS="secrets versions access latest --secret=seed-password"
```

**La contraseña nunca sale en pantalla.** Se pega en la columna *Current value*
de Postman, que no se sincroniza ni se exporta.

### Pestañas abiertas en el navegador, en este orden

1. Consola de GCP · **Compute Engine → Instancias de VM**
2. **Red de VPC → Firewall**, filtrado por la red `mooc`
3. **SQL → mooc-pg-…** (la pantalla de resumen, con «Conexiones»)
4. **Cloud Storage → Buckets** (`mooc-509602-originals`, `-derived`)
5. **Artifact Registry → mooc**
6. **Secret Manager**
7. **Monitoring → Explorador de métricas**, modo PromQL, con las consultas
   del bloque 5 y del bloque 6 guardadas
8. `https://35-184-146-250.sslip.io/docs`
9. En el repositorio: `capacity-planning/graficas/` y
   `capacity-planning/pruebas_de_carga_entrega2.md`, sección 9

---

## El guion

### 0 · Apertura — 0,5 min

> «Esta es la Entrega 2: la plataforma de la Entrega 1, sin funcionalidad
> nueva, desplegada en Google Cloud sobre dos máquinas virtuales, con la base
> en Cloud SQL y los archivos en Cloud Storage. Vamos a enseñar qué se
> desplegó, que funciona igual que en local, y cuánto aguanta.»

### 1 · La arquitectura y el mapa a GCP — 2,5 min

**Pantalla:** el diagrama de despliegue de `docs/entrega2/informe-entrega-2.md`,
sección 7.

> «Dos máquinas. El Web Server es el único punto público: Caddy termina TLS y
> pasa a la API. El Worker Server no tiene IP externa: ahí viven Redis, los
> dos workers y ClamAV. La base es Cloud SQL, zonal, sin réplicas y solo con IP
> privada. Los archivos, en cuatro buckets.»

Luego, **la consola**, una pestaña por componente, sin detenerse:

1. **Instancias de VM.** `mooc-web` con IP externa, `mooc-worker` sin ella.
   `e2-highcpu-2`: dos vCPU y 2 GiB exactos.
   > «El generador de carga es una tercera máquina, fuera de las dos de la
   > aplicación. Solo existe mientras se mide.» *(Si ya se borró, no se dice.)*
2. **Firewall.** 80 y 443 solo al Web Server, 6379 solo del Web Server al
   Worker, 22 solo desde el rango de IAP, y una regla que deniega y registra
   todo lo demás.
   > «Las reglas apuntan a cuentas de servicio, no a etiquetas. Y la red
   > `default`, que viene con SSH abierto a Internet, la borramos.»
3. **Cloud SQL.** «Dirección IP privada: 10.20.0.3. Pública: ninguna.»
4. **Buckets.** Los cuatro. `badges` es de lectura pública sin listado.
5. **Artifact Registry.** Las imágenes, etiquetadas con el SHA del commit.
6. **Secret Manager.** `database-url` y `seed-password`.
   > «Terraform genera estas claves y las escribe con atributos de solo
   > escritura: no están ni en el estado ni en el repositorio. Cada máquina las
   > lee al arrancar con su propia identidad.»

**Acredita:** arquitectura y correspondencia con los servicios.

### 2 · Infraestructura como código y operación — 1,5 min

**Terminal.**

```bash
make tf ENTORNO=entrega2 ARGS=plan
```

> «Todo lo que acabamos de ver está en Terraform. Plan sin cambios: la nube es
> exactamente lo que dice el repositorio.»

```bash
make ssh MAQUINA=mooc-worker CMD="sudo docker ps --format '{{.Names}} {{.Status}}'"
```

> «Se entra por un túnel de IAP con la identidad de Google, sin abrir el 22.
> Los mismos contenedores de local, con las imágenes del registro.»

**Navegador:** `https://35-184-146-250.sslip.io/metrics` → **404**.

> «Las métricas no son públicas. Las lee el agente de operaciones dentro de
> la máquina y las lleva a Cloud Monitoring.»

### 3 · Recorrido funcional sobre la nube — 4 min

**Postman**, con `base_url` a la nube. Las mismas carpetas de la Entrega 1, más
rápido: aquí lo que se enseña es que **funciona igual en otra infraestructura**.

1. **Identidad:** registro → leer el token en Mailpit (por el túnel) →
   verificar → login. Dos frases.
2. **Autoría:** crear curso, añadir módulo, unidad y recurso, publicar. Luego
   **«INMUTABILIDAD: mutar la versión publicada»** → rechazado.
3. **Catálogo e inscripción:** el curso aparece en el catálogo público, el
   estudiante se inscribe y ve el contenido.
4. **Quiz:** iniciar intento, responder, enviar. **«Reenviar: idempotente»**
   devuelve la misma nota.

> «Es la colección de la Entrega 1, sin cambiar el recorrido. Pasa entera
> contra la nube: 149 aserciones, cero fallos.»

### 4 · Multimedia: carga directa y acceso autorizado — 2 min

**Postman → Carga multimedia.**

1. **Declarar la carga.** Enseña la URL de la parte:
   > «La URL va firmada hacia Cloud Storage: `X-Goog-Signature`. Firmada sin
   > clave descargada: la API le pide la firma a IAM con la identidad de la
   > máquina.»
2. **Subir la parte** directa al bucket. «Los bytes no pasan por la API.»
3. **Completar** → `202`, sin esperar al worker.
4. **Verificación del worker** hasta `ready`.
5. **Consola → bucket `derived`**: la carpeta del asset con las variantes HLS.
6. **Descargar** → `302` a una URL firmada de 15 minutos. Y la misma petición
   **con otro usuario** → rechazada.
7. **EICAR** → termina en `infected`, en cuarentena, **sin derivados**.

**Acredita:** carga directa, acceso autorizado, procesamiento asíncrono.

### 5 · Persistencia, idempotencia y fallo con reintento — 2,5 min

**Al empezar el bloque 3, sin comentarlo**, lanza el trabajo que va a fallar.
Tarda unos 3,5 minutos en llegar a la DLQ:

```bash
make psql-nube
```

```sql
INSERT INTO platform.job_runs (job_key, type, queue, status, payload)
VALUES ('email.send:demo-dlq-nube','email.send','critical','queued',
        '{"template":"inexistente","to":"x@mooc.local"}'::jsonb)
ON CONFLICT (job_key) DO UPDATE SET status='queued', attempt=0,
        created_at = now() - interval '1 hour';
```

Ahora, en el bloque:

1. **Persistencia en Cloud SQL**, en el mismo `psql`:

   ```sql
   SELECT count(*) FROM identity.users;
   SELECT status, count(*) FROM assessment.quiz_attempts GROUP BY 1;
   ```

   > «Esto es Cloud SQL, por la IP privada, desde el Web Server. Los intentos
   > que acabamos de enviar están aquí, junto a los diez mil de las pruebas de
   > carga.»

2. **Idempotencia bajo concurrencia.** Pantalla:
   `capacity-planning/pruebas_de_carga_entrega2.md`, la tabla de integridad de
   §9.1, o directamente `capacity-planning/resultados/e1-L2.json`.

   > «Durante el nivel de 61 peticiones por segundo, en paralelo con la carga,
   > k6 envió el mismo intento dos veces a la vez, 49 veces: siempre una sola
   > calificación. Reenvío con la misma clave de idempotencia: la misma
   > respuesta. Un intento ajeno: rechazado. La clave del quiz: nunca en la
   > respuesta. Cero fallos en las cinco.»

   > «Y aquí, en la base: la nota es una columna del intento, y enviar solo
   > acepta un intento en curso. El segundo envío encuentra el intento cerrado.»

   ```sql
   SELECT status, count(*) FROM assessment.quiz_attempts GROUP BY 1;
   ```

3. **El fallo con reintento:**

   ```bash
   make ssh MAQUINA=mooc-worker CMD="sudo docker logs mooc-worker-1 2>&1 | grep demo-dlq-nube"
   ```

   > «Tres intentos, cada vez más separados: backoff exponencial. Al tercero,
   > a la dead-letter queue.»

4. **Monitoring → PromQL:** `queue_depth{state="archived"}` y
   `jobs_dead_letter_total`.

   > «Y se ve en Cloud Monitoring: un trabajo muerto en la cola `critical`.»

5. **Postman → Administración → «Reencolar»** → `202`.

**Acredita:** persistencia, idempotencia, fallo con reintento.

### 6 · Escenario 1: actividad académica — 2,5 min

**Pantalla:** `capacity-planning/graficas/e1-latencia-por-nivel.png`.

> «k6 desde una tercera máquina, en modelo abierto: la carga no baja cuando el
> servidor se pone lento, así se ve la saturación. 600 estudiantes con cuenta
> propia recorriendo catálogo, contenido, progreso y quizzes.»

> «Hasta 61 peticiones por segundo, p95 de 25 milisegundos, sostenido 15
> minutos. A 91 empieza la degradación: p95 de 272. A 110 se dispara: 8
> segundos. Y ningún error: todo lo que respondió estaba bien.»

**Pantalla:** `e1-L3-serie.png`.

> «Lo interesante: la latencia explota y la CPU **baja**. El trabajo espera en
> algún sitio. La base es `db-g1-small`, de núcleo compartido: consume medio
> núcleo unos minutos y luego Google la limita. Ese es el cuello de botella.»

> «Lo comprobamos cambiando una cosa cada vez. Ampliar el pool de conexiones
> de la API ayudó poco: de 8 a 5,6 segundos. Con un vCPU dedicado en la base,
> la misma carga da 27 milisegundos y ni una iteración perdida. El límite es la
> base, y ahí está la medición que lo demuestra.»

**Pantalla:** `e1-L3-sql-dedicado-serie.png` junto a `e1-L3-serie.png`.

**Monitoring → PromQL**, sobre la ventana de L3:
`rate(cloudsql_googleapis_com:database_cpu_usage_time[1m])`.

> «Y la ráfaga de inicios de sesión, que medimos aparte: argon2 reserva 64 MiB
> por login. A unos dos por segundo el Web Server se queda sin memoria y se
> congela. Es el parámetro de seguridad haciendo su trabajo, en una máquina de
> 2 GiB.»

### 7 · Escenario 2: multimedia — 2 min

**PENDIENTE: se escribe con los resultados.** Lo que tiene que decir:

- Cargas directas a Cloud Storage mientras estudiantes reproducen HLS a
  cadencia real.
- Latencia de autorizar y firmar, tiempo de transferencia, de la carga
  completa a `ready`.
- La cola: profundidad y antigüedad en el tiempo (`e2-M*-cola.png`), y si
  drena.
- Qué limita: FFmpeg, memoria del Worker Server o la firma.

### 8 · Evolución, costo y cierre — 1,5 min

> «Qué cambiaríamos, con la medición que lo respalda: una base con vCPU
> dedicado, porque es lo primero que se satura. Acotar cuántos hashes de
> contraseña corren a la vez, porque la memoria del Web Server es lo que cae en
> una ráfaga de logins. **PENDIENTE_EVOLUCION_E2**.»

> «El costo previsto de la entrega es de unos 31 dólares, con presupuesto y
> alertas activos. La base se apaga entre sesiones con una variable, y la
> reconstrucción está documentada y probada.»

> «Todo está en el repositorio: la infraestructura, los guiones de carga, los
> resultados originales y el documento de arquitectura.»

---

## Si algo falla

| Síntoma | Qué hacer |
| --- | --- |
| Postman no lee el correo | `make tunel-mailpit` otra vez: el túnel se cae si la sesión SSH se corta |
| La API no responde | `curl …/readyz`. Si la base está detenida, `cloudsql_encendida = true` y `apply` |
| El trabajo de la DLQ no llega | Esperar: son 3,5 minutos. Seguir con el bloque y volver al final |
| Un certificado inválido | No grabar. Revisar que el nombre en `DOMINIO` sea el de la IP actual |

## Qué no prometer

- No decir que la plataforma «escala»: la configuración es fija a propósito.
- No afirmar tiempo hasta el primer cuadro ni cortes de reproducción: no se
  midieron con un reproductor.
- No decir que hay correo real: el correo se queda en Mailpit, en la nube.
