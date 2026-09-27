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
