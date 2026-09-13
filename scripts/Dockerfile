# Imagen de herramientas: aquí corren los scripts de scripts/ y la colección de
# Postman.
#
# Antes corrían en la máquina de cada quien, y eso arrastraba las diferencias
# entre sistemas: `declare -A` y `${var,,}` exigen bash 4 y macOS trae 3.2;
# `stat -c`, `sha256sum` y `split -d` son de GNU; `python3` —del que dependen
# las 53 llamadas a `jq()`— no viene de fábrica en macOS 12.3+. Cada diferencia
# se arreglaba una a una hasta que aparecía la siguiente. Con la imagen fija,
# todo el mundo ejecuta el mismo bash, el mismo coreutils y el mismo python.
#
# La base es Node porque newman —el corredor de la colección de Postman— es un
# paquete de npm: así la colección corre en la misma imagen que los scripts, y
# no hace falta una segunda.
#
# Versión fija a propósito: un clon nuevo debe traer exactamente esto.
FROM node:22.22.3-alpine3.22

# bash, coreutils, grep, sed y gawk: las versiones GNU, no las de busybox, que
#   es lo que suponen los scripts.
# py3-yaml: contrato.sh compara el contrato servido con el del repositorio.
# socat: reproduce dentro del contenedor los puertos que la máquina publica
#   (ver entrada.sh).
# docker-cli y su plugin compose: los scripts consultan PostgreSQL y Redis con
#   `docker compose exec`, a través del socket montado.
RUN apk add --no-cache \
      bash \
      coreutils \
      grep \
      sed \
      gawk \
      findutils \
      curl \
      python3 \
      py3-yaml \
      socat \
      docker-cli \
      docker-cli-compose \
      ca-certificates \
 && npm install --global --no-fund --no-audit newman@6.2.1 \
 && npm cache clean --force

COPY entrada.sh /usr/local/bin/entrada.sh

WORKDIR /repo
ENTRYPOINT ["/usr/local/bin/entrada.sh"]
CMD ["bash"]
