# Una cuenta de servicio por máquina, con lo mínimo que usa (ADR-0016, D7).
# Ninguna tiene clave descargada: la máquina se autentica con su identidad de
# instancia y nada más (ADR-0015, D3).

resource "google_service_account" "web" {
  account_id   = "mooc-web"
  display_name = "Web Server (caddy, api)"
  depends_on   = [google_project_service.apis]
}

resource "google_service_account" "worker" {
  account_id   = "mooc-worker"
  display_name = "Worker Server (worker, worker-media)"
  depends_on   = [google_project_service.apis]
}

# El permiso que todo el mundo olvida. La API firma URL V4 sin clave privada
# llamando a signBlob de IAM Credentials con su propia identidad, y para eso
# necesita ser creadora de tokens sobre sí misma. Sin esto, cada carga directa
# y cada reproducción fallan con 403.
resource "google_service_account_iam_member" "web_firma_url" {
  service_account_id = google_service_account.web.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.web.email}"
}

# Quien desarrolla el adaptador de GCS firma desde su máquina, con su ADC de
# usuario, en nombre de mooc-web: así la prueba firma con los mismos permisos
# que la API desplegada. roles/owner no incluye este permiso; hay que darlo.
resource "google_service_account_iam_member" "web_firma_desarrollo" {
  for_each = toset(var.firmantes_desarrollo)

  service_account_id = google_service_account.web.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = each.value
}

# Registros y métricas del agente de las máquinas (A3): memoria y disco no
# están en las métricas por defecto, y el informe las pide.
locals {
  roles_proyecto = {
    for par in setproduct(
      ["roles/logging.logWriter", "roles/monitoring.metricWriter"],
      ["web", "worker"],
    ) : "${par[1]}/${par[0]}" => { rol = par[0], maquina = par[1] }
  }

  cuentas = {
    web    = google_service_account.web.email
    worker = google_service_account.worker.email
  }
}

resource "google_project_iam_member" "maquinas" {
  for_each = local.roles_proyecto

  project = var.project_id
  role    = each.value.rol
  member  = "serviceAccount:${local.cuentas[each.value.maquina]}"
}

# --- Objetos ------------------------------------------------------------------
# La API crea y completa las cargas en `originals`, mueve lo infectado a
# `quarantine` y emite insignias; de `derived` solo lee listas HLS y firma
# segmentos. El worker escanea, mueve, transcodifica y emite: escribe en todos.
locals {
  acceso_objetos = {
    "web/originals"     = { maquina = "web", bucket = "originals", rol = "roles/storage.objectAdmin" }
    "web/quarantine"    = { maquina = "web", bucket = "quarantine", rol = "roles/storage.objectAdmin" }
    "web/badges"        = { maquina = "web", bucket = "badges", rol = "roles/storage.objectAdmin" }
    "web/derived"       = { maquina = "web", bucket = "derived", rol = "roles/storage.objectViewer" }
    "worker/originals"  = { maquina = "worker", bucket = "originals", rol = "roles/storage.objectAdmin" }
    "worker/quarantine" = { maquina = "worker", bucket = "quarantine", rol = "roles/storage.objectAdmin" }
    "worker/badges"     = { maquina = "worker", bucket = "badges", rol = "roles/storage.objectAdmin" }
    "worker/derived"    = { maquina = "worker", bucket = "derived", rol = "roles/storage.objectAdmin" }
  }
}

resource "google_storage_bucket_iam_member" "maquinas" {
  for_each = local.acceso_objetos

  bucket = module.gcs.nombres[each.value.bucket]
  role   = each.value.rol
  member = "serviceAccount:${local.cuentas[each.value.maquina]}"
}
