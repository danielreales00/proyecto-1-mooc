# Red privada del entorno: una VPC, una subred, el acceso privado a servicios
# que necesita Cloud SQL, la salida a Internet de las máquinas y las reglas
# que dejan al balanceador como único punto público (ADR-0019, D3).

resource "google_compute_network" "vpc" {
  name                    = var.nombre
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"
}

resource "google_compute_subnetwork" "principal" {
  name          = "${var.nombre}-${var.region}"
  network       = google_compute_network.vpc.id
  region        = var.region
  ip_cidr_range = var.cidr_subred

  # Las máquinas llegan a Cloud Storage, Artifact Registry y Secret Manager por
  # la red de Google, sin pasar por el NAT ni necesitar IP externa.
  private_ip_google_access = true
}

# --- Acceso privado a servicios ---------------------------------------------
# Sin este rango y el peering, Cloud SQL no puede tener IP privada, y el
# enunciado exige que la conexión a la base administrada sea privada.

resource "google_compute_global_address" "servicios" {
  name          = "${var.nombre}-servicios"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  address       = split("/", var.cidr_servicios)[0]
  prefix_length = tonumber(split("/", var.cidr_servicios)[1])
  network       = google_compute_network.vpc.id
}

resource "google_service_networking_connection" "servicios" {
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.servicios.name]
}

# --- Dirección fija de la cola ----------------------------------------------
# REDIS_ADDR y SMTP_ADDR de las plantillas apuntan aquí. Las instancias de los
# grupos nacen y mueren con direcciones efímeras; la cola no puede.

resource "google_compute_address" "cola_interna" {
  name         = "${var.nombre}-cola-interna"
  region       = var.region
  subnetwork   = google_compute_subnetwork.principal.id
  address_type = "INTERNAL"
  address      = var.ip_interna_cola
}

# --- Salida a Internet ------------------------------------------------------
# Ninguna máquina de la aplicación tiene IP externa, y todas necesitan salir:
# imágenes de Docker Hub, paquetes del sistema, el repositorio del agente.

resource "google_compute_router" "salida" {
  name    = "${var.nombre}-router"
  region  = var.region
  network = google_compute_network.vpc.id
}

resource "google_compute_router_nat" "salida" {
  name                               = "${var.nombre}-nat"
  router                             = google_compute_router.salida.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"

  subnetwork {
    name                    = google_compute_subnetwork.principal.id
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

# --- Firewall ---------------------------------------------------------------
# Las reglas apuntan a cuentas de servicio y no a etiquetas: una etiqueta la
# puede poner cualquiera que edite la máquina, la cuenta de servicio no. Cada
# instancia de un grupo hereda la cuenta de su plantilla, así que las reglas
# valen para las que el autoscaler cree mañana.
#
# Cloud SQL no aparece: vive en la red del productor, al otro lado del
# peering, y las reglas de entrada de esta VPC no la gobiernan. Lo que la
# protege es no tener IP pública.

locals {
  # Rangos desde los que llegan el balanceador y las verificaciones de salud de
  # Google. Ninguna otra dirección de Internet alcanza las instancias.
  rangos_google = ["130.211.0.0/22", "35.191.0.0/16"]
}

# El tráfico del balanceador y sus verificaciones entran por el 80 de Caddy. El
# TLS termina en el balanceador; dentro de la VPC el salto es HTTP.
resource "google_compute_firewall" "lb_api" {
  name      = "${var.nombre}-lb-api"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["80"]
  }

  source_ranges           = local.rangos_google
  target_service_accounts = [var.cuenta_web]
}

# Autorreparación del grupo de workers: el /healthz del worker es lo único que
# se publica, y solo para estos rangos.
resource "google_compute_firewall" "salud_worker" {
  name      = "${var.nombre}-salud-worker"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["8081"]
  }

  source_ranges           = local.rangos_google
  target_service_accounts = [var.cuenta_worker]
}

# Redis: cola, sesiones, caché y límites. Lo usan las dos capas.
resource "google_compute_firewall" "redis_interno" {
  name      = "${var.nombre}-redis-interno"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["6379"]
  }

  source_service_accounts = [var.cuenta_web, var.cuenta_worker]
  target_service_accounts = [var.cuenta_cola]
}

# SMTP de Mailpit. Lo envía el worker; la API solo lo tiene configurado.
resource "google_compute_firewall" "smtp_interno" {
  name      = "${var.nombre}-smtp-interno"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["1025"]
  }

  source_service_accounts = [var.cuenta_web, var.cuenta_worker]
  target_service_accounts = [var.cuenta_cola]
}

# Administración: SSH solo a través del túnel de IAP, que autentica con la
# identidad de Google de quien entra. El 22 no queda abierto a Internet.
resource "google_compute_firewall" "ssh_iap" {
  name      = "${var.nombre}-ssh-iap"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  # Rango fijo desde el que IAP reenvía las conexiones.
  source_ranges           = ["35.235.240.0/20"]
  target_service_accounts = [var.cuenta_web, var.cuenta_worker, var.cuenta_cola]
}

# La VPC ya deniega por defecto toda entrada no permitida. Esta regla lo hace
# explícito y lo registra: cualquier intento bloqueado deja rastro.
resource "google_compute_firewall" "denegar_resto" {
  name      = "${var.nombre}-denegar-resto"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  priority  = 65000

  deny {
    protocol = "all"
  }

  source_ranges = ["0.0.0.0/0"]

  log_config {
    metadata = "EXCLUDE_ALL_METADATA"
  }
}
