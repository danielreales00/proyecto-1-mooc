# Los tres servicios de Cloud Run y los dos trabajos de despliegue (ADR-0020).
#
# Cada servicio escala de 0 a 3 instancias, con un contenedor por instancia. La
# configuración sin secretos va en variables de entorno y los secretos salen de
# Secret Manager por referencia: nunca están en la plantilla ni en el plan.

locals {
  numero_proyecto = data.google_project.este.number

  # URL deterministas de Cloud Run. Se conocen antes de crear los servicios, así
  # que el frontend recibe la del backend y el backend la del frontend (CORS)
  # sin que un servicio dependa del otro.
  url = {
    for s in ["frontend", "backend", "workers"] :
    s => "https://mooc-${s}-${local.numero_proyecto}.${var.region}.run.app"
  }
  host_frontend = trimprefix(local.url.frontend, "https://")

  imagen = {
    for i in ["frontend", "api", "workers-cloudrun", "migrate", "seed"] :
    i => "${module.registry.url}/${i}:${var.version_imagenes}"
  }

  origenes_frontend = distinct([local.url.frontend, google_cloud_run_v2_service.frontend.uri])

  redis_addr = "${module.red.ip_interna_cola}:6379"
  smtp_addr  = "${module.red.ip_interna_cola}:1025"

  # Lo que comparten el backend y los workers.
  env_comun = {
    APP_ENV              = "production"
    LOG_LEVEL            = "info"
    LOG_FORMAT           = "gcp"
    PUBLIC_BASE_URL      = local.url.frontend
    OBJECT_STORE         = "gcs"
    S3_BUCKET_ORIGINALS  = module.gcs.nombres["originals"]
    S3_BUCKET_DERIVED    = module.gcs.nombres["derived"]
    S3_BUCKET_BADGES     = module.gcs.nombres["badges"]
    S3_BUCKET_QUARANTINE = module.gcs.nombres["quarantine"]
    MAIL_FROM            = "no-reply@${local.host_frontend}"
    SMTP_ADDR            = local.smtp_addr
    QUEUE_BACKEND        = "pubsub"
    PUBSUB_PROJECT       = var.project_id
    PUBSUB_TOPIC         = google_pubsub_topic.trabajos.name
  }

  env_backend = merge(local.env_comun, {
    REDIS_ADDR   = local.redis_addr
    REDIS_TLS    = "false"
    DB_MAX_CONNS = tostring(var.db_max_conns_api)
    # Las dos URL con que Cloud Run publica el frontend: la determinista y la
    # de su primera versión. El navegador llega por cualquiera.
    CORS_ORIGINS         = join(",", local.origenes_frontend)
    RATE_LIMIT_IP_FACTOR = tostring(var.factor_limite_ip)
    # Sin proxy delante, /metrics sería público. Cloud Monitoring ya tiene las
    # métricas del servicio.
    METRICS_ENABLED = "false"
  })

  env_workers = merge(local.env_comun, {
    DB_MAX_CONNS        = tostring(var.db_max_conns)
    PUBSUB_MAX_ENTREGAS = tostring(var.pubsub_max_entregas)
    # Con Pub/Sub un trabajo puede esperar turno sin estar perdido (D6).
    REAPER_EDAD_MINIMA = "10m"
  })
}

# --- backend ------------------------------------------------------------------

resource "google_cloud_run_v2_service" "backend" {
  name                = "mooc-backend"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false

  template {
    service_account                  = google_service_account.web.email
    max_instance_request_concurrency = var.backend.concurrencia
    timeout                          = "${var.backend.tiempo_maximo_s}s"

    scaling {
      min_instance_count = 0
      max_instance_count = var.max_instancias
    }

    # Salida directa a la VPC: Cloud SQL por IP privada y Redis en mooc-cola.
    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = module.red.red_id
        subnetwork = module.red.subred_id
      }
    }

    containers {
      image = local.imagen.api

      resources {
        limits = { cpu = var.backend.cpu, memory = var.backend.memoria }
      }

      dynamic "env" {
        for_each = local.env_backend
        content {
          name  = env.key
          value = env.value
        }
      }
      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = module.secretos.ids["database-url"]
            version = "latest"
          }
        }
      }

      startup_probe {
        http_get {
          path = "/healthz"
        }
        period_seconds    = 3
        failure_threshold = 20
      }
    }
  }

  depends_on = [google_project_service.apis, module.secretos]
}

# --- workers ------------------------------------------------------------------

