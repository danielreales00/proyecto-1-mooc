provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zona

  # Terraform se autentica con el ADC de usuario (authorized_user). Algunas
  # APIs —la de presupuestos, la primera— rechazan esas credenciales si la
  # petición no nombra un proyecto de cuota. Con esto se cobra la cuota al
  # propio proyecto en todas las llamadas.
  user_project_override = true
  billing_project       = var.project_id

  default_labels = {
    proyecto = "mooc"
    entorno  = "entrega2"
  }
}
