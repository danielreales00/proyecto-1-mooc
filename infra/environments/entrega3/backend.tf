# El estado vive en el bucket creado a mano una sola vez (ADR-0015, D5): es la
# única excepción a «todo por Terraform», porque Terraform no puede crear el
# sitio donde guarda su propio estado. El bucket tiene versionado, así que un
# estado corrupto se recupera de la versión anterior.
#
# El prefijo entrega3 nace como copia del estado de entrega2 (ADR-0018, D1):
# la base, los buckets, los secretos y el registro pasan a este entorno sin
# recrearse. Desde esa copia, entrega2 queda congelado y `make tf` lo rechaza.
terraform {
  backend "gcs" {
    bucket = "mooc-tfstate-mooc-509602"
    prefix = "entrega3"
  }
}
