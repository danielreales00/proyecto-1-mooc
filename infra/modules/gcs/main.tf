# Los cuatro buckets del ADR-0005, ahora en Cloud Storage. Ningún binario va a
# la base relacional: originales, derivados HLS, cuarentena e insignias viven
# aquí.
#
# Todos privados salvo el de insignias, que se lee sin firma porque verificar
# una insignia es público (CA-07). Los multimedia se entregan con URL firmada,
# directamente desde aquí y sin CDN (ADR-0016, D5).

locals {
  buckets = {
    originals  = { publico = false }
    derived    = { publico = false }
    badges     = { publico = true }
    quarantine = { publico = false }
    # Registros de acceso de los cinco buckets, incluido este.
    registros = { publico = false }
  }
}

resource "google_storage_bucket" "b" {
  for_each = local.buckets

  name          = "${var.prefijo}-${each.key}"
  location      = upper(var.region)
  storage_class = "STANDARD"
  force_destroy = var.borrar_con_objetos

  uniform_bucket_level_access = true
  public_access_prevention    = each.value.publico ? "inherited" : "enforced"

  # La carga en partes crea objetos temporales que se borran al componer. Con
  # el borrado suave por defecto se seguirían cobrando siete días más.
  soft_delete_policy {
    retention_duration_seconds = 0
  }

  # Un objeto sobrescrito o borrado por error se recupera durante un día. Las
  # versiones viejas se borran después: lo que sale de originals hacia
  # cuarentena no se queda como copia.
  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      days_since_noncurrent_time = 1
    }
    action {
      type = "Delete"
    }
  }

  logging {
    log_bucket = "${var.prefijo}-registros"
  }

  dynamic "cors" {
    for_each = length(var.frontend_origins) > 0 ? [1] : []
    content {
      origin          = var.frontend_origins
      method          = ["GET", "HEAD", "PUT"]
      response_header = ["Content-Type", "ETag", "Range", "Content-Range"]
      max_age_seconds = 3600
    }
  }

  dynamic "lifecycle_rule" {
    for_each = each.key == "registros" ? [1] : []
    content {
      condition {
        age = 30
      }
      action {
        type = "Delete"
      }
    }
  }

  # Partes de cargas que nunca se completaron. La carga multipart se emula
  # con `compose`, que exige que las partes estén en el mismo bucket que el
  # destino: por eso la regla vive en todos, aunque solo `originals` las use.
  lifecycle_rule {
    condition {
      age            = 7
      matches_prefix = ["tmp/"]
    }
    action {
      type = "Delete"
    }
  }
}

# Lectura pública sin listado. roles/storage.objectViewer incluiría
# storage.objects.list y dejaría enumerar las insignias de todo el mundo;
# legacyObjectReader solo concede leer un objeto cuyo nombre ya se conoce.
resource "google_storage_bucket_iam_member" "lectura_publica" {
  for_each = { for k, v in local.buckets : k => v if v.publico }

  bucket = google_storage_bucket.b[each.key].name
  role   = "roles/storage.legacyObjectReader"
  member = "allUsers"
}

# Cloud Storage entrega los registros de acceso con esta cuenta de Google.
resource "google_storage_bucket_iam_member" "entrega_registros" {
  bucket = google_storage_bucket.b["registros"].name
  role   = "roles/storage.objectCreator"
  member = "group:cloud-storage-analytics@google.com"
}
