output "url" {
  description = "Prefijo de las imágenes: <url>/<imagen>:<sha>."
  value       = "${var.region}-docker.pkg.dev/${google_artifact_registry_repository.docker.project}/${google_artifact_registry_repository.docker.repository_id}"
}
