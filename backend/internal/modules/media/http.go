package media

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/problem"
)

type API struct{ svc *Service }

func NewAPI(svc *Service) *API { return &API{svc: svc} }

func (a *API) Routes(mux *http.ServeMux, auth httpx.Middleware,
	soloDocentes httpx.Middleware, idem httpx.Middleware) {

	p := func(h http.HandlerFunc) http.Handler { return auth(soloDocentes(h)) }
	pi := func(h http.HandlerFunc) http.Handler {
		if idem == nil {
			return p(h)
		}
		return auth(soloDocentes(idem(h)))
	}

	mux.Handle("POST /api/v1/assets/init", pi(a.iniciar))
	mux.Handle("GET /api/v1/assets/{id}", p(a.ver))
	mux.Handle("GET /api/v1/assets/{id}/upload", p(a.estadoDeCarga))
	mux.Handle("POST /api/v1/assets/{id}/upload/parts", p(a.renovarPartes))
	mux.Handle("POST /api/v1/assets/{id}/complete", pi(a.completar))
	mux.Handle("POST /api/v1/assets/{id}/abort", p(a.abortar))
	mux.Handle("GET /api/v1/assets/{id}/content", p(a.contenido))

	// El manifiesto maestro va autenticado como todo lo demás.
	mux.Handle("GET /api/v1/assets/{id}/hls/master.m3u8", p(a.hlsMaster))
	// Las playlists de variante NO: un reproductor pide cada documento del
	// manifiesto por su cuenta y no lleva cabecera de autorización a ninguno.
	// Lo que autoriza es la credencial de reproducción que el maestro incrusta
	// en la URL, que caduca en media hora y no identifica a nadie. El patrón
	// literal de arriba gana al comodín de abajo, así que master.m3u8 nunca
	// cae aquí.
	mux.Handle("GET /api/v1/assets/{id}/hls/{playlist}", http.HandlerFunc(a.hlsPlaylist))
}

func actor(r *http.Request) Actor {
	pr, _ := httpx.PrincipalFrom(r.Context())
	return Actor{UserID: pr.UserID, Role: pr.Role, IP: httpx.ClientIP(r),
		Agent: r.UserAgent(), Trace: httpx.RequestIDFrom(r.Context())}
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, problem.Validation("El identificador no es un UUID válido.")
	}
	return id, nil
}

func vistaAsset(a Asset) map[string]any {
	return map[string]any{
		"id": a.ID.String(), "kind": a.Kind,
		"original_filename": a.OriginalFilename,
		"declared_mime":     a.DeclaredMIME,
		"detected_mime":     a.DetectedMIME,
		"size_bytes":        a.SizeBytes,
		"sha256":            a.SHA256,
		"duration_seconds":  a.DurationSeconds,
		"status":            a.Status,
		"last_error":        a.LastError,
		"created_at":        a.CreatedAt,
	}
}

func (a *API) iniciar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
		SHA256      string `json:"sha256"`
		Kind        string `json:"kind"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	t, err := a.svc.Iniciar(r.Context(), NuevaCarga{
		Filename: body.Filename, ContentType: body.ContentType,
		SizeBytes: body.SizeBytes, SHA256: body.SHA256, Kind: body.Kind,
	}, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}

	partes := make([]map[string]any, 0, len(t.Partes))
	for _, p := range t.Partes {
		partes = append(partes, map[string]any{
			"part_number": p.Numero, "url": p.URL, "expires_at": p.ExpiresAt,
		})
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"asset_id": t.Asset.ID.String(), "upload_id": t.UploadID,
		"part_size": t.PartSize, "total_parts": t.TotalParts,
		"expires_at": t.ExpiresAt, "parts": partes,
	})
}

func (a *API) ver(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	as, err := a.svc.Asset(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	vista := vistaAsset(as)

	derivados, err := a.svc.Derivados(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	salidas := make([]map[string]any, 0, len(derivados))
	for _, d := range derivados {
		salidas = append(salidas, map[string]any{
			"kind": d.Kind, "variant": d.Variante, "key": d.Key, "bytes": d.Bytes,
		})
	}
	vista["derivatives"] = salidas

	httpx.JSON(w, http.StatusOK, vista)
}

// estadoDeCarga es lo que permite reanudar: dice qué partes tiene ya el
// almacén para que el cliente suba solo las que faltan (SEG-3).
func (a *API) estadoDeCarga(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	as, c, partes, err := a.svc.EstadoDeCarga(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	recibidas := make([]map[string]any, 0, len(partes))
	vistas := map[int]bool{}
	for _, p := range partes {
		vistas[p.Numero] = true
		recibidas = append(recibidas, map[string]any{
			"part_number": p.Numero, "etag": p.ETag, "size_bytes": p.Bytes,
		})
	}
	faltan := make([]int, 0)
	for i := 1; i <= c.TotalParts; i++ {
		if !vistas[i] {
			faltan = append(faltan, i)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"asset_id": as.ID.String(), "upload_id": c.S3UploadID,
		"part_size": c.PartSize, "total_parts": c.TotalParts,
		"received_parts": recibidas, "missing_parts": faltan,
		"expires_at": c.ExpiresAt,
	})
}

func (a *API) renovarPartes(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body struct {
		Parts []int `json:"parts"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if len(body.Parts) == 0 {
		httpx.Fail(w, r, problem.Validation("Indica qué partes hay que renovar."))
		return
	}
	ps, err := a.svc.RenovarPartes(r.Context(), id, body.Parts, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	out := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		out = append(out, map[string]any{"part_number": p.Numero, "url": p.URL, "expires_at": p.ExpiresAt})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"parts": out})
}

