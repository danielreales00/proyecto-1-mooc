output "numero_proyecto" {
  value = data.google_project.este.number
}

output "ip_balanceador" {
  value = module.balanceador.ip
}

# URL del frontend y de la API: el mismo origen, detrás del balanceador.
output "nombre_publico" {
  value = local.nombre_publico
}

output "url_publica" {
  value = "https://${local.nombre_publico}"
}

output "grupos" {
  value = {
    api     = module.api.nombre
    workers = module.workers.nombre
  }
}

output "plantillas" {
  value = {
    api     = module.api.plantilla
    workers = module.workers.plantilla
  }
}

output "cola" {
  value = module.cola.nombre
}

output "redis_addr" {
  description = "REDIS_ADDR de los dos grupos."
  value       = local.redis_addr
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
    cola   = google_service_account.cola.email
  }
}
