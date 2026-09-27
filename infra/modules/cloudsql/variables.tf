variable "nombre" {
  description = "Prefijo del nombre de la instancia; lleva un sufijo aleatorio."
  type        = string
}

variable "region" {
  type = string
}

variable "zona" {
  type = string
}

variable "tier" {
  description = "Tamaño de cómputo. Se registra en el informe de capacidad."
  type        = string
}

variable "encendida" {
  description = "false detiene la instancia sin borrarla."
  type        = bool
}

variable "disco_gb" {
  type = number
}

variable "red_id" {
  description = "VPC con la que Cloud SQL hace peering."
  type        = string
}

variable "conexion_servicios" {
  description = "Id del peering de servicios. Solo sirve para ordenar la creación."
  type        = string
}

variable "base" {
  type = string
}

variable "usuario" {
  type = string
}

variable "version_clave" {
  description = "Subirla rota la clave del usuario y reescribe DATABASE_URL."
  type        = number
}

variable "secreto_database_url" {
  description = "Id del secreto de Secret Manager donde se escribe DATABASE_URL."
  type        = string
}

variable "proteger_borrado" {
  description = "Impide que Terraform borre la instancia. Se apaga a propósito en C2."
  type        = bool
}
