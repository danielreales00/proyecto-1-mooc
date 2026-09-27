variable "region" {
  type = string
}

variable "secretos" {
  description = "Nombre de cada secreto y los correos de las cuentas de servicio que lo leen."
  type        = map(list(string))
}
