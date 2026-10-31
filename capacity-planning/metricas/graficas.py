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


CPU_WORKER = "CPU Worker Server"


def etiquetas_cpu(etiquetas):
    """Como llamar a cada papel: en la Entrega 2 son maquinas, en la 3 grupos."""
    if any(e.startswith("e3-") for e in etiquetas):
        return "CPU grupo API", "CPU grupo workers"
    return "CPU Web Server", CPU_WORKER

def cargar(n):
    p = os.path.join(RES, n)
    return json.load(open(p)) if os.path.exists(p) else None


def k6(d, m, k):
    x = d["metrics"].get(m)
    return x["values"].get(k) if x else None


def del_papel(nombre, papel):
    """Si una serie pertenece al papel, en cualquiera de las dos entregas.

    En la Entrega 2 habia una maquina fija por papel: `mooc-web` y
    `mooc-worker`. En la 3 son grupos y cada instancia lleva sufijo propio
    (`mooc-api-znwh`). Comparar por igualdad dejaba estas graficas con la
    leyenda puesta y sin una sola barra, que es como se descubrio.
    """
    if papel == "mooc-web":
        return nombre == "mooc-web" or nombre.startswith("mooc-api-")
    if papel == "mooc-worker":
        return nombre == "mooc-worker" or nombre.startswith("mooc-worker-")
    return nombre == papel


ETIQUETAS_DE_MAQUINA = ("instance_name", "metadata_system_name")


def casa(serie, clave, valor):
    """Si una etiqueta de la serie casa con lo pedido.

    Las que nombran maquinas se comparan por papel, porque en la Entrega 3
    cada instancia del grupo lleva su propio sufijo.
    """
    actual = serie.get(clave, "")
    if clave in ETIQUETAS_DE_MAQUINA:
        return del_papel(actual, valor)
    return actual == valor


def series_de(m, nombre, filtro):
    """Las series que casan con el filtro, tratando el papel por prefijo."""
    candidatas = [
        f for f in m["metricas"].get(nombre) or []
        if isinstance(f, dict) and "serie" in f
    ]
    if not filtro:
        return candidatas
    return [
        f for f in candidatas
        if all(casa(f["serie"], c, v) for c, v in filtro.items())
    ]


def met(m, nombre, filtro=None, campo="media"):
    """Un valor por papel, agregando las instancias del grupo.

    El maximo se toma entre todas; la media se promedia entre ellas, porque el
    maximo de las medias describiria a la instancia mas cargada y no al grupo.
    """
    fs = series_de(m, nombre, filtro)
    if not fs:
        return None
    if campo == "valores":
        # Un grupo son varias series, cada una viva en un tramo distinto: la
        # instancia que se retira pronto solo tiene valores del principio.
        # Quedarse con la primera cortaba la linea a mitad de la grafica. Se
        # promedia por instante entre las que esten vivas en ese momento.
        if len(fs) == 1:
            return fs[0]["valores"]
        por_instante = {}
        for f in fs:
            for marca, valor in f.get("valores") or []:
                por_instante.setdefault(int(marca), []).append(float(valor))
        return [[k, sum(v) / len(v)] for k, v in sorted(por_instante.items())]
    vals = [f[campo] for f in fs if f.get(campo) is not None]
    if not vals:
        return None
    return sum(vals) / len(vals) if campo == "media" else max(vals)


def entrega_de(etiquetas):
    """Prefijo de entrega a partir de las etiquetas: `e3-L2-1` → `e3`.

    Los modos que agregan varios niveles escribian un nombre fijo, asi que
    dibujar la Entrega 3 sobrepisaba las graficas de la Entrega 2, que son
    evidencia de una entrega ya presentada. El nombre tiene que depender de los
    datos y no solo del modo.
    """
    prefijos = {e.split("-", 1)[0] for e in etiquetas if "-" in e}
    return prefijos.pop() if len(prefijos) == 1 else "mixto"


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
    guardar(fig, f"{entrega_de(etiquetas)}-latencia-por-nivel.png")

    fig, ax1 = plt.subplots(figsize=(7, 4))
    idx = range(len(filas))
    w = 0.2
    etq_web, etq_wrk = etiquetas_cpu([f["et"] for f in filas])
    ax1.bar([i - 1.5 * w for i in idx], [f["web"] for f in filas], w, label=etq_web)
    ax1.bar([i - 0.5 * w for i in idx], [f["sql"] for f in filas], w, label="CPU Cloud SQL")
    ax1.bar([i + 0.5 * w for i in idx], [f["wrk"] for f in filas], w, label=etq_wrk)
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
    guardar(fig, f"{entrega_de(etiquetas)}-recursos-por-nivel.png")


