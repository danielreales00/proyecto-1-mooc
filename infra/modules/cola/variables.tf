variable "nombre" {
  type    = string
  default = "mooc-cola"
}

variable "zona" {
  type = string
}

variable "tipo_maquina" {
  description = "Redis y Mailpit caben de sobra en e2-small (2 vCPU compartidas, 2 GiB)."
  type        = string
}

variable "imagen" {
  type = string
}

variable "disco_gb" {
  type = number
}

variable "encendida" {
  description = "false la detiene sin borrarla; el AOF de Redis se conserva en el disco."
  type        = bool
}

variable "subred_id" {
  type = string
}

variable "ip_interna" {
  description = "Dirección fija: REDIS_ADDR y SMTP_ADDR de las plantillas apuntan aquí."
  type        = string
}

variable "cuenta" {
  description = "Cuenta de servicio de la máquina (mooc-cola)."
  type        = string
}

variable "env" {
  description = "Configuración sin secretos que el arranque escribe en el .env."
  type        = string
}

variable "compose" {
  description = "Contenido de deploy/compose.cola.yml."
  type        = string
}

variable "publicador" {
  description = "Contenido de deploy/publicar-cola.sh."
  type        = string
}

variable "arranque" {
  description = "Contenido de deploy/arranque.sh."
  type        = string
}
