# El estado vive en el bucket creado a mano una sola vez (ADR-0015, D5).
#
# El prefijo entrega5 nace como copia del estado de entrega4 (ADR-0021, como
# hicieron ADR-0019 y ADR-0020): Cloud Run, Pub/Sub, la red, la base, los
# buckets, los secretos, el registro y la cola pasan a este entorno sin
# recrearse. Desde esa copia, entrega4 queda congelado y `make tf` lo rechaza.
terraform {
  backend "gcs" {
    bucket = "mooc-tfstate-mooc-509602"
    prefix = "entrega5"
  }
}
