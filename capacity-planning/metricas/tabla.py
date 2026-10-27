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


def del_papel(nombre, papel):
    """Si una serie pertenece al papel, en cualquiera de las dos entregas.

    En la Entrega 2 había una máquina fija por papel: `mooc-web` y
    `mooc-worker`. En la 3 son grupos, y cada instancia lleva su propio sufijo
    (`mooc-api-znwh`, `mooc-worker-0w44`). Comparar por igualdad dejaba las
    tablas de la Entrega 3 vacías, y es la forma de descubrirlo con las
    corridas ya hechas.
    """
    if papel == "api":
        return nombre == "mooc-web" or nombre.startswith("mooc-api-")
    if papel == "worker":
        return nombre == "mooc-worker" or nombre.startswith("mooc-worker-")
    return nombre == f"mooc-{papel}"


def filas_del_papel(met, nombre, etiqueta, papel):
    filas = met["metricas"].get(nombre) if met else None
    if not filas or isinstance(filas, dict):
        return []
    return [f for f in filas if del_papel(f["serie"].get(etiqueta, ""), papel)]


def grupo(met, nombre, etiqueta, papel, campo="max", entre="max"):
    """Un valor por papel, agregando las instancias que lo atendieron.

    El máximo se toma entre todas las instancias. La media se promedia entre
    ellas: el máximo de las medias describiría a la instancia más cargada del
    grupo, no al grupo, y con autoscaling eso exagera el uso justo en los
    niveles donde el grupo creció.
    """
    vals = [f[campo] for f in filas_del_papel(met, nombre, etiqueta, papel) if f[campo] is not None]
    if not vals:
        return None
    return max(vals) if entre == "max" else sum(vals) / len(vals)


def instancias(met, papel):
    """Cuántas instancias del papel aparecieron en la ventana de la corrida.

    Es la diferencia arquitectónica que la Entrega 3 evalúa, así que va en la
    tabla y no en una nota al pie.
    """
    nombres = {f["serie"].get("instance_name") for f in filas_del_papel(met, "cpu_uso", "instance_name", papel)}
    return len(nombres) or None


def nombre_api(met):
    """«API» con grupos, «web» con la máquina fija de la línea base."""
    for f in filas_del_papel(met, "cpu_uso", "instance_name", "api"):
        return "web" if f["serie"].get("instance_name") == "mooc-web" else "API"
    return "API"


def ms(x):
    return "-" if x is None else f"{x:.0f}"


def pct(x):
    return "-" if x is None else f"{x * 100:.0f} %"


SEPARADOR = "| --- | --- | --- | --- | --- | --- |"
DURACION = "http_req_duration{scenario:medicion}"


def de_serie(met, nombre, filtro=None, campo="max"):
    """serie() que tolera que la corrida no tenga métricas exportadas."""
    return serie(met, nombre, filtro, campo) if met else None


def por_ciento(x, decimales=0):
    return "-" if x is None else f"{x * 100:.{decimales}f} %"


def errores(d, reqs):
    if not reqs:
        return "-"
    e5 = k6(d, "errores_5xx{scenario:medicion}", "count") or 0
    to = k6(d, "timeouts{scenario:medicion}", "count") or 0
    return f"{(e5 + to) / reqs * 100:.2f} %"


def memoria(met):
    api = grupo(met, "memoria_uso_pct", "metadata_system_name", "api")
    wrk = grupo(met, "memoria_uso_pct", "metadata_system_name", "worker")
    return f"{api:.0f} / {wrk:.0f} %" if api and wrk else "-"


def fila_e1(et):
    d, met = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
    if not d:
        return None
    reqs = k6(d, "http_reqs{scenario:medicion}", "count") or 0
    rate = k6(d, "http_reqs{scenario:medicion}", "rate") or 0
    l429 = k6(d, "limitados_429{scenario:medicion}", "count") or 0
    caidas = k6(d, "dropped_iterations", "count") or 0
    return {
        "et": et, "req_s": f"{rate:.1f}", "p50": ms(k6(d, DURACION, "med")), "p95": ms(k6(d, DURACION, "p(95)")),
        "p99": ms(k6(d, DURACION, "p(99)")), "err": errores(d, reqs),
        "429": str(int(l429)), "chk": por_ciento(k6(d, "checks{scenario:medicion}", "rate"), 2),
        "caidas": str(int(caidas)),
        "cpu_web": pct(grupo(met, "cpu_uso", "instance_name", "api", "media", "media")),
        "cpu_wrk": pct(grupo(met, "cpu_uso", "instance_name", "worker", "media", "media")),
        "cpu_gen": pct(grupo(met, "cpu_uso", "instance_name", "generador", "max")),
        "inst": f"{instancias(met, 'api') or '-'} / {instancias(met, 'worker') or '-'}",
        "mem": memoria(met),
        "sql": pct(de_serie(met, "sql_cpu", None, "media")), "conex": ms(de_serie(met, "sql_conexiones")),
    }


def primeras_metricas(etiquetas):
    """Las métricas de la primera corrida que las tenga, para rotular."""
    for et in etiquetas:
        met = cargar(f"{et}-metricas.json")
        if met:
            return met
    return None


