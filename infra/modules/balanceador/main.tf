# Balanceador de aplicaciones externo global (ADR-0018, D3): la única entrada
# pública. Termina TLS con un certificado gestionado por Google para
# <ip>.sslip.io, reparte entre las instancias del grupo de API y deja de
# enviarles tráfico cuando no pasan la verificación de preparación.
#
# Las reglas de reenvío cobran por hora aunque no haya tráfico: con
# `encendido = false` desaparecen, y la IP y el certificado se quedan para que
# la URL pública no cambie.

resource "google_compute_global_address" "lb" {
  name = var.nombre
}

locals {
  # sslip.io resuelve el nombre a la IP codificada en él, así que el
  # certificado se emite sin comprar dominio (ADR-0016, D4, que sigue valiendo).
  nombre_publico = coalesce(var.nombre_publico, "${replace(google_compute_global_address.lb.address, ".", "-")}.sslip.io")
}

# Google lo emite cuando el nombre resuelve a la IP del balanceador y hay una
# regla de reenvío escuchando: tarda de 15 a 60 minutos la primera vez.
resource "google_compute_managed_ssl_certificate" "lb" {
  name = "${var.nombre}-${replace(local.nombre_publico, ".", "-")}"

  managed {
    domains = [local.nombre_publico]
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Preparación, no vida: /readyz comprueba base, Redis y objetos. Una instancia
# que no puede atender sale del reparto, aunque su proceso siga vivo; el grupo
# no la recrea por esto (eso lo decide la verificación de vida del grupo).
resource "google_compute_health_check" "preparacion" {
  name                = "${var.nombre}-preparacion"
  check_interval_sec  = var.salud.intervalo_s
  timeout_sec         = var.salud.timeout_s
  healthy_threshold   = var.salud.umbral_sano
  unhealthy_threshold = var.salud.umbral_enfermo

  http_health_check {
    port_specification = "USE_NAMED_PORT"
    port_name          = var.puerto_nombrado
    request_path       = var.salud.ruta
  }

  log_config {
    enable = true
  }
}

resource "google_compute_backend_service" "api" {
  name                  = "${var.nombre}-api"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  protocol              = "HTTP"
  port_name             = var.puerto_nombrado
  timeout_sec           = var.timeout_s
  health_checks         = [google_compute_health_check.preparacion.id]

  # Al retirar una instancia, las peticiones en curso tienen este margen para
  # terminar; las nuevas ya van a otra.
  connection_draining_timeout_sec = var.drenaje_s

  backend {
    group                 = var.grupo_instancias
    balancing_mode        = "RATE"
    max_rate_per_instance = var.rps_por_instancia
    capacity_scaler       = 1
  }

  # Cada petición registra la instancia que la atendió: es la evidencia del
  # reparto y de la incorporación de instancias nuevas.
  log_config {
    enable      = true
    sample_rate = var.muestreo_registros
  }
}

resource "google_compute_url_map" "https" {
  name            = var.nombre
  default_service = google_compute_backend_service.api.id
}

resource "google_compute_target_https_proxy" "https" {
  name             = var.nombre
  url_map          = google_compute_url_map.https.id
  ssl_certificates = [google_compute_managed_ssl_certificate.lb.id]
}

resource "google_compute_global_forwarding_rule" "https" {
  count = var.encendido ? 1 : 0

  name                  = "${var.nombre}-https"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  ip_address            = google_compute_global_address.lb.id
  port_range            = "443"
  target                = google_compute_target_https_proxy.https.id
}

# El 80 solo redirige al 443: nadie usa la plataforma sin TLS por error.
resource "google_compute_url_map" "redireccion" {
  name = "${var.nombre}-redireccion"

  default_url_redirect {
    https_redirect         = true
    redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
    strip_query            = false
  }
}

resource "google_compute_target_http_proxy" "redireccion" {
  name    = "${var.nombre}-redireccion"
  url_map = google_compute_url_map.redireccion.id
}

resource "google_compute_global_forwarding_rule" "http" {
  count = var.encendido ? 1 : 0

  name                  = "${var.nombre}-http"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  ip_address            = google_compute_global_address.lb.id
  port_range            = "80"
  target                = google_compute_target_http_proxy.redireccion.id
}
