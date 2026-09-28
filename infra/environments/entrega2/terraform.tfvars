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
cidr_subred       = "10.10.0.0/24"
cidr_servicios    = "10.20.0.0/20"
ip_interna_web    = "10.10.0.10"
ip_interna_worker = "10.10.0.20"

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
cloudsql_encendida = true

# Lo mismo para las dos máquinas: detenidas no pagan cómputo, sí disco e IP
# estática. Al encenderlas, el arranque deja todo en pie solo.
maquinas_encendidas = true

# Máquinas: 2 vCPU completas y 2048 MiB, la combinación exacta del enunciado,
# y 30 GiB de disco. Es el tipo del catálogo e2-highcpu-2: pedirlo como
# e2-custom-2-2048 funciona, pero GCP lo normaliza a este nombre y Terraform ve
# una diferencia en cada plan, y la «corregiría» deteniendo las máquinas. Debian 12 porque el agente de operaciones no corre en
# Container-Optimized OS.
tipo_maquina      = "e2-highcpu-2"
imagen_maquinas   = "debian-cloud/debian-12"
disco_maquinas_gb = 30

# Qué imágenes corren: el SHA que imprimió `make publicar`. Se registra en cada
# corrida de carga.
version_imagenes = "ef646a192a82"

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
generador_encendido = true
tipo_generador      = "e2-standard-4"
