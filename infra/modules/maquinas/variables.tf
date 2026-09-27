variable "zona" {
  type = string
}

variable "tipo_maquina" {
  description = "Tipo de las dos máquinas. El enunciado fija 2 vCPU y 2 GiB."
  type        = string
}

variable "imagen" {
  type = string
}

variable "disco_gb" {
  description = "Disco persistente de cada máquina. El enunciado fija 30 GiB."
  type        = number
}

variable "subred_id" {
  type = string
}

variable "ip_interna_web" {
  type = string
}

variable "ip_interna_worker" {
  type = string
}

variable "ip_externa_web" {
  type = string
}

variable "cuenta_web" {
  type = string
}

variable "cuenta_worker" {
  type = string
}

variable "arranque" {
  description = "Contenido de deploy/arranque.sh."
  type        = string
}

variable "compose_web" {
  type = string
}

variable "compose_worker" {
  type = string
}

variable "caddyfile" {
  type = string
}

variable "clamd_conf" {
  type = string
}

variable "env_web" {
  description = "Variables sin secretos del Web Server, en formato .env."
  type        = string
}

variable "env_worker" {
  description = "Variables sin secretos del Worker Server, en formato .env."
  type        = string
}
