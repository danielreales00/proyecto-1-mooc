# La entrada pública de la API y del frontend (ADR-0019, D3).
module "balanceador" {
  source = "../../modules/balanceador"

  nombre_publico    = var.nombre_publico
  encendido         = var.entorno_encendido
  grupo_instancias  = module.api.grupo_instancias
  rps_por_instancia = var.rps_por_instancia_api

  # /readyz a través de Caddy: comprueba base, Redis y objetos. Con estos
  # valores, una instancia sale del reparto unos 15 s después de dejar de
  # atender y vuelve tras dos respuestas sanas.
  salud = {
    ruta           = "/readyz"
    intervalo_s    = 5
    timeout_s      = 5
    umbral_sano    = 2
    umbral_enfermo = 3
  }

  depends_on = [google_project_service.apis]
}
