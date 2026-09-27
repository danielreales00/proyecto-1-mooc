variable "nombre" {
  type = string
}

variable "region" {
  type = string
}

variable "lectores" {
  description = "Correos de las cuentas de servicio que descargan imágenes."
  type        = list(string)
}
