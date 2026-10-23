# Un grupo elástico de máquinas (ADR-0019, D2 y D5): plantilla, grupo
# administrado zonal, verificación de vida para la autorreparación y
# autoscaler. El entorno lo usa dos veces, para la API y para los workers, con
# señales de escalado distintas.
#
# Cada instancia arranca sola con deploy/arranque.sh, que llega por los
# metadatos de la plantilla junto con la composición y la configuración sin
# secretos. Nada se ajusta a mano después de crearla.

locals {
  # Encendido sin autoscaler es el modo de puesta en marcha: una instancia
  # fija para probar balanceador y arranque antes de dejar decidir al
  # autoscaler. Con autoscaler el tamaño lo gobierna él y Terraform no lo toca.
  tamano = !var.encendido ? 0 : (var.autoescalado ? null : var.minimo)
}

resource "google_compute_instance_template" "plantilla" {
  # Una plantilla es inmutable: cada cambio crea otra con nombre nuevo y el
  # grupo pasa a ella antes de que se borre la anterior.
  name_prefix  = "${var.nombre}-"
  machine_type = var.tipo_maquina

  disk {
    source_image = var.imagen
    disk_size_gb = var.disco_gb
    disk_type    = "pd-balanced"
    boot         = true
    auto_delete  = true
  }

  # Sin IP externa: el balanceador es la única entrada y la salida va por
  # Cloud NAT.
  network_interface {
    subnetwork = var.subred_id
  }

  service_account {
    email  = var.cuenta
    scopes = ["cloud-platform"]
  }

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  # El script de apagado solo si el papel lo tiene. Al retirar la instancia,
  # GCE lo ejecuta antes de apagarla: detiene los contenedores con el tiempo
  # de gracia de cada uno (ADR-0019, D6).
  metadata = merge(var.archivos, {
    mooc-rol       = var.rol
    mooc-env       = var.env
    startup-script = var.arranque
    enable-oslogin = "TRUE"
  }, { for clave, valor in { shutdown-script = var.apagado } : clave => valor if valor != "" })

  # La versión de las imágenes queda en cada instancia: así se comprueba que
  # todas las de una corrida ejecutan el mismo código.
  labels = {
    rol     = var.rol
    version = var.version_imagenes
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Vida, no preparación: si cae Redis o la base, /readyz falla en todas las
# instancias a la vez y recrearlas no arregla nada. La autorreparación solo
# reemplaza una instancia cuyo proceso dejó de contestar.
resource "google_compute_health_check" "vida" {
  name                = "${var.nombre}-vida"
  check_interval_sec  = 10
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 3

  http_health_check {
    port         = var.salud_vida.puerto
    request_path = var.salud_vida.ruta
  }
}

resource "google_compute_instance_group_manager" "grupo" {
  name               = var.nombre
  zone               = var.zona
  base_instance_name = var.nombre

  version {
    name              = var.version_imagenes
    instance_template = google_compute_instance_template.plantilla.self_link_unique
  }

  target_size = local.tamano

  dynamic "named_port" {
    for_each = var.puerto_nombrado == null ? [] : [var.puerto_nombrado]
    content {
      name = named_port.value.nombre
      port = named_port.value.puerto
    }
  }

  auto_healing_policies {
    health_check = google_compute_health_check.vida.id
    # Más que el arranque completo: antes de eso /healthz aún no responde y la
    # instancia se recrearía en bucle.
    initial_delay_sec = var.retraso_autorreparacion_s
  }

  # Una versión nueva reemplaza instancias una a una, creando primero la nueva:
  # nunca hay menos capacidad que antes y, al terminar, todas las instancias
  # del grupo corren la misma plantilla.
  update_policy {
    type                  = "PROACTIVE"
    minimal_action        = "REPLACE"
    replacement_method    = "SUBSTITUTE"
    max_surge_fixed       = 1
    max_unavailable_fixed = 0
  }

  wait_for_instances = false
}

resource "google_compute_autoscaler" "autoscaler" {
  count = var.encendido && var.autoescalado ? 1 : 0

  name   = var.nombre
  zone   = var.zona
  target = google_compute_instance_group_manager.grupo.id

  autoscaling_policy {
    min_replicas = var.minimo
    max_replicas = var.maximo
    mode         = "ON"

    # Periodo de inicialización: mientras una instancia nueva arranca, su
    # señal no cuenta. Se fija con el tiempo de arranque medido.
    cooldown_period = var.inicializacion_s

    # API: utilización del balanceador contra max_rate_per_instance.
    dynamic "load_balancing_utilization" {
      for_each = var.objetivo_balanceador == null ? [] : [var.objetivo_balanceador]
      content {
        target = load_balancing_utilization.value
      }
    }

    # Workers: métrica de grupo con trabajos asignados por instancia. El
    # tamaño recomendado es la métrica dividida por la asignación.
    dynamic "metric" {
      for_each = var.metrica == null ? [] : [var.metrica]
      content {
        name                       = metric.value.nombre
        filter                     = metric.value.filtro
        single_instance_assignment = metric.value.por_instancia
      }
    }

    # Nunca retira más de una instancia por ventana: una caída breve de la
    # demanda no vacía el grupo de golpe.
    scale_in_control {
      max_scaled_in_replicas {
        fixed = 1
      }
      time_window_sec = var.ventana_reduccion_s
    }
  }
}
