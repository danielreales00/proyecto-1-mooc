// Swagger UI servido por la interfaz: en Cloud Run la API vive en otro origen
// y no tiene el contenedor de Swagger del entorno local. El contrato lo sirve
// la API en /openapi.yaml, con CORS para este origen.
const api = window.MOOC_API || "";
const LOCAL = "http://localhost:8090";
// El contrato declara el servidor local. En la nube se sustituye por el
// origen de la API, para que «Try it out» llegue al backend desplegado.
window.SwaggerUIBundle({
  url: api + "/openapi.yaml",
  dom_id: "#swagger-ui",
  persistAuthorization: true,
  displayRequestDuration: true,
  responseInterceptor: (res) => {
    if (api && res.url.endsWith("/openapi.yaml") && typeof res.text === "string") {
      res.text = res.text.split(LOCAL).join(api);
      res.data = res.text;
    }
    return res;
  },
  requestInterceptor: (req) => {
    if (api && req.url.startsWith(LOCAL)) req.url = api + req.url.slice(LOCAL.length);
    return req;
  },
});
