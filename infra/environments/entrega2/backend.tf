# El estado vive en el bucket creado a mano una sola vez (ADR-0015, D5): es la
# única excepción a «todo por Terraform», porque Terraform no puede crear el
# sitio donde guarda su propio estado. El bucket tiene versionado, así que un
# estado corrupto se recupera de la versión anterior.
terraform {
  backend "gcs" {
    bucket = "mooc-tfstate-mooc-509602"
    prefix = "entrega2"
  }
}
