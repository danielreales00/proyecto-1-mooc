# La red `default` no existe en este proyecto, y es a propósito.
#
# Habilitar Compute Engine la crea sola, con SSH (22) y RDP (3389) abiertos a
# 0.0.0.0/0 y todo abierto dentro de 10.128.0.0/9. No aloja nada nuestro, pero
# contradice que el Web Server sea el único punto público, y quien evalúa la
# red la ve en `gcloud compute firewall-rules list`.
#
# Se borró con Terraform el 27-09-2026 en dos pasos, porque Terraform no puede
# destruir lo que no está en su estado:
#
#   1. Declarar google_compute_network.default y sus cuatro reglas con bloques
#      `import`, y aplicar comprobando que el plan solo importe (0 a cambiar).
#   2. Quitar esos bloques y aplicar: el plan destruye las cinco.
#
# La reconstrucción de C2 usa el mismo proyecto y no necesita repetirlo. En un
# proyecto nuevo sí; el paso 1 es esto, con los atributos que tenga la red:
#
#   import {
#     to = google_compute_network.default
#     id = "projects/<proyecto>/global/networks/default"
#   }
#   resource "google_compute_network" "default" {
#     name                    = "default"
#     description             = "Default network for the project"
#     auto_create_subnetworks = true
#   }
#
# y un import + google_compute_firewall por cada regla default-allow-{icmp,
# internal,rdp,ssh}, con network = google_compute_network.default.name para
# que se destruyan antes que la red.
