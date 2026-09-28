#!/usr/bin/env python3
"""Arma las filas de las tablas de resultados (§9) a partir de los archivos.

Lee, por etiqueta, el resumen de k6 (<etiqueta>.json) y las métricas de Cloud
Monitoring de la misma ventana (<etiqueta>-metricas.json), y escribe la fila en
Markdown. Las cifras del informe salen de aquí, no se copian a mano.

  tabla.py e1 e1-L0 e1-L1 ...        # escenario 1
  tabla.py e2 e2-M0 e2-M1 ...        # escenario 2
  tabla.py detalle e1-L2             # p95 por operación y por máquina
"""
import json
import os
import sys

DIR = os.path.join(os.path.dirname(__file__), "..", "resultados")


def cargar(nombre):
    ruta = os.path.join(DIR, nombre)
    return json.load(open(ruta)) if os.path.exists(ruta) else None


def k6(d, metrica, clave):
    m = d["metrics"].get(metrica)
    return m["values"].get(clave) if m else None


def serie(met, nombre, filtro=None, campo="max"):
    filas = met["metricas"].get(nombre) or []
    if isinstance(filas, dict):
        return None
    vals = [f[campo] for f in filas if not filtro or all(f["serie"].get(k) == v for k, v in filtro.items())]
    return max(vals) if vals else None


def ms(x):
    return "-" if x is None else f"{x:.0f}"


def pct(x):
    return "-" if x is None else f"{x * 100:.0f} %"


def fila_e1(et):
    d, met = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
    if not d:
        return None
    dur = "http_req_duration{scenario:medicion}"
    reqs = k6(d, "http_reqs{scenario:medicion}", "count") or 0
    rate = k6(d, "http_reqs{scenario:medicion}", "rate") or 0
    e5 = k6(d, "errores_5xx{scenario:medicion}", "count") or 0
    to = k6(d, "timeouts{scenario:medicion}", "count") or 0
    l429 = k6(d, "limitados_429{scenario:medicion}", "count") or 0
    chk = k6(d, "checks{scenario:medicion}", "rate")
    caidas = k6(d, "dropped_iterations", "count") or 0
    cpu_web = serie(met, "cpu_uso", {"instance_name": "mooc-web"}, "media") if met else None
    cpu_wrk = serie(met, "cpu_uso", {"instance_name": "mooc-worker"}, "media") if met else None
    cpu_gen = serie(met, "cpu_uso", {"instance_name": "mooc-generador"}, "max") if met else None
    mem_web = serie(met, "memoria_uso_pct", {"metadata_system_name": "mooc-web"}) if met else None
    mem_wrk = serie(met, "memoria_uso_pct", {"metadata_system_name": "mooc-worker"}) if met else None
    sql_cpu = serie(met, "sql_cpu", None, "media") if met else None
    conex = serie(met, "sql_conexiones") if met else None
    return {
        "et": et, "req_s": f"{rate:.1f}", "p50": ms(k6(d, dur, "med")), "p95": ms(k6(d, dur, "p(95)")),
        "p99": ms(k6(d, dur, "p(99)")), "err": f"{(e5 + to) / reqs * 100:.2f} %" if reqs else "-",
        "429": str(int(l429)), "chk": "-" if chk is None else f"{chk * 100:.2f} %", "caidas": str(int(caidas)),
        "cpu_web": pct(cpu_web), "cpu_wrk": pct(cpu_wrk), "cpu_gen": pct(cpu_gen),
        "mem": f"{mem_web:.0f} / {mem_wrk:.0f} %" if mem_web and mem_wrk else "-",
        "sql": pct(sql_cpu), "conex": ms(conex),
    }


def e1(etiquetas):
    print("| Nivel | Req/s | p50 / p95 / p99 (ms) | Errores (5xx + timeouts) | CPU web / worker / SQL | Memoria web / worker |")
    print("| --- | --- | --- | --- | --- | --- |")
    for et in etiquetas:
        f = fila_e1(et)
        if f:
            print(f"| {f['et']} | {f['req_s']} | {f['p50']} / {f['p95']} / {f['p99']} | {f['err']} | "
                  f"{f['cpu_web']} / {f['cpu_wrk']} / {f['sql']} | {f['mem']} |")
    print()
    print("| Nivel | 429 | Comprobaciones | Iteraciones perdidas | CPU máx. generador | Conexiones BD máx. |")
    print("| --- | --- | --- | --- | --- | --- |")
    for et in etiquetas:
        f = fila_e1(et)
        if f:
            print(f"| {f['et']} | {f['429']} | {f['chk']} | {f['caidas']} | {f['cpu_gen']} | {f['conex']} |")


