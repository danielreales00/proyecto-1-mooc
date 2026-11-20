#!/usr/bin/env python3
"""Entrega a la función local los mensajes de un topic, como lo haría Eventarc.

Saca los mensajes pendientes de una suscripción de pull del emulador de Pub/Sub
y se los envía a la función como CloudEvent `messagePublished`, el formato que
usa Eventarc en la nube (ADR-0021). Con --veces 2 entrega cada uno dos veces,
para comprobar que una reentrega no repite el trabajo.

Uso (dentro de la imagen mooc-herramientas, con make prueba-funcion):
  entregar-evento.py EMULADOR SUSCRIPCION FUNCION [--veces N]
"""
import argparse
import json
import sys
import time
import urllib.request


def post(url, cuerpo, cabeceras=None):
    req = urllib.request.Request(url, data=json.dumps(cuerpo).encode(), method="POST",
                                 headers={"Content-Type": "application/json", **(cabeceras or {})})
    with urllib.request.urlopen(req, timeout=60) as r:
        return r.status, r.read()


def main():
    a = argparse.ArgumentParser()
    a.add_argument("emulador")
    a.add_argument("suscripcion", help="projects/<p>/subscriptions/<s>")
    a.add_argument("funcion")
    a.add_argument("--veces", type=int, default=1)
    a.add_argument("--espera", type=int, default=30, help="segundos que espera el mensaje")
    args = a.parse_args()

    # El trabajo se publica después del commit y, en la prueba, después de que
    # el estudiante apruebe: se espera hasta --espera segundos.
    recibidos = []
    for _ in range(max(1, args.espera // 2)):
        _, cuerpo = post(f"http://{args.emulador}/v1/{args.suscripcion}:pull",
                         {"maxMessages": 10, "returnImmediately": True})
        recibidos = json.loads(cuerpo or b"{}").get("receivedMessages", [])
        if recibidos:
            break
        time.sleep(2)
    if not recibidos:
        sys.exit(f"no llegó ningún mensaje a {args.suscripcion} en {args.espera} s")
    for r in recibidos:
        m = r["message"]
        evento = {
            "specversion": "1.0",
            "type": "google.cloud.pubsub.topic.v1.messagePublished",
            "source": "//pubsub.googleapis.com/" + args.suscripcion,
            "id": m["messageId"],
            "datacontenttype": "application/json",
            "data": {"message": m, "subscription": args.suscripcion},
        }
        for n in range(args.veces):
            estado, _ = post(args.funcion, evento,
                             {"Content-Type": "application/cloudevents+json"})
            print(f"  entrega {n + 1} de {m['attributes'].get('job_key', '?')}: HTTP {estado}")
        post(f"http://{args.emulador}/v1/{args.suscripcion}:acknowledge", {"ackIds": [r["ackId"]]})


if __name__ == "__main__":
    main()