def comparar(etiquetas):
    """Antes y después en el mismo nivel: una barra por configuración."""
    nombres = {"e1-L3": "g1-small, pool 10", "e1-L3-pool25": "g1-small, pool 25",
               "e1-L3-sql-dedicado": "1 vCPU dedicado, pool 25"}
    filas = []
    for et in etiquetas:
        d = cargar(f"{et}.json")
        dur = "http_req_duration{scenario:medicion}"
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


def e2(etiquetas):
    """Escenario 2 por nivel: cuánto tarda un video en estar listo y cuánto trabaja el Worker Server."""
    import math
    niveles, listos, cpu, fila = [], [], [], []
    for et in etiquetas:
        d, m = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
        vals = [k6(d, f"completa_a_ready_s{{perfil:{p}}}", "med") for p in "abc"
                if k6(d, f"completa_a_ready_s{{perfil:{p}}}", "count")]
        niveles.append(et.replace("e2-", ""))
        listos.append(max(vals) / 60 if vals else 0)
        cpu.append(100 * (met(m, "cpu_uso", {"instance_name": "mooc-worker"}) or 0))
        fila.append(max((f["max"] for f in m["metricas"].get("cola_profundidad") or []
                         if f["serie"].get("queue") == "bulk" and f["serie"].get("state") == "pending"), default=0))
    cargas = {"M0": "1 video/min", "M1": "3 videos/min", "M2": "6 videos/min", "M3": "12 videos/min"}
    etq = [f"{n}\n{cargas.get(n, '')}" for n in niveles]
    fig, (a1, a2) = plt.subplots(1, 2, figsize=(9, 3.8))
    barras = a1.bar(etq, listos, color=["tab:green" if v < 1 else "tab:red" for v in listos])
    for b, v in zip(barras, listos):
        a1.annotate(f"{v * 60:.0f} s" if v < 1 else f"{v:.0f} min", (b.get_x() + b.get_width() / 2, v),
                    ha="center", va="bottom", fontsize=8)
    a1.set_ylabel("minutos hasta que el video está listo\n(mediana, perfil más lento del nivel)")
    a1.set_title("Espera de un video", fontsize=9)
    barras = a2.bar(etq, cpu, color="tab:blue")
    for b, v in zip(barras, cpu):
        a2.annotate(f"{v:.0f} %", (b.get_x() + b.get_width() / 2, v), ha="center", va="bottom", fontsize=8)
    a2.set_ylim(0, 100)
    quien = etiquetas_cpu(etiquetas)[1].replace("CPU ", "")
    a2.set_ylabel(f"CPU media, {quien} (%)")
    a2.set_title(f"Trabajo: {quien}", fontsize=9)
    fig.suptitle("Escenario 2: cada nivel de carga")
    guardar(fig, f"{entrega_de(etiquetas)}-cola-por-nivel.png")


def minutos(valores, t0):
    return [(float(t) - t0) / 60 for t, _ in valores], [float(v) for _, v in valores]


def serie(et):
    m = cargar(f"{et}-metricas.json")
    t0 = datetime.fromisoformat(m["desde"].replace("Z", "+00:00")).timestamp()
    fig, axs = plt.subplots(3, 1, figsize=(7, 7), sharex=True)
    etq_web, etq_wrk = etiquetas_cpu([et])
    for nombre, filtro, etq, esc in (("cpu_uso", {"instance_name": "mooc-web"}, etq_web, 100),
                                     ("cpu_uso", {"instance_name": "mooc-worker"}, etq_wrk, 100),
                                     ("sql_cpu", None, "CPU Cloud SQL", 100)):
        v = met(m, nombre, filtro, "valores")
        if v:
            x, y = minutos(v, t0)
            axs[0].plot(x, [a * esc for a in y], label=etq)
    axs[0].set_ylabel("CPU (%)")
    # El bucket mas alto de http_request_duration_seconds son 10 s, asi que el
    # p95 del servidor se queda pegado ahi cuando lo rebasa: la linea plana en
    # 10.000 ms significa «10 s o mas», no «10 s». Se marca para que nadie la
    # lea como una medicion.
    axs[2].axhline(10000, color="gray", linestyle=":", linewidth=1)
    axs[2].text(0.01, 10000, " tope del histograma (10 s)", fontsize=7,
                color="gray", va="bottom", transform=axs[2].get_yaxis_transform())
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
    _, etq_wrk = etiquetas_cpu([et])
    mem_wrk = etq_wrk.replace("CPU", "Memoria")
    for nombre, filtro, etq in (("cpu_uso", {"instance_name": "mooc-worker"}, etq_wrk),
                                ("memoria_uso_pct", {"metadata_system_name": "mooc-worker"}, mem_wrk)):
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
    elif modo == "e2":
        e2(etqs)
    else:
        for e in etqs:
            {"serie": serie, "cola": cola}[modo](e)
