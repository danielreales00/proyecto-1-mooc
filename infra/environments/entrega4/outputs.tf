output "numero_proyecto" {
  value = data.google_project.este.number
}

output "url_frontend" {
  description = "La interfaz web."
  value       = google_cloud_run_v2_service.frontend.uri
}

output "url_backend" {
  description = "La API (/api/v1) y su documentación (/docs)."
  value       = google_cloud_run_v2_service.backend.uri
}

output "url_workers" {
  description = "Solo invocable por la suscripción push y Cloud Scheduler."
  value       = google_cloud_run_v2_service.workers.uri
}

output "pubsub" {
  value = {
    topic        = google_pubsub_topic.trabajos.name
    suscripcion  = google_pubsub_subscription.workers.name
    muertos      = google_pubsub_topic.muertos.name
    revision     = google_pubsub_subscription.muertos.name
    max_entregas = var.pubsub_max_entregas
  }
}

output "cola" {
  value = module.cola.nombre
}

output "redis_addr" {
  description = "Redis de sesiones, caché y límites de tasa, alcanzado desde el backend por la VPC."
  value       = local.redis_addr
}

output "cloudsql_instancia" {
  value = module.cloudsql.instancia
}

output "cloudsql_ip_privada" {
  value = module.cloudsql.ip_privada
}

output "buckets" {
  description = "Buckets de Cloud Storage por papel."
  value       = module.gcs.nombres
}

output "registro" {
  value = module.registry.url
}

output "cuentas_de_servicio" {
  value = {
    backend     = google_service_account.web.email
    workers     = google_service_account.worker.email
    frontend    = google_service_account.frontend.email
    pubsub_push = google_service_account.pubsub_push.email
    programador = google_service_account.programador.email
    cola        = google_service_account.cola.email
  }
}