def e2(etiquetas):
    """Escenario 2: control (API), transferencia directa, cola y reproducción."""
    def v(d, m, k):
        x = d["metrics"].get(m)
        return x["values"].get(k) if x else None
    print("| Nivel | p95 autorizar / confirmar (ms) | Transferencia (MiB/s) | Completa→ready p50 A / B / C (s) | p95 playlist / segmento (ms) | Segmentos fallidos |")
    print("| --- | --- | --- | --- | --- | --- |")
    filas2 = []
    for et in etiquetas:
        d, met = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
        if not d:
            continue
        rd = lambda p: (v(d, f"completa_a_ready_s{{perfil:{p}}}", "med")
                        if v(d, f"completa_a_ready_s{{perfil:{p}}}", "count") else None)
        f = lambda x: "-" if x is None else f"{x:.0f}"
        print(f"| {et} | {ms(v(d, 'http_req_duration{name:POST /assets/init}', 'p(95)'))} / "
              f"{ms(v(d, 'http_req_duration{name:POST /assets/complete}', 'p(95)'))} | "
              f"{(v(d, 'transferencia_mib_s', 'med') or 0):.1f} | {f(rd('a'))} / {f(rd('b'))} / {f(rd('c'))} | "
              f"{ms(v(d, 'http_req_duration{name:GET playlist}', 'p(95)'))} / "
              f"{ms(v(d, 'http_req_duration{name:GET segmento (almacén)}', 'p(95)'))} | "
              f"{(v(d, 'http_req_failed{name:GET segmento (almacén)}', 'rate') or 0) * 100:.2f} % |")
        if met:
            filas2.append((et, met))
    print()
    print("| Nivel | Cola bulk máx. (pendientes) | Antigüedad máx. (s) | CPU worker media / máx. | Memoria worker máx. | Disco worker máx. |")
    print("| --- | --- | --- | --- | --- | --- |")
    for et, met in filas2:
        pend = serie(met, "cola_profundidad", {"queue": "bulk", "state": "pending"})
        ant = serie(met, "cola_antiguedad_s", {"queue": "bulk"})
        cm = serie(met, "cpu_uso", {"instance_name": "mooc-worker"}, "media")
        cx = serie(met, "cpu_uso", {"instance_name": "mooc-worker"})
        mx = serie(met, "memoria_uso_pct", {"metadata_system_name": "mooc-worker"})
        dx = serie(met, "disco_uso_pct", {"metadata_system_name": "mooc-worker"})
        print(f"| {et} | {ms(pend)} | {ms(ant)} | {pct(cm)} / {pct(cx)} | "
              f"{'-' if mx is None else f'{mx:.0f} %'} | {'-' if dx is None else f'{dx:.0f} %'} |")


def detalle(et):
    d, met = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
    print(f"== {et}")
    for k, v in sorted(d["metrics"].items()):
        if k.startswith("http_req_duration{scenario:medicion,name:"):
            vals = v["values"]
            print(f"  {k[41:-1]:34} p50 {ms(vals.get('med')):>6}  p95 {ms(vals.get('p(95)')):>6}  p99 {ms(vals.get('p(99)')):>6}")
    if met:
        for nombre in ("cpu_uso", "memoria_uso_pct", "sql_cpu", "sql_conexiones", "redis_comandos_s",
                       "cola_profundidad", "cola_antiguedad_s", "api_p95_s"):
            for f in (met["metricas"].get(nombre) or [])[:12]:
                if isinstance(f, dict) and "serie" in f and (f["max"] or nombre in ("cpu_uso", "sql_cpu")):
                    print(f"  {nombre:20} {json.dumps(f['serie'], ensure_ascii=False):60.60} media {f['media']:.4g}  máx {f['max']:.4g}")


if __name__ == "__main__":
    modo, etiquetas = sys.argv[1], sys.argv[2:]
    {"e1": lambda: e1(etiquetas), "e2": lambda: e2(etiquetas),
     "detalle": lambda: [detalle(e) for e in etiquetas]}[modo]()
