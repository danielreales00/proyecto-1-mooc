variable "nombre" {
  description = "Prefijo de los recursos de red."
  type        = string
}

variable "region" {
  type = string
}

variable "cidr_subred" {
  description = "Rango de la subred de las máquinas."
  type        = string
}

variable "cidr_servicios" {
  description = "Rango reservado para el peering de servicios administrados (Cloud SQL)."
  type        = string
}

variable "ip_interna_cola" {
  description = "Dirección fija de la máquina de la cola (Redis y Mailpit)."
  type        = string
}

variable "cuenta_web" {
  description = "Correo de la cuenta de servicio del grupo de API."
  type        = string
}

variable "cuenta_worker" {
  description = "Correo de la cuenta de servicio del grupo de workers."
  type        = string
}

variable "cuenta_cola" {
  description = "Correo de la cuenta de servicio de la máquina de la cola."
  type        = string
}
