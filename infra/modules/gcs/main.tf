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
