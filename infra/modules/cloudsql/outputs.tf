output "instancia" {
  value = google_sql_database_instance.pg.name
}

output "ip_privada" {
  value = google_sql_database_instance.pg.private_ip_address
}
