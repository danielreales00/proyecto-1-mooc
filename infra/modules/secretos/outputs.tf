output "ids" {
  value = { for k, s in google_secret_manager_secret.s : k => s.id }
}
