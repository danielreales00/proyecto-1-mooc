# Entorno de la Entrega 5 (ADR-0021). Hereda el estado de la Entrega 4 sin
# recrear nada: red, datos, secretos, registro y cola aquí, los tres servicios
# de Cloud Run en cloudrun.tf, el transporte de los trabajos en pubsub.tf y la
# función de insignias en funcion.tf.

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

  # La salida a Internet solo la usan las maquinas, asi que sigue al mismo
  # interruptor: con el entorno apagado, la NAT deja de cobrar.
  salida_encendida = var.entorno_encendido
}

module "secretos" {
  source = "../../modules/secretos"

  region = var.region
  secretos = {
    # Lo escribe el módulo de Cloud SQL, con la clave que genera.
    # La función de insignias también la lee (funcion.tf).
    "database-url" = [google_service_account.web.email, google_service_account.worker.email,
    google_service_account.insignias.email]
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
  # El navegador sube las partes y reproduce HLS directo desde Cloud Storage.
  frontend_origins = local.origenes_frontend

  depends_on = [google_project_service.apis]
}

module "registry" {
  source = "../../modules/registry"

  nombre   = "mooc"
  region   = var.region
  lectores = [google_service_account.web.email, google_service_account.worker.email]

  depends_on = [google_project_service.apis]
}
