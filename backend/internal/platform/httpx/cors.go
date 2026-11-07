package httpx

import (
	"net/http"
	"slices"
)

// CORS deja que el frontend, servido desde otro origen, llame a la API desde
// el navegador (Entrega 4: frontend y backend son dos servicios de Cloud Run).
// Solo responde a los orígenes configurados. La sesión viaja en Authorization
// y no en cookies, así que no hacen falta credenciales CORS. Sin orígenes
// configurados no hace nada: la interfaz comparte origen con la API.
func CORS(origenes []string) Middleware {
	return func(next http.Handler) http.Handler {
		if len(origenes) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origen := r.Header.Get("Origin")
			if origen == "" || !slices.Contains(origenes, origen) {
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origen)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Expose-Headers", "ETag, Location, Retry-After, X-Request-Id")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
				h.Set("Access-Control-Allow-Headers",
					"Authorization, Content-Type, Idempotency-Key, If-Match, If-None-Match, X-Request-Id")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
