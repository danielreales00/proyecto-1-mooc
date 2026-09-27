# Cloud SQL para PostgreSQL 17, como lo permite el enunciado: una zona, sin
# réplicas, sin alta disponibilidad y sin IP pública (ADR-0016, D3).

# Un nombre de instancia borrada no se puede reutilizar durante días. El sufijo
# deja que la reconstrucción de C2 (destroy + apply) no tropiece con eso.
resource "random_id" "sufijo" {
  byte_length = 2
}

resource "google_sql_database_instance" "pg" {
  name             = "${var.nombre}-${random_id.sufijo.hex}"
  region           = var.region
  database_version = "POSTGRES_17"

  deletion_protection = var.proteger_borrado

  settings {
    # PostgreSQL 17 crea Enterprise Plus si no se dice nada, y ese no admite
    # los tamaños pequeños.
    edition           = "ENTERPRISE"
    tier              = var.tier
    availability_type = "ZONAL"

    disk_type = "PD_SSD"
    disk_size = var.disco_gb
    # Fijo: la configuración no cambia a mitad de una corrida de carga.
    disk_autoresize = false

    location_preference {
      zone = var.zona
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = var.red_id
      # Cifrado en tránsito aunque el tráfico no salga de la red privada.
      # DATABASE_URL lleva sslmode=require.
      ssl_mode = "ENCRYPTED_ONLY"
    }

    # Copias diarias: son la vía para borrar la instancia al final sin perder
    # la evidencia. Sin recuperación a un punto en el tiempo, que el
    # enunciado no pide y cobra el almacenamiento de los registros.
    backup_configuration {
      enabled                        = true
      start_time                     = "08:00" # 03:00 en Colombia
      point_in_time_recovery_enabled = false

      backup_retention_settings {
        retained_backups = 7
      }
    }

    # Las copias sobreviven al borrado de la instancia: C2 la elimina tras
    # registrar las evidencias, y la reconstrucción parte de ellas.
    retain_backups_on_delete = true

    # Mantenimiento en la madrugada del domingo, fuera de las corridas.
    maintenance_window {
      day          = 7
      hour         = 8
      update_track = "stable"
    }

    # Latencia por consulta sin instalar nada: es evidencia para el análisis
    # del cuello de botella.
    insights_config {
      query_insights_enabled = true
    }
  }

  depends_on = [var.conexion_servicios]
}

resource "google_sql_database" "app" {
  name     = var.base
  instance = google_sql_database_instance.pg.name
}

# --- Credencial ---------------------------------------------------------------
# La clave se genera aquí y se escribe en dos sitios —el usuario y el secreto
# de DATABASE_URL— con atributos de solo escritura. Terraform no la guarda en
# el estado ni en el plan: un secreto en el estado es un secreto en claro
# dentro de un bucket (ADR-0015, D5).
#
# Para rotarla se sube version_clave: ambos se reescriben en la misma pasada
# con la misma clave nueva.

ephemeral "random_password" "app" {
  length  = 32
  special = false # va dentro de una URL
}

resource "google_sql_user" "app" {
  name     = var.usuario
  instance = google_sql_database_instance.pg.name

  password_wo         = ephemeral.random_password.app.result
  password_wo_version = var.version_clave
}

resource "google_secret_manager_secret_version" "database_url" {
  secret = var.secreto_database_url

  secret_data_wo = format(
    "postgres://%s:%s@%s:5432/%s?sslmode=require",
    google_sql_user.app.name,
    ephemeral.random_password.app.result,
    google_sql_database_instance.pg.private_ip_address,
    google_sql_database.app.name,
  )
  secret_data_wo_version = var.version_clave

  # Si la instancia o el usuario se recrean, la clave y la IP cambian, y el
  # secreto tiene que escribirse de nuevo en la misma pasada.
  lifecycle {
    replace_triggered_by = [google_sql_user.app]
  }
}
