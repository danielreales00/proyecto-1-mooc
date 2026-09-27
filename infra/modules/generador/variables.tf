variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "zona" {
  type = string
}

variable "tipo_maquina" {
  type = string
}

variable "imagen" {
  type = string
}

variable "subred_id" {
  type = string
}

variable "repositorio" {
  description = "Nombre del repositorio de Artifact Registry."
  type        = string
}

variable "arranque" {
  type = string
}

variable "red_id" {
  type = string
}

variable "secreto_semilla" {
  description = "Id del secreto seed-password."
  type        = string
}
