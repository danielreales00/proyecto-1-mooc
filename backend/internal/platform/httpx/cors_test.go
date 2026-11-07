package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	llamadas := 0
	h := CORS([]string{"https://frontend.example"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llamadas++
		w.WriteHeader(http.StatusOK)
	}))

	pre := httptest.NewRequest(http.MethodOptions, "/api/v1/courses", nil)
	pre.Header.Set("Origin", "https://frontend.example")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, pre)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "https://frontend.example" {
		t.Fatalf("preflight: código %d, origen %q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
	if llamadas != 0 {
		t.Fatal("el preflight no debe llegar al manejador")
	}

	ajeno := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	ajeno.Header.Set("Origin", "https://otro.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, ajeno)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("un origen no configurado no recibe cabeceras CORS")
	}

	propio := httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil)
	propio.Header.Set("Origin", "https://frontend.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, propio)
	if w.Header().Get("Access-Control-Expose-Headers") == "" || llamadas != 2 {
		t.Fatal("la petición permitida debe exponer ETag y llegar al manejador")
	}
}
