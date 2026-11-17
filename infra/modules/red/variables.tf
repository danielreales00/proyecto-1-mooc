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

# La salida a Internet solo hace falta cuando hay maquinas encendidas: las usan
# para bajar imagenes de Docker Hub, paquetes y el repositorio del agente. Las
# APIs de Google no pasan por aqui —la subred tiene acceso privado— y Cloud Run
# sale con `private-ranges-only`, sin enrutar Internet por la VPC.
#
# Cloud NAT cobra por hora y por pasarela, haya trafico o no, del orden de 30
# USD al mes. Con el entorno apagado eso es gasto puro.
variable "salida_encendida" {
  type    = bool
  default = true
}
