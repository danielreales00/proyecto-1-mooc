# Versiones fijadas de Terraform y de los proveedores (ADR-0015, D5), igual que
# las dependencias de Go (ADR-0002). Cada entorno enlaza este archivo en vez de
# copiarlo: así dos entornos no pueden quedar con versiones distintas.
terraform {
  required_version = "~> 1.16"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.46"
    }
    # Solo para generar valores efímeros (la clave de la base) y sufijos de
    # nombre. Lo efímero existe desde 3.7.
    random = {
      source  = "hashicorp/random"
      version = "~> 3.9"
    }
  }
}
