# La cola compartida, fuera de los dos grupos elásticos (ADR-0019, D2). El
# enunciado lo exige: su disponibilidad no puede depender de una máquina que el
# autoscaling pueda retirar. Es una máquina suelta, con dirección interna fija,
# que ningún grupo gestiona.
#
# Lleva Redis (cola, sesiones, caché y límites), Mailpit y el publicador de la
# señal de escalado de los workers. Arranca con el mismo deploy/arranque.sh que
# las plantillas, con el papel `cola`.

resource "google_compute_instance" "cola" {
  name         = var.nombre
  zone         = var.zona
  machine_type = var.tipo_maquina

  allow_stopping_for_update = true

  # Detenida entre sesiones de trabajo: sin cómputo, solo disco. El AOF de
  # Redis queda en el disco y la cola sobrevive a la parada.
  desired_status = var.encendida ? "RUNNING" : "TERMINATED"

  boot_disk {
    initialize_params {
      image = var.imagen
      size  = var.disco_gb
      type  = "pd-balanced"
    }
  }

  # Sin IP externa: sale por Cloud NAT y nadie la alcanza desde Internet.
  network_interface {
    subnetwork = var.subred_id
    network_ip = var.ip_interna
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

  metadata = {
    mooc-rol        = "cola"
    mooc-ip-privada = var.ip_interna
    mooc-env        = var.env
    mooc-compose    = var.compose
    mooc-publicador = var.publicador
    startup-script  = var.arranque
    enable-oslogin  = "TRUE"
    # Solo OS Login: ninguna clave SSH de proyecto entra en la máquina.
    block-project-ssh-keys = "TRUE"
  }

  labels = {
    rol = "cola"
  }
}
