# Nada de esto es secreto: son identificadores. Los secretos van en Secret
# Manager y nunca pasan por aquí (ADR-0016, D7). cuenta_facturacion,
# firmantes_desarrollo e integrantes van en privado.auto.tfvars, que no se
# versiona; la plantilla es privado.auto.tfvars.example.
project_id      = "mooc-509602"
region          = "us-central1"
zona            = "us-central1-a"
presupuesto_usd = 50

# Red. La subred es también la salida directa de Cloud Run a la VPC.
cidr_subred     = "10.10.0.0/24"
cidr_servicios  = "10.20.0.0/20"
ip_interna_cola = "10.10.0.30"

# Cloud SQL, la misma de las entregas anteriores.
cloudsql_tier     = "db-g1-small"
cloudsql_disco_gb = 10

# Para detener la base entre sesiones: false y `make tf ENTORNO=entrega4
# ARGS=apply`. Se versiona, así el equipo ve en git si debería estar encendida.
cloudsql_encendida = true

# La máquina de la cola (Redis de sesiones y Mailpit) y el disparo del reaper.
# Los servicios de Cloud Run escalan a cero solos y no cobran sin peticiones.
entorno_encendido = true

imagen_maquinas = "debian-cloud/debian-12"
tipo_cola       = "e2-small"
disco_cola_gb   = 20

# Qué imágenes corren: el SHA que imprimió `make publicar`.
version_imagenes = "8048683a3abb"

# Pools por proceso contra las 47 conexiones útiles de db-g1-small: con tres
# instancias de cada servicio, 3 × 8 + 3 × 3 = 33, más las de los trabajos de
# migración y semilla.
db_max_conns     = 3
db_max_conns_api = 8

# 1 fuera de las pruebas de carga, que salen de una sola IP.
factor_limite_ip = 1

generador_encendido = false
tipo_generador      = "e2-standard-2"

# --- Cloud Run (ADR-0020) -------------------------------------------------------

max_instancias      = 3
pubsub_max_entregas = 5

# Archivos estáticos: muchas peticiones por instancia. 512 MiB es el mínimo de
# Cloud Run con un vCPU completo.
frontend = {
  cpu             = "1"
  memoria         = "512Mi"
  concurrencia    = 80
  tiempo_maximo_s = 30
}

# Cada inicio de sesión reserva 64 MiB para argon2id. Con 20 peticiones por
# instancia caben 20 inicios simultáneos con margen en 2 GiB; en la Entrega 2
# una ráfaga de inicios agotó la memoria de una máquina de 2 GiB.
backend = {
  cpu             = "1"
  memoria         = "2Gi"
  concurrencia    = 20
  tiempo_maximo_s = 60
}

# Un trabajo por instancia: FFmpeg ocupa los dos núcleos. 4 GiB para clamd
# (cerca de 1 GiB con las firmas) y FFmpeg. 600 s cubren la transcodificación
# más larga medida (196 s) y es el plazo de confirmación de la suscripción.
workers = {
  cpu             = "2"
  memoria         = "4Gi"
  concurrencia    = 1
  tiempo_maximo_s = 600
}
