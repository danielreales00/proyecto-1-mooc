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

variable "ip_interna_web" {
  type = string
}

variable "ip_interna_worker" {
  description = "Dirección de Redis para el Web Server (REDIS_ADDR)."
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
  description = "Tipo de las dos máquinas: 2 vCPU y 2 GiB, lo que fija el enunciado."
  type        = string
}

variable "imagen_maquinas" {
  type = string
}

variable "disco_maquinas_gb" {
  type = number
}

variable "version_imagenes" {
  description = "SHA con que make publicar etiquetó las imágenes."
  type        = string
}

variable "db_max_conns" {
  description = "Pool por proceso. Tres procesos (api, worker, worker-media) contra el límite de la instancia."
  type        = number
}

variable "nombre_publico" {
  description = "Dominio propio. Vacío usa <ip>.sslip.io (ADR-0016, D4)."
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
  description = "RATE_LIMIT_IP_FACTOR del Web Server. 1 en producción; más solo para pruebas de carga."
  type        = number
  default     = 1
}
