variable "nombre" {
  type    = string
  default = "mooc-lb"
}

variable "nombre_publico" {
  description = "Dominio propio. Vacío usa <ip>.sslip.io."
  type        = string
  default     = ""
}

variable "encendido" {
  description = "false borra las reglas de reenvío, que cobran por hora. La IP y el certificado se quedan."
  type        = bool
}

variable "grupo_instancias" {
  description = "Grupo de instancias de la API (salida grupo_instancias del módulo grupos)."
  type        = string
}

variable "puerto_nombrado" {
  description = "Puerto con nombre del grupo de API donde escucha Caddy."
  type        = string
  default     = "http"
}

variable "rps_por_instancia" {
  description = "max_rate_per_instance: peticiones por segundo que una instancia atiende a plena utilización."
  type        = number
}

variable "timeout_s" {
  description = "Tiempo máximo de respuesta de la API. Las cargas no pasan por aquí: van directas a Cloud Storage."
  type        = number
  default     = 30
}

variable "drenaje_s" {
  description = "Margen de las peticiones en curso al retirar una instancia."
  type        = number
  default     = 30
}

variable "muestreo_registros" {
  description = "Fracción de peticiones registradas (0–1)."
  type        = number
  default     = 1
}

variable "salud" {
  description = "Verificación de preparación del balanceador."
  type = object({
    ruta           = string
    intervalo_s    = number
    timeout_s      = number
    umbral_sano    = number
    umbral_enfermo = number
  })
}
