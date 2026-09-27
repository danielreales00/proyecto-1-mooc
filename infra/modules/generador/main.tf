# Generador de carga: una tercera máquina, fuera de las dos de la aplicación,
# como exige el enunciado. Se crea solo mientras se mide.

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

    # IP externa propia, efímera, como cualquier cliente de Internet. Por
    # Cloud NAT la primera corrida perdió 813 conexiones: NAT reserva 64
    # puertos por VM y destino, y k6 abre cientos hacia el mismo 443. El NAT
    # no es parte del sistema que se mide. Ninguna regla deja entrar nada
    # salvo SSH por IAP.
    access_config {}
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

# La contraseña de las cuentas sintéticas: k6 inicia sesión con ellas. La lee
# la propia máquina de Secret Manager; no viaja por la línea de órdenes.
resource "google_secret_manager_secret_iam_member" "semilla" {
  secret_id = var.secreto_semilla
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.generador.email}"
}
