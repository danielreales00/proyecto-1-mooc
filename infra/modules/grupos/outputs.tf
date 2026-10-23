output "nombre" {
  value = google_compute_instance_group_manager.grupo.name
}

# El grupo de instancias, para el servicio de backend del balanceador.
output "grupo_instancias" {
  value = google_compute_instance_group_manager.grupo.instance_group
}

output "plantilla" {
  value = google_compute_instance_template.plantilla.name
}
