# El estado vive en el bucket creado a mano una sola vez (ADR-0015, D5).
#
# El prefijo entrega4 nace como copia del estado de entrega3 (ADR-0020, como
# hizo ADR-0019 con entrega2): la red, la base, los buckets, los secretos, el
# registro y la cola pasan a este entorno sin recrearse. Desde esa copia,
# entrega3 queda congelado y `make tf` lo rechaza.
terraform {
  backend "gcs" {
    bucket = "mooc-tfstate-mooc-509602"
    prefix = "entrega4"
  }
}
