output "nombres" {
  value = { for k, m in google_compute_instance.m : k => m.name }
}
