# El transporte de los trabajos (ADR-0020): un topic, una suscripción push
# autenticada hacia los workers y un topic de mensajes muertos.

resource "google_pubsub_topic" "trabajos" {
  name = "mooc-trabajos"
  # Un mensaje sin entregar se conserva un día: lo bastante para que los
  # workers lo recojan tras un arranque en frío o una caída breve.
  message_retention_duration = "86400s"

  depends_on = [google_project_service.apis]
}

resource "google_pubsub_topic" "muertos" {
  name = "mooc-trabajos-muertos"

  depends_on = [google_project_service.apis]
}

resource "google_pubsub_subscription" "workers" {
  name  = "mooc-workers"
  topic = google_pubsub_topic.trabajos.id

  # Cubre la transcodificación más larga medida (196 s) con margen, y es el
  # mismo tiempo máximo de solicitud del servicio de workers (D3).
  ack_deadline_seconds = 600

  push_config {
    push_endpoint = "${local.url.workers}/pubsub/push"

    # Pub/Sub firma cada entrega con un token OIDC de esta cuenta. Cloud Run lo
    # valida antes de llegar al worker: es la única cuenta, con la de Cloud
    # Scheduler, que puede invocarlo.
    oidc_token {
      service_account_email = google_service_account.pubsub_push.email
      audience              = local.url.workers
    }
  }

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }

  # En la última entrega el worker marca el trabajo `dead` en la base, y
  # Pub/Sub manda el mensaje aquí.
  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.muertos.id
    max_delivery_attempts = var.pubsub_max_entregas
  }

  # La suscripción no caduca aunque pase un mes sin mensajes.
  expiration_policy {
    ttl = ""
  }
}

# Una suscripción sin destino para consultar los mensajes muertos:
# `gcloud pubsub subscriptions pull mooc-trabajos-muertos-revision`.
resource "google_pubsub_subscription" "muertos" {
  name                       = "mooc-trabajos-muertos-revision"
  topic                      = google_pubsub_topic.muertos.id
  message_retention_duration = "604800s"

  expiration_policy {
    ttl = ""
  }
}

# --- quién publica --------------------------------------------------------------

# La API publica los trabajos y los workers publican los que encadena un
# trabajo (verificar → escanear → transcodificar). Nadie más.
resource "google_pubsub_topic_iam_member" "publicadores" {
  for_each = {
    backend = google_service_account.web.email
    workers = google_service_account.worker.email
  }

  topic  = google_pubsub_topic.trabajos.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${each.value}"
}

# --- el agente de Pub/Sub -------------------------------------------------------

locals {
  agente_pubsub = "serviceAccount:service-${local.numero_proyecto}@gcp-sa-pubsub.iam.gserviceaccount.com"
}

# Para firmar el token OIDC de cada entrega en nombre de la cuenta de push.
resource "google_service_account_iam_member" "pubsub_firma_push" {
  service_account_id = google_service_account.pubsub_push.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = local.agente_pubsub

  depends_on = [google_project_service.apis]
}

# Para mover al topic de muertos un mensaje que agotó sus entregas.
resource "google_pubsub_topic_iam_member" "agente_muertos" {
  topic  = google_pubsub_topic.muertos.name
  role   = "roles/pubsub.publisher"
  member = local.agente_pubsub
}

resource "google_pubsub_subscription_iam_member" "agente_workers" {
  subscription = google_pubsub_subscription.workers.name
  role         = "roles/pubsub.subscriber"
  member       = local.agente_pubsub
}

# --- quién invoca a los workers -------------------------------------------------

resource "google_cloud_run_v2_service_iam_member" "invocadores_workers" {
  for_each = {
    pubsub      = google_service_account.pubsub_push.email
    programador = google_service_account.programador.email
  }

  name     = google_cloud_run_v2_service.workers.name
  location = var.region
  role     = "roles/run.invoker"
  member   = "serviceAccount:${each.value}"
}

# --- el reaper y la poda (D6) ---------------------------------------------------

resource "google_cloud_scheduler_job" "tareas" {
  for_each = {
    reaper = { ruta = "/tareas/reaper", cada = "*/5 * * * *" }
    podar  = { ruta = "/tareas/podar", cada = "17 * * * *" }
  }

  name      = "mooc-${each.key}"
  region    = var.region
  schedule  = each.value.cada
  time_zone = "Etc/UTC"
  paused    = !var.entorno_encendido

  http_target {
    http_method = "POST"
    uri         = "${local.url.workers}${each.value.ruta}"

    oidc_token {
      service_account_email = google_service_account.programador.email
      audience              = local.url.workers
    }
  }

  depends_on = [google_project_service.apis]
}
