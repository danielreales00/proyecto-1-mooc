variable "nombre" {
  description = "Nombre del grupo, de su plantilla y prefijo de sus instancias (mooc-api, mooc-worker)."
  type        = string
}

variable "rol" {
  description = "Papel que lee deploy/arranque.sh: web o worker."
  type        = string
}

variable "zona" {
  type = string
}

variable "tipo_maquina" {
  type = string
}

variable "imagen" {
  description = "Imagen de arranque: debian-cloud/debian-12 o la familia horneada mooc-base."
  type        = string
}

variable "disco_gb" {
  type = number
}

variable "subred_id" {
  type = string
}

variable "cuenta" {
  description = "Cuenta de servicio que heredan las instancias."
  type        = string
}

variable "version_imagenes" {
  description = "SHA de las imágenes de contenedor. Etiqueta las instancias y nombra la versión del grupo."
  type        = string
}

variable "archivos" {
  description = "Metadatos con archivos de configuración (composición, Caddyfile, clamd.conf)."
  type        = map(string)
}

variable "env" {
  description = "Configuración sin secretos que el arranque escribe en el .env."
  type        = string
}

variable "arranque" {
  description = "Contenido de deploy/arranque.sh."
  type        = string
}

variable "apagado" {
  description = "Script de apagado. Vacío: ninguno."
  type        = string
  default     = ""
}

variable "puerto_nombrado" {
  description = "Puerto con nombre que usa el servicio de backend del balanceador. null si el grupo no está detrás de uno."
  type = object({
    nombre = string
    puerto = number
  })
  default = null
}

variable "salud_vida" {
  description = "Endpoint de vida para la autorreparación."
  type = object({
    puerto = number
    ruta   = string
  })
}

variable "retraso_autorreparacion_s" {
  description = "Margen tras crear una instancia antes de juzgar su vida. Mayor que el arranque medido."
  type        = number
}

variable "encendido" {
  description = "false deja el grupo en cero instancias y sin autoscaler."
  type        = bool
}

variable "autoescalado" {
  description = "false fija el grupo en `minimo` instancias, para la puesta en marcha."
  type        = bool
}

variable "minimo" {
  type    = number
  default = 1
}

variable "maximo" {
  type    = number
  default = 3
}

variable "inicializacion_s" {
  description = "Periodo de inicialización del autoscaler (cooldown_period)."
  type        = number
}

variable "ventana_reduccion_s" {
  description = "Ventana de scale_in_control: como mucho una instancia retirada por ventana."
  type        = number
}

variable "objetivo_balanceador" {
  description = "Utilización objetivo del balanceador (0–1). Solo el grupo de API."
  type        = number
  default     = null
}

variable "metrica" {
  description = "Métrica de grupo de Cloud Monitoring. Solo el grupo de workers."
  type = object({
    nombre        = string
    filtro        = string
    por_instancia = number
  })
  default = null
}
