# Nombre de cada bucket por su papel: originals, derived, badges, quarantine.
output "nombres" {
  value = { for k, b in google_storage_bucket.b : k => b.name }
}
