# Las dos máquinas y lo que cada una recibe al arrancar. Aquí vive la
# configuración sin secretos de cada una; los secretos los lee la propia
# máquina de Secret Manager (deploy/arranque.sh).

locals {
  nombre_publico = coalesce(var.nombre_publico, "${replace(module.red.ip_externa_web, ".", "-")}.sslip.io")
  registro       = module.registry.url

  # Lo común a las dos. Ver «Variables por máquina» en despliegue-gcp.md.
  env_comun = <<-EOT
    APP_ENV=production
    LOG_LEVEL=info
    LOG_FORMAT=gcp
    PUBLIC_BASE_URL=https://${local.nombre_publico}
    REDIS_TLS=false
    OBJECT_STORE=gcs
    S3_BUCKET_ORIGINALS=${module.gcs.nombres["originals"]}
    S3_BUCKET_DERIVED=${module.gcs.nombres["derived"]}
    S3_BUCKET_BADGES=${module.gcs.nombres["badges"]}
    S3_BUCKET_QUARANTINE=${module.gcs.nombres["quarantine"]}
    MAIL_FROM=no-reply@${local.nombre_publico}
    REGISTRO=${local.registro}
    VERSION=${var.version_imagenes}
  EOT
}

module "maquinas" {
  source = "../../modules/maquinas"

  zona         = var.zona
  encendidas   = var.maquinas_encendidas
  tipo_maquina = var.tipo_maquina
  imagen       = var.imagen_maquinas
  disco_gb     = var.disco_maquinas_gb
  subred_id    = module.red.subred_id

  ip_interna_web    = module.red.ip_interna_web
  ip_interna_worker = module.red.ip_interna_worker
  ip_externa_web    = module.red.ip_externa_web
  cuenta_web        = google_service_account.web.email
  cuenta_worker     = google_service_account.worker.email

  arranque       = file("${path.module}/../../../deploy/arranque.sh")
  compose_web    = file("${path.module}/../../../deploy/compose.web.yml")
  compose_worker = file("${path.module}/../../../deploy/compose.worker.yml")
  caddyfile      = file("${path.module}/../../../deploy/Caddyfile.produccion")
  clamd_conf     = file("${path.module}/../../../deploy/clamd.conf")

  env_web = <<-EOT
    ${local.env_comun}
    HTTP_ADDR=:8080
    DB_MAX_CONNS=${var.db_max_conns_api}
    DOMINIO=${local.nombre_publico}
    TRUSTED_PROXY_HOPS=0
    RATE_LIMIT_IP_FACTOR=${var.factor_limite_ip}
    REDIS_ADDR=${module.red.ip_interna_worker}:6379
    # La API no envía correo, pero la configuración lo exige.
    SMTP_ADDR=${module.red.ip_interna_worker}:1025
  EOT

  env_worker = <<-EOT
    ${local.env_comun}
    IP_PRIVADA=${module.red.ip_interna_worker}
    DB_MAX_CONNS=${var.db_max_conns}
    REDIS_ADDR=redis:6379
    CLAMAV_ADDR=clamav:3310
    # Mailpit hasta elegir proveedor en A4.
    SMTP_ADDR=mailpit:1025
  EOT

  # La base tiene que existir antes que la API intente migrar.
  depends_on = [module.cloudsql]
}
