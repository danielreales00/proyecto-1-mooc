# La función de insignias (ADR-0021): una Cloud Run function de segunda
# generación que recibe los trabajos badge.issue de su propio topic por
# Eventarc. El backend enruta ese tipo a mooc-insignias y el worker deja de
# atenderlo (PUBSUB_TOPICOS en cloudrun.tf).

# --- el topic -------------------------------------------------------------------

resource "google_pubsub_topic" "insignias" {
  name                       = "mooc-insignias"
  message_retention_duration = "86400s"

  depends_on = [google_project_service.apis]
}

# El backend publica los badge.issue y el reaper de los workers republica los
# que se quedaron sin ejecutar. Nadie más.
resource "google_pubsub_topic_iam_member" "publicadores_insignias" {
  for_each = {
    backend = google_service_account.web.email
    workers = google_service_account.worker.email
  }

  topic  = google_pubsub_topic.insignias.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${each.value}"
}

# --- el código desplegable ------------------------------------------------------

# El paquete raíz del módulo backend/ es la función (backend/funcion.go). Se
# sube el módulo entero porque la función compila con sus paquetes internal.
# El hash va en el nombre del objeto: si el código cambia, cambia el objeto y
# `apply` redespliega la función.
data "archive_file" "funcion" {
  type        = "zip"
  source_dir  = "${path.module}/../../../backend"
  output_path = "${path.module}/.terraform/funcion-insignias.zip"
  # Fuera solo lo que no es Go. migrations y openapi son paquetes que
  # incrustan sus archivos, y el buildpack compila el módulo entero.
  excludes = ["bin", "tmp", "coverage.out", "Dockerfile", "cloudrun"]
}

resource "google_storage_bucket" "fuentes" {
  name                        = "${var.project_id}-fuentes"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = true

  # Solo hace falta el código de la versión desplegada.
  lifecycle_rule {
    condition {
      num_newer_versions = 3
    }
    action {
      type = "Delete"
    }
  }
  versioning {
    enabled = true
  }

  depends_on = [google_project_service.apis]
}

resource "google_storage_bucket_object" "funcion" {
  name   = "funcion-insignias-${data.archive_file.funcion.output_md5}.zip"
  bucket = google_storage_bucket.fuentes.name
  source = data.archive_file.funcion.output_path
}

# --- identidades ----------------------------------------------------------------

# La función: lee la cadena de conexión y escribe las imágenes de las
# insignias. Nada más.
resource "google_service_account" "insignias" {
  account_id   = "mooc-insignias"
  display_name = "Función de insignias (Cloud Run functions)"
  depends_on   = [google_project_service.apis]
}

resource "google_storage_bucket_iam_member" "insignias_badges" {
  bucket = module.gcs.nombres["badges"]
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.insignias.email}"
}

# El disparador: recibe los eventos de Eventarc e invoca la función.
resource "google_service_account" "eventarc" {
  account_id   = "mooc-eventarc"
  display_name = "Disparador de Eventarc hacia la función de insignias"
  depends_on   = [google_project_service.apis]
}

resource "google_project_iam_member" "eventarc_receptor" {
  project = var.project_id
  role    = "roles/eventarc.eventReceiver"
  member  = "serviceAccount:${google_service_account.eventarc.email}"
}

# La función de segunda generación corre sobre un servicio de Cloud Run con su
# mismo nombre. Solo el disparador puede invocarlo.
resource "google_cloud_run_v2_service_iam_member" "eventarc_invoca" {
  name     = google_cloudfunctions2_function.insignias.name
  location = var.region
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.eventarc.email}"
}

# El agente de Pub/Sub firma el token de cada entrega con la cuenta del
# disparador, como hace con la de la suscripción push de los workers.
resource "google_service_account_iam_member" "pubsub_firma_eventarc" {
  service_account_id = google_service_account.eventarc.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = local.agente_pubsub
}

# La compilación: Cloud Build construye la función con esta cuenta y no con la
# de Compute Engine por omisión, que tiene permisos de editor.
resource "google_service_account" "compilacion" {
  account_id   = "mooc-compilacion"
  display_name = "Cloud Build: compila la función de insignias"
  depends_on   = [google_project_service.apis]
}

resource "google_project_iam_member" "compilacion" {
  for_each = toset([
    "roles/logging.logWriter",
    "roles/artifactregistry.writer",
  ])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.compilacion.email}"
}

# Cloud Run functions copia el código a un bucket propio,
# gcf-v2-sources-<número>-<región>, que crea con la primera función, y la
# compilación lo lee de ahí. Como no existe antes del primer apply, el permiso
# va en el proyecto, con una condición que lo limita a esos buckets: la cuenta
# no puede leer los originales ni ningún otro.
resource "google_project_iam_member" "compilacion_lee_fuentes_gcf" {
  project = var.project_id
  role    = "roles/storage.objectViewer"
  member  = "serviceAccount:${google_service_account.compilacion.email}"

  condition {
    title       = "solo-fuentes-de-funciones"
    description = "Solo los buckets donde Cloud Run functions guarda el código"
    expression  = "resource.name.startsWith(\"projects/_/buckets/gcf-v2-sources-\")"
  }
}

resource "google_storage_bucket_iam_member" "compilacion_fuentes" {
  bucket = google_storage_bucket.fuentes.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.compilacion.email}"
}

# --- la función -----------------------------------------------------------------

resource "google_cloudfunctions2_function" "insignias" {
  name     = "mooc-insignias"
  location = var.region

  build_config {
    runtime         = "go126"
    entry_point     = "EmitirInsignia"
    service_account = google_service_account.compilacion.id
    source {
      storage_source {
        bucket = google_storage_bucket.fuentes.name
        object = google_storage_bucket_object.funcion.name
      }
    }
  }

  service_config {
    min_instance_count               = 0
    max_instance_count               = var.funcion.max_instancias
    max_instance_request_concurrency = 1
    available_cpu                    = var.funcion.cpu
    available_memory                 = var.funcion.memoria
    timeout_seconds                  = var.funcion.tiempo_maximo_s
    service_account_email            = google_service_account.insignias.email
    ingress_settings                 = "ALLOW_INTERNAL_ONLY"
    all_traffic_on_latest_revision   = true

    # Llega a Cloud SQL por su IP privada, como los servicios de Cloud Run,
    # sin conector que cobre por hora (ADR-0021, D6).
    direct_vpc_network_interface {
      network    = module.red.red_id
      subnetwork = module.red.subred_id
    }
    direct_vpc_egress = "VPC_EGRESS_PRIVATE_RANGES_ONLY"

    environment_variables = merge(local.env_comun, {
      DB_MAX_CONNS        = tostring(var.funcion.conexiones_db)
      PUBSUB_MAX_ENTREGAS = tostring(var.pubsub_max_entregas)
    })

    secret_environment_variables {
      key        = "DATABASE_URL"
      project_id = var.project_id
      secret     = "database-url"
      version    = "latest"
    }
  }

  event_trigger {
    trigger_region        = var.region
    event_type            = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic          = google_pubsub_topic.insignias.id
    retry_policy          = "RETRY_POLICY_RETRY"
    service_account_email = google_service_account.eventarc.email
  }

  depends_on = [
    google_project_service.apis,
    google_project_iam_member.compilacion,
    google_storage_bucket_iam_member.compilacion_fuentes,
    google_project_iam_member.compilacion_lee_fuentes_gcf,
    module.secretos,
  ]
}
