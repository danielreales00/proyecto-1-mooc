#!/usr/bin/env python3
"""Gráficas del informe de capacidad a partir de los archivos de resultados.

Lee capacity-planning/resultados/ (resúmenes de k6 y métricas de Cloud
Monitoring de cada corrida) y escribe PNG en capacity-planning/graficas/. Nada
se dibuja a mano: si cambian los resultados, se vuelven a generar.

  graficas.py e1 e1-L0 e1-L1 e1-L2 e1-L3   # niveles del escenario 1
  graficas.py serie e1-L3                 # una corrida minuto a minuto
  graficas.py cola e2-M1                  # cola del escenario 2
"""
import json
import os
import sys
from datetime import datetime, timezone

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402

BASE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
RES = os.path.join(BASE, "resultados")
OUT = os.path.join(BASE, "graficas")
os.makedirs(OUT, exist_ok=True)
plt.rcParams.update({"figure.dpi": 110, "font.size": 9, "axes.grid": True, "grid.alpha": 0.3})


def cargar(n):
    p = os.path.join(RES, n)
    return json.load(open(p)) if os.path.exists(p) else None


def k6(d, m, k):
    x = d["metrics"].get(m)
    return x["values"].get(k) if x else None


def met(m, nombre, filtro=None, campo="media"):
    for f in m["metricas"].get(nombre) or []:
        if isinstance(f, dict) and "serie" in f and (not filtro or all(f["serie"].get(a) == b for a, b in filtro.items())):
            return f[campo] if campo != "valores" else f["valores"]
    return None


def guardar(fig, nombre):
    ruta = os.path.join(OUT, nombre)
    fig.tight_layout()
    fig.savefig(ruta)
    plt.close(fig)
    print(os.path.relpath(ruta, BASE))


def e1(etiquetas):
    filas = []
    for et in etiquetas:
        d, m = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
        if not d:
            continue
        dur = "http_req_duration{scenario:medicion}"
        filas.append({
            "et": et.replace("e1-", ""),
            "rps": k6(d, "http_reqs{scenario:medicion}", "rate"),
            "p50": k6(d, dur, "med"), "p95": k6(d, dur, "p(95)"), "p99": k6(d, dur, "p(99)"),
            "web": 100 * (met(m, "cpu_uso", {"instance_name": "mooc-web"}) or 0),
            "wrk": 100 * (met(m, "cpu_uso", {"instance_name": "mooc-worker"}) or 0),
            "sql": 100 * (met(m, "sql_cpu") or 0),
            "gen": 100 * (met(m, "cpu_uso", {"instance_name": "mooc-generador"}, "max") or 0),
            "conex": met(m, "sql_conexiones", None, "max") or 0,
        })

    fig, ax = plt.subplots(figsize=(7, 4))
    x = [f["rps"] for f in filas]
    for k, estilo in (("p50", "o-"), ("p95", "s-"), ("p99", "^-")):
        ax.plot(x, [f[k] for f in filas], estilo, label=k)
    for f in filas:
        ax.annotate(f["et"], (f["rps"], f["p95"]), textcoords="offset points", xytext=(4, 6))
    ax.set_yscale("log")
    ax.set_xlabel("peticiones por segundo medidas")
    ax.set_ylabel("latencia (ms, escala log)")
    ax.set_title("Escenario 1: latencia por nivel de carga")
    ax.legend()
    guardar(fig, "e1-latencia-por-nivel.png")

    fig, ax1 = plt.subplots(figsize=(7, 4))
    idx = range(len(filas))
    w = 0.2
    ax1.bar([i - 1.5 * w for i in idx], [f["web"] for f in filas], w, label="CPU Web Server")
    ax1.bar([i - 0.5 * w for i in idx], [f["sql"] for f in filas], w, label="CPU Cloud SQL")
    ax1.bar([i + 0.5 * w for i in idx], [f["wrk"] for f in filas], w, label="CPU Worker Server")
    ax1.bar([i + 1.5 * w for i in idx], [f["gen"] for f in filas], w, label="CPU máx. generador")
    ax1.set_xticks(list(idx), [f["et"] for f in filas])
    ax1.set_ylabel("CPU media en la ventana (%)")
    ax1.set_ylim(0, 100)
    ax2 = ax1.twinx()
    ax2.plot(list(idx), [f["conex"] for f in filas], "k--o", label="conexiones BD (máx.)")
    ax2.set_ylabel("conexiones a Cloud SQL")
    ax2.grid(False)
    h1, l1 = ax1.get_legend_handles_labels()
    h2, l2 = ax2.get_legend_handles_labels()
    ax1.legend(h1 + h2, l1 + l2, loc="upper left", fontsize=8)
    ax1.set_title("Escenario 1: recursos por nivel")
    guardar(fig, "e1-recursos-por-nivel.png")