resource "google_cloud_run_v2_service" "workers" {
  name     = "mooc-workers"
  location = var.region
  # Solo tráfico interno: Pub/Sub y Cloud Scheduler llegan, Internet no.
  ingress             = "INGRESS_TRAFFIC_INTERNAL_ONLY"
  deletion_protection = false

  template {
    service_account                  = google_service_account.worker.email
    max_instance_request_concurrency = var.workers.concurrencia
    timeout                          = "${var.workers.tiempo_maximo_s}s"
    # FFmpeg y clamd necesitan el entorno completo de Linux.
    execution_environment = "EXECUTION_ENVIRONMENT_GEN2"

    scaling {
      min_instance_count = 0
      max_instance_count = var.max_instancias
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = module.red.red_id
        subnetwork = module.red.subred_id
      }
    }

    containers {
      image = local.imagen["workers-cloudrun"]

      resources {
        limits = { cpu = var.workers.cpu, memory = var.workers.memoria }
      }

      dynamic "env" {
        for_each = local.env_workers
        content {
          name  = env.key
          value = env.value
        }
      }
      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = module.secretos.ids["database-url"]
            version = "latest"
          }
        }
      }

      # clamd carga sus firmas antes de que el worker escuche: unos 40 s.
      startup_probe {
        http_get {
          path = "/healthz"
        }
        period_seconds    = 5
        failure_threshold = 36
      }
    }
  }

  depends_on = [google_project_service.apis, module.secretos]
}

# --- frontend -----------------------------------------------------------------

resource "google_cloud_run_v2_service" "frontend" {
  name                = "mooc-frontend"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false

  template {
    service_account                  = google_service_account.frontend.email
    max_instance_request_concurrency = var.frontend.concurrencia
    timeout                          = "${var.frontend.tiempo_maximo_s}s"

    scaling {
      min_instance_count = 0
      max_instance_count = var.max_instancias
    }

    containers {
      image = local.imagen.frontend

      resources {
        limits = { cpu = var.frontend.cpu, memory = var.frontend.memoria }
      }

      # /config.js entrega esta URL a la interfaz (ADR-0020, D7).
      env {
        name  = "API_URL"
        value = local.url.backend
      }
    }
  }

  depends_on = [google_project_service.apis]
}

# El frontend y el backend son públicos: la autenticación de la aplicación la
# hace la API con su sesión. Los workers no, ver pubsub.tf.
resource "google_cloud_run_v2_service_iam_member" "publico" {
  for_each = {
    frontend = google_cloud_run_v2_service.frontend.name
    backend  = google_cloud_run_v2_service.backend.name
  }

  name     = each.value
  location = var.region
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# --- trabajos de despliegue ---------------------------------------------------

# Las migraciones corren una vez por despliegue, no en cada réplica (D9).
resource "google_cloud_run_v2_job" "migrate" {
  name                = "mooc-migrate"
  location            = var.region
  deletion_protection = false

  template {
    template {
      service_account = google_service_account.web.email
      max_retries     = 1
      timeout         = "300s"

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = module.red.red_id
          subnetwork = module.red.subred_id
        }
      }

      containers {
        image = local.imagen.migrate
        # config.Load exige la misma configuración que la API, aunque migrar
        # solo use la base.
        dynamic "env" {
          for_each = local.env_backend
          content {
            name  = env.key
            value = env.value
          }
        }
        env {
          name = "DATABASE_URL"
          value_source {
            secret_key_ref {
              secret  = module.secretos.ids["database-url"]
              version = "latest"
            }
          }
        }
      }
    }
  }

  depends_on = [google_project_service.apis, module.secretos]
}

# Las cuentas sintéticas, con la contraseña de Secret Manager (semilla.tf).
resource "google_cloud_run_v2_job" "seed" {
  name                = "mooc-seed"
  location            = var.region
  deletion_protection = false

  template {
    template {
      service_account = google_service_account.web.email
      max_retries     = 0
      timeout         = "600s"

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = module.red.red_id
          subnetwork = module.red.subred_id
        }
      }

      containers {
        image = local.imagen.seed
        dynamic "env" {
          for_each = local.env_backend
          content {
            name  = env.key
            value = env.value
          }
        }
        env {
          name  = "SEED_FORCE"
          value = "1"
        }
        env {
          name = "DATABASE_URL"
          value_source {
            secret_key_ref {
              secret  = module.secretos.ids["database-url"]
              version = "latest"
            }
          }
        }
        env {
          name = "SEED_PASSWORD"
          value_source {
            secret_key_ref {
              secret  = module.secretos.ids["seed-password"]
              version = "latest"
            }
          }
        }
      }
    }
  }

  depends_on = [google_project_service.apis, module.secretos]
}
