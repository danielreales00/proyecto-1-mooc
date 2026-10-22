# La cola compartida, fuera de los grupos (ADR-0018, D2).
module "cola" {
  source = "../../modules/cola"

  zona         = var.zona
  tipo_maquina = var.tipo_cola
  imagen       = var.imagen_maquinas
  disco_gb     = var.disco_cola_gb
  encendida    = var.entorno_encendido
  subred_id    = module.red.subred_id
  ip_interna   = module.red.ip_interna_cola
  cuenta       = google_service_account.cola.email

  env = <<-EOT
    IP_PRIVADA=${module.red.ip_interna_cola}
  EOT

  compose    = file("${path.module}/../../../deploy/compose.cola.yml")
  publicador = file("${path.module}/../../../deploy/publicar-cola.sh")
  arranque   = file("${path.module}/../../../deploy/arranque.sh")

  depends_on = [google_project_iam_member.maquinas]
}