def comparar(etiquetas):
    """Antes y después en el mismo nivel: una barra por configuración."""
    nombres = {"e1-L3": "g1-small, pool 10", "e1-L3-pool25": "g1-small, pool 25",
               "e1-L3-sql-dedicado": "1 vCPU dedicado, pool 25"}
    filas = []
    for et in etiquetas:
        d, m = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
        dur = "http_req_duration{scenario:medicion}"
        uso = met(m, "sql_cpu", None, "valores") or []
        filas.append((nombres.get(et, et), k6(d, dur, "med"), k6(d, dur, "p(95)"),
                      k6(d, "dropped_iterations", "count") or 0))
    fig, (a1, a2) = plt.subplots(1, 2, figsize=(8, 3.6))
    x = range(len(filas))
    a1.bar([i - 0.2 for i in x], [f[1] for f in filas], 0.4, label="p50")
    a1.bar([i + 0.2 for i in x], [f[2] for f in filas], 0.4, label="p95")
    a1.set_yscale("log")
    a1.set_ylabel("latencia (ms, escala log)")
    a1.set_xticks(list(x), [f[0] for f in filas], rotation=15, fontsize=7)
    a1.legend()
    a2.bar(list(x), [f[3] for f in filas], color="tab:red")
    a2.set_ylabel("iteraciones perdidas")
    a2.set_xticks(list(x), [f[0] for f in filas], rotation=15, fontsize=7)
    fig.suptitle("Escenario 1, L3 (16 iter/s): qué cambia cada ajuste")
    guardar(fig, "e1-L3-antes-y-despues.png")


def minutos(valores, t0):
    return [(float(t) - t0) / 60 for t, _ in valores], [float(v) for _, v in valores]


def serie(et):
    m = cargar(f"{et}-metricas.json")
    t0 = datetime.fromisoformat(m["desde"].replace("Z", "+00:00")).timestamp()
    fig, axs = plt.subplots(3, 1, figsize=(7, 7), sharex=True)
    for nombre, filtro, etq, esc in (("cpu_uso", {"instance_name": "mooc-web"}, "CPU Web Server", 100),
                                     ("cpu_uso", {"instance_name": "mooc-worker"}, "CPU Worker Server", 100),
                                     ("sql_cpu", None, "CPU Cloud SQL", 100)):
        v = met(m, nombre, filtro, "valores")
        if v:
            x, y = minutos(v, t0)
            axs[0].plot(x, [a * esc for a in y], label=etq)
    axs[0].set_ylabel("CPU (%)")
    axs[0].legend(fontsize=8)
    v = met(m, "sql_conexiones", None, "valores")
    if v:
        x, y = minutos(v, t0)
        axs[1].plot(x, y, "k-")
    axs[1].set_ylabel("conexiones BD")
    for f in m["metricas"].get("api_p95_s") or []:
        r = f["serie"].get("route", "")
        if r.startswith(("POST /api/v1/enrollments/{id}/progress", "PATCH", "GET /api/v1/enrollments/{id}/content")):
            x, y = minutos(f["valores"], t0)
            axs[2].plot(x, [a * 1000 for a in y], label=r.replace("/api/v1", ""))
    axs[2].set_ylabel("p95 en la API (ms)")
    axs[2].set_xlabel("minutos desde el inicio (2 de calentamiento)")
    axs[2].legend(fontsize=7)
    axs[0].set_title(f"{et}: minuto a minuto")
    guardar(fig, f"{et}-serie.png")


def cola(et):
    m = cargar(f"{et}-metricas.json")
    t0 = datetime.fromisoformat(m["desde"].replace("Z", "+00:00")).timestamp()
    fig, axs = plt.subplots(3, 1, figsize=(7, 7), sharex=True)
    for f in m["metricas"].get("cola_profundidad") or []:
        s = f["serie"]
        if s.get("queue") == "bulk" and s.get("state") in ("pending", "active", "retry", "archived") and f["max"] > 0:
            x, y = minutos(f["valores"], t0)
            axs[0].plot(x, y, label=f"bulk {s['state']}")
    axs[0].set_ylabel("trabajos en cola")
    axs[0].legend(fontsize=8)
    v = met(m, "cola_antiguedad_s", {"queue": "bulk"}, "valores")
    if v:
        x, y = minutos(v, t0)
        axs[1].plot(x, [a / 60 for a in y], "r-")
    axs[1].set_ylabel("antigüedad del más viejo (min)")
    for nombre, filtro, etq in (("cpu_uso", {"instance_name": "mooc-worker"}, "CPU Worker Server"),
                                ("memoria_uso_pct", {"metadata_system_name": "mooc-worker"}, "Memoria Worker Server")):
        v = met(m, nombre, filtro, "valores")
        if v:
            x, y = minutos(v, t0)
            axs[2].plot(x, [a * (100 if nombre == "cpu_uso" else 1) for a in y], label=etq)
    axs[2].set_ylabel("%")
    axs[2].set_ylim(0, 100)
    axs[2].legend(fontsize=8)
    axs[2].set_xlabel("minutos desde el inicio")
    axs[0].set_title(f"{et}: cola de procesamiento")
    guardar(fig, f"{et}-cola.png")


if __name__ == "__main__":
    modo, etqs = sys.argv[1], sys.argv[2:]
    if modo == "e1":
        e1(etqs)
    elif modo == "comparar":
        comparar(etqs)
    else:
        for e in etqs:
            {"serie": serie, "cola": cola}[modo](e)
