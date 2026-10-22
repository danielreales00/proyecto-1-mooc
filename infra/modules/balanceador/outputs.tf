output "ip" {
  value = google_compute_global_address.lb.address
}

output "nombre_publico" {
  value = local.nombre_publico
}

output "servicio_backend" {
  value = google_compute_backend_service.api.name
}
