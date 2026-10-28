# Nada de esto es secreto: son identificadores. Los secretos van en Secret
# Manager y nunca pasan por aquí (ADR-0016, D7).
project_id         = "mooc-509602"
region             = "us-central1"
zona               = "us-central1-a"
presupuesto_usd    = 50

# cuenta_facturacion, firmantes_desarrollo e integrantes van en
# privado.auto.tfvars, que no se versiona: son datos de personas y de
# facturación. Terraform lo carga solo; la plantilla es
# privado.auto.tfvars.example.

# Red. Rangos propios, sin solaparse con los 10.128.0.0/9 de la red default.
cidr_subred    = "10.10.0.0/24"
cidr_servicios = "10.20.0.0/20"
# Fuera del rango que usaban el Web Server (.10) y el Worker Server (.20): la
# dirección de la cola no se confunde con la de una máquina retirada.
ip_interna_cola = "10.10.0.30"

# Cloud SQL: núcleo compartido y 1,7 GiB. El enunciado deja el tamaño de la
# base al presupuesto (pág. 4), y el presupuesto real es de 50 USD: cuesta la
# mitad que 1 vCPU dedicada y deja margen para repetir corridas. El precio es
# que un núcleo compartido puede ralentizarse sin aviso y no tiene SLA; el
# informe lo registra como limitación del experimento.
cloudsql_tier     = "db-g1-small"
cloudsql_disco_gb = 10

# Para detener la base entre sesiones: false y `make tf ENTORNO=entrega3
# ARGS=apply`. Aquí y no con -var: con -var, el siguiente apply sin él la
# volvería a encender sin que nadie lo note. Se versiona, así el equipo ve en
# git si la base debería estar encendida.
cloudsql_encendida = false

# Grupos en cero, cola detenida y reglas de reenvío borradas: lo que cobra por
# hora se apaga. La IP del balanceador, su certificado y el disco de la cola se
# quedan. Al encenderlo, cada pieza arranca sola.
entorno_encendido = false

# Instancias de los dos grupos: las mismas 2 vCPU completas, 2048 MiB y 30 GiB
# de la Entrega 2, para que la capacidad por instancia sea comparable con la
# línea base. Era la combinación exacta de aquel enunciado. Es el tipo del catálogo e2-highcpu-2: pedirlo como
# e2-custom-2-2048 funciona, pero GCP lo normaliza a este nombre y Terraform ve
# una diferencia en cada plan, y la «corregiría» deteniendo las máquinas. Debian 12 porque el agente de operaciones no corre en
# Container-Optimized OS.
tipo_maquina      = "e2-highcpu-2"
imagen_maquinas   = "debian-cloud/debian-12"
disco_maquinas_gb = 30

# Imagen de las plantillas. Debian limpio mientras se pone en marcha; antes de
# la primera corrida medida, la familia horneada (make hornear-imagen), y ya no
# cambia entre corridas: el arranque es parte de lo que se mide (ADR-0019, D7).
imagen_grupos = "projects/mooc-509602/global/images/mooc-base-d4738e9e7919"

# La cola: Redis y Mailpit. e2-small (2 vCPU compartidas, 2 GiB) basta para el
# volumen de la Entrega 2, que no pasó de 412 comandos por segundo.
tipo_cola     = "e2-small"
disco_cola_gb = 20

# Qué imágenes corren: el SHA que imprimió `make publicar`. Se registra en cada
# corrida de carga.
version_imagenes = "d4738e9e7919"

# Pools por proceso, repartidos para el máximo de los dos grupos contra las 50
# conexiones de db-g1-small (3 reservadas al superusuario, quedan 47):
#
#   3 API × 8 + 3 workers × 2 procesos (worker y worker-media) × 3 = 42
#
# Las 5 restantes cubren el migrate de una API que arranca (pool de pgx por
# defecto, 4 como mucho, y unos segundos) y una consola de psql-nube.
#
# Es menos de lo que tenía cada proceso en la Entrega 2 (API 25, workers 10),
# donde una sola API con pool 10 se saturó antes que con 25 (capacity-planning,
# §6.4). Con una instancia la API tiene ahora 8; con tres, 24 entre todas, casi
# las 25 de la línea base. Es una diferencia declarada de la comparación, no un
# detalle: en el escenario 1 con una sola instancia se espera más espera por el
# pool que en la Entrega 2. Los workers pierden poco: sus trabajos tocan la base
# al reclamar, latir y cerrar, y worker-media procesa de uno en uno.
db_max_conns     = 3
db_max_conns_api = 8

# Multiplica los límites POR IP (login_ip, register, verify_email) durante las
# pruebas de carga: el generador es una sola IP. 1 fuera de las pruebas. Es
# una condición fija de cada corrida y se registra en el informe.
factor_limite_ip = 100

# Generador de carga: solo encendido mientras se mide (capacity-planning/).
# 4 vCPU para que su CPU no pase del 60 % y no sea él quien limite.
generador_encendido = false
tipo_generador      = "e2-standard-4"

# --- Escalado (ADR-0019, D4 y D5) ---------------------------------------------
# Puesta en marcha con una instancia fija por grupo; true cuando el balanceador
# y el arranque estén comprobados y los pools repartidos.
autoescalado = true

# API: en la Entrega 2 una instancia atendió 61-68 peticiones por segundo con
# p95 de 25 ms y 35 % de CPU, y se degradó a 91 por la base, no por la API. Con
# 40 por instancia y objetivo 0,8, el grupo crece al pasar de unas 32 por
# instancia: antes de la degradación y lejos de la cota de la base.
rps_por_instancia_api    = 40
objetivo_balanceador_api = 0.8

# Workers: worker-media transcodifica de uno en uno (bulk=1, concurrencia 1).
# Dos trabajos por instancia, uno en curso y uno esperando: tres trabajos piden
# dos instancias y cinco o más, tres.
trabajos_por_worker = 2

# Periodos de inicialización: lo que tarda una instancia nueva en atender o en
# consumir. Medidos con la imagen horneada mooc-base-86b31a8f89a0 (Sesión B,
# 2026-10-23): de la máquina en marcha a todos los contenedores sanos, el peor
# caso fue 80 s en la API y 100 s en los workers. Margen de ~50 %.
inicializacion_api_s     = 120
inicializacion_workers_s = 150

# Como mucho una instancia retirada cada diez minutos en cada grupo.
ventana_reduccion_s = 600

# La autorreparación no juzga una instancia hasta que pudo arrancar entera: el
# doble del peor arranque horneado, y por encima de los ~180 s del arranque por
# script si alguna vez se vuelve a Debian limpio.
retraso_autorreparacion_s = 300
