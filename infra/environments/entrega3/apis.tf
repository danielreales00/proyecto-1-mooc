# APIs del proyecto.
#
# disable_on_destroy = false no es opcional: sin él, un `terraform destroy`
# deshabilita las APIs, y con ellas desaparece lo que dependía de ellas aunque
# no lo gestione Terraform. El procedimiento de reconstrucción de C2 destruye y
# vuelve a levantar el entorno; tiene que encontrar el proyecto intacto.
locals {
  apis = toset([
    "compute.googleapis.com",
    "sqladmin.googleapis.com",
    # Acceso privado a servicios: sin él Cloud SQL no tiene IP privada.
    "servicenetworking.googleapis.com",
    "storage.googleapis.com",
    "secretmanager.googleapis.com",
    "artifactregistry.googleapis.com",
    # Firma de URL con la identidad de la máquina, sin clave descargada.
    "iamcredentials.googleapis.com",
    # Crear las cuentas de servicio de las máquinas.
    "iam.googleapis.com",
    "monitoring.googleapis.com",
    "logging.googleapis.com",
    # SSH a las máquinas por túnel de IAP, sin abrir el 22 a Internet.
    "iap.googleapis.com",
    "billingbudgets.googleapis.com",
  ])
}

# El proveedor comprueba el estado de cada API a través de Cloud Resource
# Manager. Si se habilita en el mismo lote que las demás, las que terminan
# después la encuentran aún deshabilitada y fallan con 403. Va primero.
resource "google_project_service" "base" {
  service            = "cloudresourcemanager.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "apis" {
  for_each = local.apis

  service            = each.value
  disable_on_destroy = false

  depends_on = [google_project_service.base]
}

moved {
  from = google_project_service.apis["cloudresourcemanager.googleapis.com"]
  to   = google_project_service.base
}
