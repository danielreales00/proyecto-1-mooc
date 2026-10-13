FROM caddy:2.10-alpine
COPY Caddyfile /etc/caddy/Caddyfile
COPY index.html styles.css app.js api.js /srv/
COPY vendor /srv/vendor
EXPOSE 80
