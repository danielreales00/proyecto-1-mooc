variable "prefijo" {
  description = "Prefijo de los nombres de bucket, que son globales en GCS."
  type        = string
}

variable "region" {
  type = string
}

variable "borrar_con_objetos" {
  description = "Permite a destroy borrar buckets con objetos dentro."
  type        = bool
}
