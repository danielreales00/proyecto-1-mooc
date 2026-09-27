# Generador de carga: una tercera máquina, fuera de las dos de la aplicación,
# como exige el enunciado. Sin IP externa: sale por Cloud NAT y llega a la URL
# pública igual que un cliente. Se crea solo mientras se mide.

resource "google_service_account" "generador" {
  account_id   = "mooc-generador"
  display_name = "Generador de carga (k6)"
}

# Descargar la imagen de worker-media para generar los videos con el mismo
# FFmpeg, y enviar sus métricas: su CPU es parte de la validez de la corrida.
resource "google_artifact_registry_repository_iam_member" "lectura" {
  repository = var.repositorio
  location   = var.region
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.generador.email}"
}

resource "google_project_iam_member" "metricas" {
  for_each = toset(["roles/logging.logWriter", "roles/monitoring.metricWriter"])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.generador.email}"
}

resource "google_compute_instance" "generador" {
  name                      = "mooc-generador"
  zone                      = var.zona
  machine_type              = var.tipo_maquina
  allow_stopping_for_update = true

  boot_disk {
    initialize_params {
      image = var.imagen
      size  = 30
      type  = "pd-balanced"
    }
  }

  network_interface {
    subnetwork = var.subred_id
  }

  service_account {
    email  = google_service_account.generador.email
    scopes = ["cloud-platform"]
  }

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  metadata = {
    startup-script = var.arranque
    enable-oslogin = "TRUE"
  }

  labels = {
    rol = "generador"
  }

  depends_on = [google_project_iam_member.metricas]
}

# SSH por IAP, como a las otras dos: la regla nace y muere con la máquina.
resource "google_compute_firewall" "ssh_iap" {
  name      = "mooc-generador-ssh-iap"
  network   = var.red_id
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges           = ["35.235.240.0/20"]
  target_service_accounts = [google_service_account.generador.email]
}
