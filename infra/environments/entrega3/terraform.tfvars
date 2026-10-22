# Nada de esto es secreto: son identificadores. Los secretos van en Secret
# Manager y nunca pasan por aquí (ADR-0016, D7).
project_id         = "mooc-509602"
region             = "us-central1"
zona               = "us-central1-a"
cuenta_facturacion = "01D669-827C63-9BFAA3"
presupuesto_usd    = 50

# Quién puede firmar como mooc-web desde su máquina (make prueba-gcs). Cada
# persona del equipo que trabaje en el adaptador añade aquí su cuenta.
firmantes_desarrollo = ["user:santiago.chica1997@gmail.com"]

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

# Para detener la base entre sesiones: false y `make tf ENTORNO=entrega2
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
# cambia entre corridas: el arranque es parte de lo que se mide (ADR-0018, D7).
imagen_grupos = "debian-cloud/debian-12"

# La cola: Redis y Mailpit. e2-small (2 vCPU compartidas, 2 GiB) basta para el
# volumen de la Entrega 2, que no pasó de 412 comandos por segundo.
tipo_cola     = "e2-small"
disco_cola_gb = 20

# Qué imágenes corren: el SHA que imprimió `make publicar`. Se registra en cada
# corrida de carga.
version_imagenes = "ef646a192a82"

# PENDIENTE (frente de backend, punto 3 del alcance): con tres instancias por
# grupo estos pools piden 3 × 25 + 3 × 2 × 10 = 135 conexiones contra las ~47 de
# db-g1-small. No se activa el autoescalado hasta fijar el reparto aquí.
#
# Pools por proceso contra las 50 conexiones de db-g1-small (3 reservadas).
# La API empezó en 10: en el escenario 1 la latencia se disparó con el pool
# lleno y la CPU a la mitad (capacity-planning/, §9.1). Se mide el antes y el
# después por separado.
db_max_conns     = 10
db_max_conns_api = 25

# Multiplica los límites POR IP (login_ip, register, verify_email) durante las
# pruebas de carga: el generador es una sola IP. 1 fuera de las pruebas. Es
# una condición fija de cada corrida y se registra en el informe.
factor_limite_ip = 100

# Generador de carga: solo encendido mientras se mide (capacity-planning/).
# 4 vCPU para que su CPU no pase del 60 % y no sea él quien limite.
generador_encendido = false
tipo_generador      = "e2-standard-4"

# --- Escalado (ADR-0018, D4 y D5) ---------------------------------------------
# Puesta en marcha con una instancia fija por grupo; true cuando el balanceador
# y el arranque estén comprobados y los pools repartidos.
autoescalado = false

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
# consumir con el arranque por script. Se reemplazan por los medidos con la
# imagen horneada.
inicializacion_api_s     = 180
inicializacion_workers_s = 240

# Como mucho una instancia retirada cada diez minutos en cada grupo.
ventana_reduccion_s = 600

# La autorreparación no juzga una instancia hasta que pudo arrancar entera.
retraso_autorreparacion_s = 600
