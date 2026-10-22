# Los dos grupos elásticos y lo que cada instancia recibe al arrancar
# (ADR-0018, D2). Aquí vive la configuración sin secretos de cada papel; los
# secretos los lee la propia instancia de Secret Manager (deploy/arranque.sh).

locals {
  nombre_publico = module.balanceador.nombre_publico
  registro       = module.registry.url
  redis_addr     = "${module.red.ip_interna_cola}:6379"
  smtp_addr      = "${module.red.ip_interna_cola}:1025"

  # Lo común a los dos grupos. Ver «Variables por papel» en
  # despliegue-gcp-entrega3.md.
  env_comun = <<-EOT
    APP_ENV=production
    LOG_LEVEL=info
    LOG_FORMAT=gcp
    PUBLIC_BASE_URL=https://${local.nombre_publico}
    REDIS_ADDR=${local.redis_addr}
    REDIS_TLS=false
    SMTP_ADDR=${local.smtp_addr}
    OBJECT_STORE=gcs
    S3_BUCKET_ORIGINALS=${module.gcs.nombres["originals"]}
    S3_BUCKET_DERIVED=${module.gcs.nombres["derived"]}
    S3_BUCKET_BADGES=${module.gcs.nombres["badges"]}
    S3_BUCKET_QUARANTINE=${module.gcs.nombres["quarantine"]}
    MAIL_FROM=no-reply@${local.nombre_publico}
    REGISTRO=${local.registro}
    VERSION=${var.version_imagenes}
  EOT

  arranque = file("${path.module}/../../../deploy/arranque.sh")
}

module "api" {
  source = "../../modules/grupos"

  nombre           = "mooc-api"
  rol              = "web"
  zona             = var.zona
  tipo_maquina     = var.tipo_maquina
  imagen           = var.imagen_grupos
  disco_gb         = var.disco_maquinas_gb
  subred_id        = module.red.subred_id
  cuenta           = google_service_account.web.email
  version_imagenes = var.version_imagenes

  archivos = {
    mooc-compose   = file("${path.module}/../../../deploy/compose.web.yml")
    mooc-caddyfile = file("${path.module}/../../../deploy/Caddyfile.produccion")
  }
  arranque = local.arranque

  # El balanceador añade «<cliente>, <ip del balanceador>» a X-Forwarded-For y
  # Caddy lo deja pasar tal cual: la IP real queda a un salto de la derecha.
  env = <<-EOT
    ${local.env_comun}
    HTTP_ADDR=:8080
    DB_MAX_CONNS=${var.db_max_conns_api}
    TRUSTED_PROXY_HOPS=1
    RATE_LIMIT_IP_FACTOR=${var.factor_limite_ip}
  EOT

  puerto_nombrado           = { nombre = "http", puerto = 80 }
  salud_vida                = { puerto = 80, ruta = "/healthz" }
  retraso_autorreparacion_s = var.retraso_autorreparacion_s

  encendido            = var.entorno_encendido
  autoescalado         = var.autoescalado
  inicializacion_s     = var.inicializacion_api_s
  ventana_reduccion_s  = var.ventana_reduccion_s
  objetivo_balanceador = var.objetivo_balanceador_api

  # La base y la cola tienen que existir antes que la primera API intente
  # migrar y conectarse.
  depends_on = [module.cloudsql, module.cola]
}

module "workers" {
  source = "../../modules/grupos"

  nombre           = "mooc-worker"
  rol              = "worker"
  zona             = var.zona
  tipo_maquina     = var.tipo_maquina
  imagen           = var.imagen_grupos
  disco_gb         = var.disco_maquinas_gb
  subred_id        = module.red.subred_id
  cuenta           = google_service_account.worker.email
  version_imagenes = var.version_imagenes

  archivos = {
    mooc-compose = file("${path.module}/../../../deploy/compose.worker.yml")
    mooc-clamd   = file("${path.module}/../../../deploy/clamd.conf")
  }
  arranque = local.arranque
  apagado  = file("${path.module}/../../../deploy/apagado-worker.sh")

  env = <<-EOT
    ${local.env_comun}
    DB_MAX_CONNS=${var.db_max_conns}
    CLAMAV_ADDR=clamav:3310
  EOT

  salud_vida                = { puerto = 8081, ruta = "/healthz" }
  retraso_autorreparacion_s = var.retraso_autorreparacion_s

  encendido           = var.entorno_encendido
  autoescalado        = var.autoescalado
  inicializacion_s    = var.inicializacion_workers_s
  ventana_reduccion_s = var.ventana_reduccion_s

  # Trabajos de `bulk` pendientes más en curso, publicados por la cola. Con
  # una asignación de N por instancia, el autoscaler pide ceil(trabajos / N)
  # instancias, entre 1 y 3 (ADR-0018, D5).
  metrica = {
    nombre        = "custom.googleapis.com/mooc/cola_bulk_trabajos"
    filtro        = "resource.type = \"global\""
    por_instancia = var.trabajos_por_worker
  }

  depends_on = [module.cloudsql, module.cola]
}
