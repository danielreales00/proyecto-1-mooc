# Cloud Run sale a la VPC con direcciones de la subred (salida directa). Las
# reglas de firewall no pueden filtrar esas conexiones por cuenta de servicio,
# así que esta admite la subred hacia Redis y Mailpit en mooc-cola. La subred
# es privada y solo la usan los servicios de Cloud Run y la cola.
resource "google_compute_firewall" "cloudrun_cola" {
  name      = "mooc-cloudrun-cola"
  network   = module.red.red_id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["6379", "1025"]
  }

  source_ranges           = [var.cidr_subred]
  target_service_accounts = [google_service_account.cola.email]

  log_config {
    metadata = "EXCLUDE_ALL_METADATA"
  }
}
