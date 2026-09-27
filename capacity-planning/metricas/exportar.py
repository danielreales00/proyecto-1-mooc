#!/usr/bin/env python3
"""Exporta las métricas de infraestructura y de aplicación de una corrida.

Consulta Cloud Monitoring con PromQL (su API es compatible con la de
Prometheus) en la ventana de la corrida y escribe, por serie, la media, el
máximo y el último valor. Es la evidencia de «CPU, memoria, red y disco de los
servidores; conexiones y carga de la base; profundidad, antigüedad y tasa de
la cola» que pide el enunciado, junto a lo que mide k6.

Uso (dentro de la imagen de gcloud, con make carga-metricas):
  exportar.py ETIQUETA DESDE HASTA        # fechas ISO 8601 en UTC
"""
import json
import subprocess
import sys
import urllib.parse
import urllib.request
from datetime import datetime

PROYECTO = "mooc-509602"
API = f"https://monitoring.googleapis.com/v1/projects/{PROYECTO}/location/global/prometheus/api/v1/query_range"

# Nombre → consulta PromQL. Las métricas del agente existen para varios tipos
# de recurso y Cloud Monitoring exige nombrar cuál (monitored_resource). Las de sistema vienen del agente de operaciones y
# de Compute Engine; las de la aplicación, de /metrics vía el mismo agente.
CONSULTAS = {
    # Máquinas: las dos de la aplicación y el generador (validez de la corrida).
    "cpu_uso": 'avg by (instance_name) (compute_googleapis_com:instance_cpu_utilization{instance_name=~"mooc-.*"})',
    "memoria_uso_pct": 'avg by (metadata_system_name) (agent_googleapis_com:memory_percent_used{monitored_resource="gce_instance",state="used"})',
    "memoria_disponible_bytes": 'sum by (metadata_system_name) (agent_googleapis_com:memory_bytes_used{monitored_resource="gce_instance",state=~"free|cached|buffered|slab_reclaimable"})',
    "disco_uso_pct": 'max by (metadata_system_name) (agent_googleapis_com:disk_percent_used{monitored_resource="gce_instance",state="used"})',
    "red_recibida_bytes_s": 'sum by (instance_name) (rate(compute_googleapis_com:instance_network_received_bytes_count{instance_name=~"mooc-.*"}[1m]))',
    "red_enviada_bytes_s": 'sum by (instance_name) (rate(compute_googleapis_com:instance_network_sent_bytes_count{instance_name=~"mooc-.*"}[1m]))',
    # Cloud SQL.
    "sql_cpu": 'avg(cloudsql_googleapis_com:database_cpu_utilization)',
    "sql_memoria": 'avg(cloudsql_googleapis_com:database_memory_utilization)',
    "sql_conexiones": 'sum(cloudsql_googleapis_com:database_postgresql_num_backends)',
    "sql_transacciones_s": 'sum(rate(cloudsql_googleapis_com:database_postgresql_transaction_count[1m]))',
    # Cola (B2).
    "cola_profundidad": 'sum by (queue, state) (queue_depth)',
    "cola_antiguedad_s": 'max by (queue) (queue_oldest_pending_age_seconds)',
    "cola_procesados_s": 'sum by (queue) (rate(queue_processed_total[2m]))',
    "cola_fallidos_s": 'sum by (queue) (rate(queue_failed_total[2m]))',
    # Aplicación.
    "api_p95_s": 'histogram_quantile(0.95, sum by (le, route) (rate(http_request_duration_seconds_bucket[2m])))',
    "api_peticiones_s": 'sum by (route) (rate(http_requests_total[2m]))',
    "trabajo_p95_s": 'histogram_quantile(0.95, sum by (le, type) (rate(job_duration_seconds_bucket[5m])))',
    "trabajos_muertos": 'sum by (type) (jobs_dead_letter_total)',
    "limitados_429_s": 'sum by (rule) (rate(rate_limited_total[2m]))',
    # Redis (receptor del agente).
    "redis_clientes": 'max(workload_googleapis_com:redis_clients_connected{monitored_resource="gce_instance"})',
    "redis_comandos_s": 'sum(rate(workload_googleapis_com:redis_commands_processed{monitored_resource="gce_instance"}[1m]))',
    "redis_memoria_bytes": 'max(workload_googleapis_com:redis_memory_used{monitored_resource="gce_instance"})',
}


def token():
    return subprocess.check_output(["gcloud", "auth", "print-access-token"], text=True).strip()


def ts(iso):
    return datetime.fromisoformat(iso.replace("Z", "+00:00")).timestamp()


def consultar(t, q, desde, hasta):
    qs = urllib.parse.urlencode({"query": q, "start": desde, "end": hasta, "step": "30s"})
    req = urllib.request.Request(f"{API}?{qs}", headers={"Authorization": f"Bearer {t}"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.load(r)["data"]["result"]


def main():
    etiqueta, desde, hasta = sys.argv[1], ts(sys.argv[2]), ts(sys.argv[3])
    t = token()
    salida = {"etiqueta": etiqueta, "desde": sys.argv[2], "hasta": sys.argv[3], "metricas": {}}
    for nombre, q in CONSULTAS.items():
        try:
            series = consultar(t, q, desde, hasta)
        except Exception as e:  # una métrica ausente no invalida las demás
            salida["metricas"][nombre] = {"error": str(e)}
            continue
        filas = []
        for s in series:
            vals = [float(v) for _, v in s["values"] if v not in ("NaN", "+Inf", "-Inf")]
            if not vals:
                continue
            filas.append({
                "serie": s["metric"],
                "media": sum(vals) / len(vals), "max": max(vals), "ultimo": vals[-1], "n": len(vals),
                "valores": s["values"],
            })
        salida["metricas"][nombre] = filas
    print(json.dumps(salida))


if __name__ == "__main__":
    main()
