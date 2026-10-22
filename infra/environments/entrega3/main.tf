# Entorno de la Entrega 3 (ADR-0018): red, datos, secretos y registro, heredados
# del estado de entrega2 sin recrearse. La cola, los grupos y el balanceador
# están en cola.tf, grupos.tf y balanceador.tf.

module "red" {
  source = "../../modules/red"

  nombre          = "mooc"
  region          = var.region
  cidr_subred     = var.cidr_subred
  cidr_servicios  = var.cidr_servicios
  ip_interna_cola = var.ip_interna_cola
  cuenta_web      = google_service_account.web.email
  cuenta_worker   = google_service_account.worker.email
  cuenta_cola     = google_service_account.cola.email

  depends_on = [google_project_service.apis]
}

module "secretos" {
  source = "../../modules/secretos"

  region = var.region
  secretos = {
    # Lo escribe el módulo de Cloud SQL, con la clave que genera.
    "database-url" = [google_service_account.web.email, google_service_account.worker.email]
    # Se carga a mano cuando se elija proveedor de correo (A4). El correo lo
    # envía el worker (trabajo email.send), no la API.
    "smtp-password" = [google_service_account.worker.email]
    # Contraseña de las cuentas sintéticas en la nube. La lee una instancia
    # del grupo de API, donde corre la semilla (semilla.tf).
    "seed-password" = [google_service_account.web.email]
  }

  depends_on = [google_project_service.apis]
}

module "cloudsql" {
  source = "../../modules/cloudsql"

  nombre               = "mooc-pg"
  region               = var.region
  zona                 = var.zona
  tier                 = var.cloudsql_tier
  encendida            = var.cloudsql_encendida
  disco_gb             = var.cloudsql_disco_gb
  red_id               = module.red.red_id
  conexion_servicios   = module.red.conexion_servicios
  base                 = "mooc"
  usuario              = "mooc"
  version_clave        = var.version_clave_db
  secreto_database_url = module.secretos.ids["database-url"]
  proteger_borrado     = var.proteger_borrado
}

module "gcs" {
  source = "../../modules/gcs"

  prefijo            = var.project_id
  region             = var.region
  borrar_con_objetos = !var.proteger_borrado
  frontend_origins   = ["https://${local.nombre_publico}"]

  depends_on = [google_project_service.apis]
}

module "registry" {
  source = "../../modules/registry"

  nombre   = "mooc"
  region   = var.region
  lectores = [google_service_account.web.email, google_service_account.worker.email]

  depends_on = [google_project_service.apis]
}
