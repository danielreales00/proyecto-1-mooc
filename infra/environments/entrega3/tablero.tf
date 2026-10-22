# Tablero del ciclo de escalado (ADR-0018, D8): en una sola pantalla, la
# demanda, la señal de cada autoscaler, las instancias de cada grupo y su
# efecto en latencia. Es la línea de tiempo que piden los escenarios 3A y 3B, y
# lo que se muestra en el video.
#
# Ojo con queue_depth de los workers: cada instancia exporta la misma cola, así
# que con tres instancias hay tres series iguales. Se agrega con max, nunca con
# sum. La señal del autoscaler (cola_bulk_trabajos) no tiene ese problema: la
# publica solo la cola.

locals {
  tablero_cpu = {
    for rol, titulo in { web = "CPU de las instancias de API", worker = "CPU de las instancias de workers" } :
    rol => {
      title = titulo
      xyChart = {
        dataSets = [{
          plotType = "LINE"
          timeSeriesQuery = {
            timeSeriesFilter = {
              filter = "metric.type=\"compute.googleapis.com/instance/cpu/utilization\" resource.type=\"gce_instance\" metadata.user_labels.\"rol\"=\"${rol}\""
              aggregation = {
                alignmentPeriod  = "60s"
                perSeriesAligner = "ALIGN_MEAN"
              }
            }
          }
        }]
        yAxis = { scale = "LINEAR" }
      }
    }
  }
}

resource "google_monitoring_dashboard" "escalado" {
  dashboard_json = jsonencode({
    displayName = "MOOC — ciclo de escalado (Entrega 3)"
    mosaicLayout = {
      columns = 12
      tiles = [
        {
          xPos = 0, yPos = 0, width = 6, height = 4
          widget = {
            title = "Instancias por grupo"
            xyChart = {
              dataSets = [{
                plotType = "STEP"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"compute.googleapis.com/instance_group/size\" resource.type=\"instance_group\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_MAX"
                      crossSeriesReducer = "REDUCE_MAX"
                      groupByFields      = ["resource.label.\"instance_group_name\""]
                    }
                  }
                }
              }]
              yAxis = { scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 6, yPos = 0, width = 6, height = 4
          widget = {
            title = "Señal de workers: trabajos de bulk pendientes + en curso"
            xyChart = {
              dataSets = [{
                plotType = "LINE"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"custom.googleapis.com/mooc/cola_bulk_trabajos\" resource.type=\"global\""
                    aggregation = {
                      alignmentPeriod  = "60s"
                      perSeriesAligner = "ALIGN_MAX"
                    }
                  }
                }
              }]
              thresholds = [{ value = var.trabajos_por_worker, label = "asignación por instancia" }]
              yAxis      = { scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 0, yPos = 4, width = 6, height = 4
          widget = {
            title = "Peticiones por segundo en el balanceador, por código"
            xyChart = {
              dataSets = [{
                plotType = "STACKED_AREA"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"loadbalancing.googleapis.com/https/request_count\" resource.type=\"https_lb_rule\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["metric.label.\"response_code_class\""]
                    }
                  }
                }
              }]
              yAxis = { scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 6, yPos = 4, width = 6, height = 4
          widget = {
            title = "Latencia total en el balanceador, p95 (ms)"
            xyChart = {
              dataSets = [{
                plotType = "LINE"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"loadbalancing.googleapis.com/https/total_latencies\" resource.type=\"https_lb_rule\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_DELTA"
                      crossSeriesReducer = "REDUCE_PERCENTILE_95"
                    }
                  }
                }
              }]
              yAxis = { scale = "LINEAR" }
            }
          }
        },
        {
          xPos   = 0, yPos = 8, width = 6, height = 4
          widget = local.tablero_cpu["web"]
        },
        {
          xPos   = 6, yPos = 8, width = 6, height = 4
          widget = local.tablero_cpu["worker"]
        },
      ]
    }
  })

  depends_on = [google_project_service.apis]
}
