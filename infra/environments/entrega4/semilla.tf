# Contraseña de las cuentas sintéticas en la nube.
#
# En local la semilla usa una contraseña publicada en el repositorio, y está
# bien: nadie más llega a ese stack. En la URL pública, esa contraseña daría la
# cuenta de administración a cualquiera que lea el repositorio. Aquí se genera
# una propia, que no pasa por el estado ni por el plan (atributos de solo
# escritura, como la de la base), y la semilla y la colección de Postman la
# leen de Secret Manager en el momento.

ephemeral "random_password" "semilla" {
  length  = 24
  special = false
}

resource "google_secret_manager_secret_version" "semilla" {
  secret                 = module.secretos.ids["seed-password"]
  secret_data_wo         = ephemeral.random_password.semilla.result
  secret_data_wo_version = 1
}
