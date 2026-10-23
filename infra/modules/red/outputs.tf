output "red_id" {
  value = google_compute_network.vpc.id
}

output "subred_id" {
  value = google_compute_subnetwork.principal.id
}

output "ip_interna_cola" {
  value = google_compute_address.cola_interna.address
}

# Cloud SQL no puede crearse antes de que exista el peering. Exponerlo como
# salida permite al entorno encadenar la dependencia sin depends_on a ciegas.
output "conexion_servicios" {
  value = google_service_networking_connection.servicios.id
}
