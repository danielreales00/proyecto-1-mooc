# Las dos máquinas de la Entrega 2 (ADR-0016, D1): Web Server y Worker Server,
# con la configuración que fija el enunciado. Cada una levanta su composición
# con deploy/arranque.sh, que llega por los metadatos junto con todo lo que
# necesita. Nada se construye en la máquina.

locals {
  maquinas = {
    web = {
      nombre     = "mooc-web"
      cuenta     = var.cuenta_web
      ip_interna = var.ip_interna_web
      ip_externa = var.ip_externa_web
      archivos = {
        mooc-compose   = var.compose_web
        mooc-caddyfile = var.caddyfile
      }
      env = var.env_web
    }
    worker = {
      nombre     = "mooc-worker"
      cuenta     = var.cuenta_worker
      ip_interna = var.ip_interna_worker
      ip_externa = null
      archivos = {
        mooc-compose = var.compose_worker
        mooc-clamd   = var.clamd_conf
      }
      env = var.env_worker
    }
  }
}

resource "google_compute_instance" "m" {
  for_each = local.maquinas

  name         = each.value.nombre
  zone         = var.zona
  machine_type = var.tipo_maquina

  # Cambiar el tipo o la cuenta exige detenerla. Se permite: la alternativa es
  # recrearla y perder el disco.
  allow_stopping_for_update = true

  boot_disk {
    initialize_params {
      image = var.imagen
      size  = var.disco_gb
      type  = "pd-balanced"
    }
  }

  network_interface {
    subnetwork = var.subred_id
    network_ip = each.value.ip_interna

    # Solo el Web Server tiene IP externa. El Worker Server sale por Cloud NAT
    # y no es alcanzable desde Internet.
    dynamic "access_config" {
      for_each = each.value.ip_externa == null ? [] : [each.value.ip_externa]
      content {
        nat_ip = access_config.value
      }
    }
  }

  service_account {
    email = each.value.cuenta
    # El alcance amplio es lo que recomienda Google: lo que la máquina puede
    # hacer lo limita el IAM de la cuenta, no el alcance.
    scopes = ["cloud-platform"]
  }

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  metadata = merge(each.value.archivos, {
    mooc-rol       = each.key
    mooc-env       = each.value.env
    startup-script = var.arranque
    # SSH con la identidad de Google de quien entra, por el túnel de IAP.
    enable-oslogin = "TRUE"
  })

  labels = {
    rol = each.key
  }
}
