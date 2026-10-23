variable "project_id" {
  description = "Proyecto de GCP donde vive el entorno."
  type        = string
}

variable "region" {
  description = "Región de todos los recursos regionales."
  type        = string
}

variable "zona" {
  description = "Zona única del entorno: máquinas y Cloud SQL en la misma, sin alta disponibilidad."
  type        = string
}

variable "cuenta_facturacion" {
  description = "Cuenta de facturación enlazada al proyecto. El presupuesto se crea sobre ella."
  type        = string
}

variable "presupuesto_usd" {
  description = "Tope mensual del presupuesto, en la moneda de la cuenta (USD)."
  type        = number
}

variable "integrantes" {
  description = "Personas del equipo (user:correo) que operan el proyecto con su propia cuenta: Terraform, consola, SSH por IAP y facturación."
  type        = list(string)
  default     = []
}

variable "firmantes_desarrollo" {
  description = "Personas (user:correo) que firman URL como mooc-web desde su máquina, para probar el adaptador de GCS."
  type        = list(string)
  default     = []
}

variable "cidr_subred" {
  description = "Subred de las máquinas."
  type        = string
}

variable "cidr_servicios" {
  description = "Rango reservado para el peering con Cloud SQL."
  type        = string
}

variable "ip_interna_cola" {
  description = "Dirección fija de la máquina de la cola: REDIS_ADDR y SMTP_ADDR de los dos grupos."
  type        = string
}

variable "cloudsql_tier" {
  description = "Tamaño de cómputo de Cloud SQL. Se registra en el informe."
  type        = string
}

variable "cloudsql_encendida" {
  description = "false detiene Cloud SQL sin borrarla. Se cambia en terraform.tfvars, nunca con -var."
  type        = bool
}

variable "cloudsql_disco_gb" {
  type = number
}

variable "version_clave_db" {
  description = "Subirla rota la clave de la base y reescribe DATABASE_URL."
  type        = number
  default     = 1
}

variable "proteger_borrado" {
  description = "Protege Cloud SQL y los buckets con objetos frente a destroy. C2 lo apaga a propósito."
  type        = bool
  default     = true
}

variable "tipo_maquina" {
  description = "Tipo de las instancias de los dos grupos: el de la Entrega 2, para que la capacidad por instancia sea comparable."
  type        = string
}

variable "imagen_maquinas" {
  description = "Imagen de la cola y del generador."
  type        = string
}

variable "imagen_grupos" {
  description = "Imagen de las plantillas: debian-cloud/debian-12 en la puesta en marcha, la familia horneada mooc-base en las corridas medidas."
  type        = string
}

variable "tipo_cola" {
  type = string
}

variable "disco_cola_gb" {
  type = number
}

variable "disco_maquinas_gb" {
  type = number
}

variable "version_imagenes" {
  description = "SHA con que make publicar etiquetó las imágenes."
  type        = string
}

variable "db_max_conns" {
  description = "Pool de cada proceso worker. La API tiene el suyo (db_max_conns_api). Se multiplica por el máximo de instancias."
  type        = number
}

variable "nombre_publico" {
  description = "Dominio propio. Vacío usa <ip del balanceador>.sslip.io (ADR-0019, D3)."
  type        = string
  default     = ""
}

variable "generador_encendido" {
  description = "Crea la máquina del generador de carga. false la borra."
  type        = bool
  default     = false
}

variable "tipo_generador" {
  type    = string
  default = "e2-standard-4"
}

variable "factor_limite_ip" {
  description = "RATE_LIMIT_IP_FACTOR de la API. 1 en producción; más solo para pruebas de carga."
  type        = number
  default     = 1
}

variable "db_max_conns_api" {
  description = "Pool de la API. Separado del de los workers: es el que atiende el tráfico síncrono."
  type        = number
}

variable "entorno_encendido" {
  description = "false deja los grupos en cero, detiene la cola y borra las reglas de reenvío. Se cambia en terraform.tfvars."
  type        = bool
}

# --- Escalado (ADR-0019, D4 y D5) ---------------------------------------------

variable "autoescalado" {
  description = "false fija cada grupo en una instancia, sin autoscaler: solo para la puesta en marcha."
  type        = bool
}

variable "rps_por_instancia_api" {
  description = "max_rate_per_instance del balanceador: peticiones por segundo de una instancia de API a plena utilización."
  type        = number
}

variable "objetivo_balanceador_api" {
  description = "Utilización objetivo del balanceador (0–1) que mantiene el autoscaler de la API."
  type        = number
}

variable "trabajos_por_worker" {
  description = "Trabajos de bulk (pendientes más en curso) asignados a cada instancia de worker."
  type        = number
}

variable "inicializacion_api_s" {
  description = "Periodo de inicialización del autoscaler de la API: arranque medido hasta pasar /readyz."
  type        = number
}

variable "inicializacion_workers_s" {
  description = "Periodo de inicialización del autoscaler de workers: arranque medido hasta consumir trabajos."
  type        = number
}

variable "ventana_reduccion_s" {
  description = "Ventana de scale_in_control de los dos grupos: como mucho una instancia retirada por ventana."
  type        = number
}

variable "retraso_autorreparacion_s" {
  description = "Margen antes de que la autorreparación juzgue una instancia nueva."
  type        = number
}
