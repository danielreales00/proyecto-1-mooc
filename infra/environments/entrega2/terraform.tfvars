# Nada de esto es secreto: son identificadores. Los secretos van en Secret
# Manager y nunca pasan por aquí (ADR-0016, D7).
project_id         = "mooc-509602"
region             = "us-central1"
zona               = "us-central1-a"
cuenta_facturacion = "01D669-827C63-9BFAA3"
presupuesto_usd    = 50

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
