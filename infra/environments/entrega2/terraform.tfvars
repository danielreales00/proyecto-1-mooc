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

# Cloud SQL: 1 vCPU dedicada y 3,75 GiB. Los tamaños de núcleo compartido
# (db-f1-micro, db-g1-small) se ralentizan sin aviso y ensuciarían la medición
# de capacidad; la base no debe ser la variable ruidosa del experimento.
cloudsql_tier     = "db-custom-1-3840"
cloudsql_disco_gb = 10
