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

variable "ip_interna_web" {
  type = string
}

variable "ip_interna_worker" {
  type = string
}

variable "cuenta_web" {
  description = "Correo de la cuenta de servicio del Web Server."
  type        = string
}

variable "cuenta_worker" {
  description = "Correo de la cuenta de servicio del Worker Server."
  type        = string
}
