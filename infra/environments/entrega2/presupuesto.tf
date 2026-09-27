# Presupuesto con alertas al 50, 80 y 100 %, que el enunciado exige.
#
# La cuenta es de educación: el consumo se paga con créditos. Si el
# presupuesto los descontara, el gasto neto sería cero y ninguna alerta
# saltaría nunca. Se miden sin créditos, que es lo que de verdad se consume.
#
# Sin canales propios: avisa por correo a los administradores de la cuenta de
# facturación, que es el comportamiento por defecto.
# Leer el proyecto necesita Cloud Resource Manager, que se habilita en esta
# misma configuración: sin la dependencia, el primer plan falla.
data "google_project" "este" {
  depends_on = [google_project_service.apis]
}

resource "google_billing_budget" "entrega2" {
  billing_account = var.cuenta_facturacion
  display_name    = "mooc-entrega2"

  budget_filter {
    projects               = ["projects/${data.google_project.este.number}"]
    credit_types_treatment = "EXCLUDE_ALL_CREDITS"
  }

  amount {
    specified_amount {
      currency_code = "USD"
      units         = tostring(var.presupuesto_usd)
    }
  }

  dynamic "threshold_rules" {
    for_each = [0.5, 0.8, 1.0]
    content {
      threshold_percent = threshold_rules.value
    }
  }

  depends_on = [google_project_service.apis]
}
