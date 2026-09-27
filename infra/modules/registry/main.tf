# Repositorio Docker de las imágenes del backend, en la misma región que las
# máquinas: la descarga no cruza regiones ni pasa por el NAT.
resource "google_artifact_registry_repository" "docker" {
  repository_id = var.nombre
  location      = var.region
  format        = "DOCKER"
}

resource "google_artifact_registry_repository_iam_member" "lectura" {
  for_each = toset(var.lectores)

  repository = google_artifact_registry_repository.docker.name
  location   = var.region
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${each.value}"
}
