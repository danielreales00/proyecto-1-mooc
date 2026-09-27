# Contenedores de Secret Manager y quién puede leerlos. Los valores no pasan
# por aquí (ADR-0016, D7): o los escribe otro recurso con atributos de solo
# escritura, o se cargan aparte.
resource "google_secret_manager_secret" "s" {
  for_each = var.secretos

  secret_id = each.key

  replication {
    user_managed {
      replicas {
        location = var.region
      }
    }
  }
}

locals {
  accesos = merge([
    for secreto, lectores in var.secretos : {
      for lector in lectores : "${secreto}/${lector}" => {
        secreto = secreto
        lector  = lector
      }
    }
  ]...)
}

resource "google_secret_manager_secret_iam_member" "lectura" {
  for_each = local.accesos

  secret_id = google_secret_manager_secret.s[each.value.secreto].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${each.value.lector}"
}