func (a *API) completar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	var body struct {
		Parts []struct {
			PartNumber int    `json:"part_number"`
			ETag       string `json:"etag"`
		} `json:"parts"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	partes := make([]Parte, 0, len(body.Parts))
	for _, p := range body.Parts {
		partes = append(partes, Parte{Numero: p.PartNumber, ETag: p.ETag})
	}
	as, err := a.svc.Completar(r.Context(), id, partes, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	// 202: la verificación la hace un worker; la API no espera (CA-02).
	httpx.JSON(w, http.StatusAccepted, vistaAsset(as))
}

func (a *API) abortar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	if err := a.svc.Abortar(r.Context(), id, actor(r)); err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	httpx.NoContent(w)
}

// contenido responde 302 a una URL firmada: los bytes nunca pasan por la API.
func (a *API) contenido(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	crudo, err := a.svc.URLDeDescarga(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}

	// El destino se comprueba dos veces: en el servicio y aquí, a la vista de
	// quien lee el redirect. Redirigir a una URL calculada sin mirar a dónde
	// va es como se abren los *open redirect*, y esta llega desde un puerto que
	// mañana puede tener otra implementación.
	destino, err := url.Parse(crudo)
	if err != nil || destino.Host != a.svc.HostDelAlmacen() ||
		(destino.Scheme != "http" && destino.Scheme != "https") {
		httpx.Fail(w, r, problem.Internal())
		return
	}

	// #nosec G710 -- el destino se acaba de validar contra el host del almacén
	// configurado; no proviene de la petición.
	http.Redirect(w, r, destino.String(), http.StatusFound)
}

func (a *API) hlsMaster(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	manifiesto, err := a.svc.MasterFirmado(r.Context(), id, actor(r))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	escribirManifiesto(w, manifiesto)
}

func (a *API) hlsPlaylist(w http.ResponseWriter, r *http.Request) {
	archivo := r.PathValue("playlist")
	if !strings.HasSuffix(archivo, ".m3u8") {
		httpx.Fail(w, r, problem.NotFound("El archivo solicitado no existe."))
		return
	}
	token := r.URL.Query().Get("t")
	if token == "" {
		httpx.Fail(w, r, problem.Unauthorized("Falta la credencial de reproducción."))
		return
	}

	playlist, err := a.svc.PlaylistFirmada(r.Context(), token,
		strings.TrimSuffix(archivo, ".m3u8"))
	if err != nil {
		httpx.Fail(w, r, traducir(err))
		return
	}
	escribirManifiesto(w, playlist)
}

// Los manifiestos llevan URLs firmadas que caducan, así que no se cachean: un
// intermediario que guarde este texto entrega enlaces muertos.
func escribirManifiesto(w http.ResponseWriter, cuerpo string) {
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	// Un manifiesto es texto plano con un tipo propio. nosniff impide que un
	// navegador decida por su cuenta tratarlo como HTML, que es la única forma
	// en que este cuerpo podría ejecutar algo.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	// #nosec G705 -- no es HTML ni se sirve como tal: el cuerpo lo arma el
	// servidor (el maestro) o sale del bucket de derivados con cada URI
	// sustituida por una URL firmada que genera el propio almacén. Nada de lo
	// que llega en la petición acaba dentro.
	_, _ = w.Write([]byte(cuerpo))
}

func traducir(err error) error {
	var verrs ValidationErrors
	if errors.As(err, &verrs) {
		p := problem.Validation("La solicitud no es válida.")
		for _, e := range verrs {
			p.Errors = append(p.Errors, problem.FieldError{Code: e.Code, Field: e.Field, Detail: e.Detail})
		}
		return p
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return problem.NotFound("El archivo solicitado no existe.")
	case errors.Is(err, ErrProhibido):
		return problem.Forbidden("Este archivo no es tuyo.")
	case errors.Is(err, ErrCargaVencida):
		return problem.Conflict("upload.expired", "La carga expiró. Hay 24 horas para completarla.")
	case errors.Is(err, ErrNoListo):
		return problem.Conflict("asset.not_ready",
			"El archivo aún no se ha transcodificado. Vuelve a intentarlo en unos segundos.")
	case errors.Is(err, ErrEstado):
		return problem.Conflict("asset.invalid_state", "La operación no es válida en el estado actual del archivo.")
	default:
		return err
	}
}
