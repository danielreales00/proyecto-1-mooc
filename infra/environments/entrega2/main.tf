# Entorno único de la Entrega 2 (ADR-0016, D6): red, datos, secretos y
# registro. Las máquinas llegan en A3.

module "red" {
  source = "../../modules/red"

  nombre            = "mooc"
  region            = var.region
  cidr_subred       = var.cidr_subred
  cidr_servicios    = var.cidr_servicios
  ip_interna_web    = var.ip_interna_web
  ip_interna_worker = var.ip_interna_worker
  cuenta_web        = google_service_account.web.email
  cuenta_worker     = google_service_account.worker.email

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

  depends_on = [google_project_service.apis]
}

module "registry" {
  source = "../../modules/registry"

  nombre   = "mooc"
  region   = var.region
  lectores = [google_service_account.web.email, google_service_account.worker.email]

  depends_on = [google_project_service.apis]
}
