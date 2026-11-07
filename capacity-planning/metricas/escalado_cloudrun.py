#!/usr/bin/env python3
"""Línea de tiempo del escalamiento en Cloud Run y Pub/Sub (Entrega 4).

Consulta Cloud Monitoring con PromQL en la ventana de una prueba y escribe una
tabla por minuto: instancias de cada servicio, peticiones por segundo y
mensajes pendientes de la suscripción de los workers. Es la evidencia de
«número de instancias ante solicitudes concurrentes y mensajes pendientes».

Uso (dentro de la imagen de gcloud, con make escalado-cloudrun):
  escalado_cloudrun.py DESDE HASTA        # fechas ISO 8601 en UTC
"""
import sys
from datetime import datetime, timezone

from exportar import consultar, token, ts

SERVICIOS = ("mooc-frontend", "mooc-backend", "mooc-workers")

CONSULTAS = {
    "instancias": 'sum by (service_name) (run_googleapis_com:container_instance_count{monitored_resource="cloud_run_revision"})',
    "peticiones_s": 'sum by (service_name) (rate(run_googleapis_com:request_count{monitored_resource="cloud_run_revision"}[1m]))',
    "pendientes": 'sum by (subscription_id) (pubsub_googleapis_com:subscription_num_undelivered_messages{subscription_id="mooc-workers"})',
    "antiguedad_s": 'max by (subscription_id) (pubsub_googleapis_com:subscription_oldest_unacked_message_age{subscription_id="mooc-workers"})',
}


def por_minuto(series, etiqueta):
    """{minuto: {valor de la etiqueta: máximo del minuto}}."""
    out = {}
    for s in series:
        clave = s["metric"].get(etiqueta, "")
        for t, v in s["values"]:
            if v in ("NaN", "+Inf", "-Inf"):
                continue
            minuto = datetime.fromtimestamp(float(t), timezone.utc).strftime("%H:%M")
            celda = out.setdefault(minuto, {})
            celda[clave] = max(celda.get(clave, 0.0), float(v))
    return out


def main():
    desde, hasta = ts(sys.argv[1]), ts(sys.argv[2])
    t = token()
    inst = por_minuto(consultar(t, CONSULTAS["instancias"], desde, hasta), "service_name")
    pet = por_minuto(consultar(t, CONSULTAS["peticiones_s"], desde, hasta), "service_name")
    pend = por_minuto(consultar(t, CONSULTAS["pendientes"], desde, hasta), "subscription_id")
    edad = por_minuto(consultar(t, CONSULTAS["antiguedad_s"], desde, hasta), "subscription_id")

    print("| UTC | Instancias frontend / backend / workers | Peticiones/s frontend / backend / workers | Mensajes pendientes | Antigüedad del más viejo (s) |")
    print("| --- | --- | --- | --- | --- |")
    for minuto in sorted(set(inst) | set(pet) | set(pend)):
        i = " / ".join(f"{inst.get(minuto, {}).get(s, 0):.0f}" for s in SERVICIOS)
        p = " / ".join(f"{pet.get(minuto, {}).get(s, 0):.1f}" for s in SERVICIOS)
        n = pend.get(minuto, {}).get("mooc-workers")
        a = edad.get(minuto, {}).get("mooc-workers")
        print(f"| {minuto} | {i} | {p} | {'-' if n is None else f'{n:.0f}'} | {'-' if a is None else f'{a:.0f}'} |")


if __name__ == "__main__":
    main()