def e1(etiquetas):
    filas = [f for f in (fila_e1(et) for et in etiquetas) if f]
    papel = nombre_api(primeras_metricas(etiquetas))
    print(f"| Nivel | Req/s | p50 / p95 / p99 (ms) | Errores (5xx + timeouts) | "
          f"CPU {papel} / worker / SQL | Memoria {papel} / worker |")
    print(SEPARADOR)
    for f in filas:
        print(f"| {f['et']} | {f['req_s']} | {f['p50']} / {f['p95']} / {f['p99']} | {f['err']} | "
              f"{f['cpu_web']} / {f['cpu_wrk']} / {f['sql']} | {f['mem']} |")
    print()
    print(f"| Nivel | 429 | Comprobaciones | Iteraciones perdidas | Instancias {papel} / worker | "
          f"CPU máx. generador | Conexiones BD máx. |")
    print("| --- | --- | --- | --- | --- | --- | --- |")
    for f in filas:
        print(f"| {f['et']} | {f['429']} | {f['chk']} | {f['caidas']} | {f['inst']} | "
              f"{f['cpu_gen']} | {f['conex']} |")


def valor(d, metrica, clave):
    x = d["metrics"].get(metrica)
    return x["values"].get(clave) if x else None


def listo_p50(d, perfil):
    """Mediana de la confirmación a ready de un perfil, si hubo cargas de él."""
    nombre = f"completa_a_ready_s{{perfil:{perfil}}}"
    return valor(d, nombre, "med") if valor(d, nombre, "count") else None


def fila_e2_control(et, d):
    a, b, c = (ms(listo_p50(d, p)) for p in "abc")
    transferencia = valor(d, "transferencia_mib_s", "med") or 0
    fallidos = (valor(d, "http_req_failed{name:GET segmento (almacén)}", "rate") or 0) * 100
    return (f"| {et} | {ms(valor(d, 'http_req_duration{name:POST /assets/init}', 'p(95)'))} / "
            f"{ms(valor(d, 'http_req_duration{name:POST /assets/complete}', 'p(95)'))} | "
            f"{transferencia:.1f} | {a} / {b} / {c} | "
            f"{ms(valor(d, 'http_req_duration{name:GET playlist}', 'p(95)'))} / "
            f"{ms(valor(d, 'http_req_duration{name:GET segmento (almacén)}', 'p(95)'))} | "
            f"{fallidos:.2f} % |")


def fila_e2_cola(et, met):
    pend = serie(met, "cola_profundidad", {"queue": "bulk", "state": "pending"})
    ant = serie(met, "cola_antiguedad_s", {"queue": "bulk"})
    cm = grupo(met, "cpu_uso", "instance_name", "worker", "media", "media")
    cx = grupo(met, "cpu_uso", "instance_name", "worker")
    mx = grupo(met, "memoria_uso_pct", "metadata_system_name", "worker")
    dx = grupo(met, "disco_uso_pct", "metadata_system_name", "worker")
    return (f"| {et} | {ms(pend)} | {ms(ant)} | {instancias(met, 'worker') or '-'} | "
            f"{pct(cm)} / {pct(cx)} | {entero_pct(mx)} | {entero_pct(dx)} |")


def entero_pct(x):
    return "-" if x is None else f"{x:.0f} %"


def e2(etiquetas):
    """Escenario 2: control (API), transferencia directa, cola y reproducción."""
    print("| Nivel | p95 autorizar / confirmar (ms) | Transferencia (MiB/s) | Completa→ready p50 A / B / C (s) | p95 playlist / segmento (ms) | Segmentos fallidos |")
    print(SEPARADOR)
    con_metricas = []
    for et in etiquetas:
        d, met = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
        if not d:
            continue
        print(fila_e2_control(et, d))
        if met:
            con_metricas.append((et, met))
    print()
    print("| Nivel | Cola bulk máx. (pendientes) | Antigüedad máx. (s) | Instancias worker | "
          "CPU worker media / máx. | Memoria worker máx. | Disco worker máx. |")
    print("| --- | --- | --- | --- | --- | --- | --- |")
    for et, met in con_metricas:
        print(fila_e2_cola(et, met))


DETALLE = ("cpu_uso", "memoria_uso_pct", "sql_cpu", "sql_conexiones", "redis_comandos_s",
           "cola_profundidad", "cola_antiguedad_s", "api_p95_s")


def detalle_operaciones(d):
    prefijo = "http_req_duration{scenario:medicion,name:"
    for k, v in sorted(d["metrics"].items()):
        if k.startswith(prefijo):
            vals = v["values"]
            print(f"  {k[41:-1]:34} p50 {ms(vals.get('med')):>6}  p95 {ms(vals.get('p(95)')):>6}  p99 {ms(vals.get('p(99)')):>6}")


def relevante(nombre, f):
    """Una serie con datos, o una de CPU aunque haya quedado en cero."""
    return isinstance(f, dict) and "serie" in f and (f["max"] or nombre in ("cpu_uso", "sql_cpu"))


def detalle_metricas(met):
    for nombre in DETALLE:
        for f in (met["metricas"].get(nombre) or [])[:12]:
            if relevante(nombre, f):
                print(f"  {nombre:20} {json.dumps(f['serie'], ensure_ascii=False):60.60} media {f['media']:.4g}  máx {f['max']:.4g}")


def detalle(et):
    d, met = cargar(f"{et}.json"), cargar(f"{et}-metricas.json")
    print(f"== {et}")
    detalle_operaciones(d)
    if met:
        detalle_metricas(met)


if __name__ == "__main__":
    modo, etiquetas = sys.argv[1], sys.argv[2:]
    {"e1": lambda: e1(etiquetas), "e2": lambda: e2(etiquetas),
     "detalle": lambda: [detalle(e) for e in etiquetas]}[modo]()
