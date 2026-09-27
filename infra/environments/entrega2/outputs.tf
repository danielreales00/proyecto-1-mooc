output "numero_proyecto" {
  value = data.google_project.este.number
}

output "ip_externa_web" {
  value = module.red.ip_externa_web
}

# Nombre público mientras no haya dominio propio (ADR-0016, D4).
output "nombre_publico" {
  value = "${replace(module.red.ip_externa_web, ".", "-")}.sslip.io"
}

output "redis_addr_web" {
  description = "REDIS_ADDR del Web Server: Redis vive en el Worker Server."
  value       = "${module.red.ip_interna_worker}:6379"
}

output "cloudsql_instancia" {
  value = module.cloudsql.instancia
}

output "cloudsql_ip_privada" {
  value = module.cloudsql.ip_privada
}

output "buckets" {
  description = "Valores de S3_BUCKET_*."
  value       = module.gcs.nombres
}

output "registro" {
  value = module.registry.url
}

output "cuentas_de_servicio" {
  value = {
    web    = google_service_account.web.email
    worker = google_service_account.worker.email
  }
}
