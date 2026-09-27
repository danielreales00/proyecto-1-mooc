# Red privada del entorno: una VPC, una subred, el acceso privado a servicios
# que necesita Cloud SQL, la salida a Internet del Worker Server y las reglas
# que dejan al Web Server como único punto público (ADR-0016, D1).

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

# --- Direcciones fijas ------------------------------------------------------

# Externa y estática: una efímera cambia al reiniciar la máquina y deja el
# nombre público y el certificado apuntando a otro sitio (ADR-0016, D4).
resource "google_compute_address" "web_externa" {
  name         = "${var.nombre}-web"
  region       = var.region
  address_type = "EXTERNAL"
}

# Internas y fijas: REDIS_ADDR del Web Server apunta a la del Worker Server, y
# no puede cambiar porque se recree la máquina.
resource "google_compute_address" "web_interna" {
  name         = "${var.nombre}-web-interna"
  region       = var.region
  subnetwork   = google_compute_subnetwork.principal.id
  address_type = "INTERNAL"
  address      = var.ip_interna_web
}

resource "google_compute_address" "worker_interna" {
  name         = "${var.nombre}-worker-interna"
  region       = var.region
  subnetwork   = google_compute_subnetwork.principal.id
  address_type = "INTERNAL"
  address      = var.ip_interna_worker
}

# --- Salida a Internet ------------------------------------------------------
# El Worker Server no tiene IP externa y necesita salir: imágenes de Docker
# Hub, firmas de ClamAV, paquetes del sistema. El Web Server tiene IP propia y
# el NAT no lo toca.

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
# puede poner cualquiera que edite la máquina, la cuenta de servicio no.
#
# Cloud SQL no aparece: vive en la red del productor, al otro lado del
# peering, y las reglas de entrada de esta VPC no la gobiernan. Lo que la
# protege es no tener IP pública.

resource "google_compute_firewall" "web_publico" {
  name      = "${var.nombre}-web-publico"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["80", "443"]
  }

  source_ranges           = ["0.0.0.0/0"]
  target_service_accounts = [var.cuenta_web]
}

resource "google_compute_firewall" "redis_interno" {
  name      = "${var.nombre}-redis-interno"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["6379"]
  }

  source_service_accounts = [var.cuenta_web]
  target_service_accounts = [var.cuenta_worker]
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
  target_service_accounts = [var.cuenta_web, var.cuenta_worker]
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
