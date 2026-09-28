# El generador de carga existe solo mientras se mide: generador_encendido en
# terraform.tfvars lo crea o lo borra entero.
module "generador" {
  source = "../../modules/generador"
  count  = var.generador_encendido ? 1 : 0

  project_id   = var.project_id
  region       = var.region
  zona         = var.zona
  tipo_maquina = var.tipo_generador
  imagen       = var.imagen_maquinas
  subred_id    = module.red.subred_id
  red_id       = module.red.red_id

  secreto_semilla = module.secretos.ids["seed-password"]
  repositorio     = "mooc"
  arranque        = file("${path.module}/../../../deploy/arranque-generador.sh")

  depends_on = [module.registry]
}
