FROM caddy:2.10-alpine
# Sin root: un usuario propio, el puerto 8080 y los directorios de Caddy
# a su nombre. Solo sirve archivos estáticos detrás del proxy.
RUN adduser -D -H -u 10001 web && chown -R web /data /config
COPY Caddyfile /etc/caddy/Caddyfile
COPY index.html styles.css app.js api.js /srv/
COPY vendor /srv/vendor
USER web
EXPOSE 8080
