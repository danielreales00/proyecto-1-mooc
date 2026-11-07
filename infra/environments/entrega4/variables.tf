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

variable "imagen_maquinas" {
  description = "Imagen de la cola y del generador."
  type        = string
}

variable "tipo_cola" {
  type = string
}

variable "disco_cola_gb" {
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


# --- Cloud Run (ADR-0020) -------------------------------------------------------

variable "max_instancias" {
  description = "Máximo de instancias de cada servicio de Cloud Run. La entrega fija 3."
  type        = number
  default     = 3

  validation {
    condition     = var.max_instancias >= 1 && var.max_instancias <= 3
    error_message = "La entrega exige como máximo 3 instancias por servicio."
  }
}

variable "frontend" {
  description = "CPU, memoria, peticiones simultáneas por instancia y tiempo máximo de solicitud del frontend."
  type = object({
    cpu             = string
    memoria         = string
    concurrencia    = number
    tiempo_maximo_s = number
  })
}

variable "backend" {
  description = "Lo mismo para el backend."
  type = object({
    cpu             = string
    memoria         = string
    concurrencia    = number
    tiempo_maximo_s = number
  })
}

variable "workers" {
  description = "Lo mismo para los workers. El tiempo máximo debe igualar el plazo de confirmación de la suscripción."
  type = object({
    cpu             = string
    memoria         = string
    concurrencia    = number
    tiempo_maximo_s = number
  })

  validation {
    condition     = var.workers.tiempo_maximo_s >= 600
    error_message = "Por debajo de 600 s, Cloud Run cortaría una transcodificación antes de que venza el plazo de confirmación."
  }
}

variable "pubsub_max_entregas" {
  description = "Entregas de un mensaje antes de mandarlo al topic de muertos. Pub/Sub admite de 5 a 100."
  type        = number
  default     = 5
}
