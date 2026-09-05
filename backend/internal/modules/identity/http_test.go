package identity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/ids"
	"mooc/backend/internal/platform/passwords"
)

// ---------------------------------------------------------------- dobles ---

// fakeDB absorbe las escrituras de auditoría y del registro de trabajos. No
// simula PostgreSQL: solo permite que el servicio corra sin base de datos.
type fakeDB struct{ execs int }

func (f *fakeDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	f.execs++
	return pgconn.CommandTag{}, nil
}
func (f *fakeDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("Query no se usa en estas pruebas")
}
func (f *fakeDB) QueryRow(context.Context, string, ...any) pgx.Row { return unusedRow{} }

type unusedRow struct{}

func (unusedRow) Scan(...any) error { return errors.New("QueryRow no se usa en estas pruebas") }

type fakeStore struct {
	db     *fakeDB
	users  map[string]User // por correo normalizado
	tokens map[string]OneTimeToken
}

func newFakeStore() *fakeStore {
	return &fakeStore{db: &fakeDB{}, users: map[string]User{}, tokens: map[string]OneTimeToken{}}
}

func (s *fakeStore) DB() dbx.DB { return s.db }

func (s *fakeStore) WithinTx(ctx context.Context, fn func(context.Context, dbx.DB) error) error {
	return fn(ctx, s.db)
}

func (s *fakeStore) InsertUser(_ context.Context, _ dbx.DB, u User) error {
	if _, existe := s.users[u.Email]; existe {
		return ErrEmailTaken
	}
	s.users[u.Email] = u
	return nil
}

func (s *fakeStore) UserByEmail(_ context.Context, _ dbx.DB, email string) (User, error) {
	u, ok := s.users[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (s *fakeStore) UserByID(_ context.Context, _ dbx.DB, id uuid.UUID) (User, error) {
	for _, u := range s.users {
		if u.ID == id {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (s *fakeStore) MarkEmailVerified(_ context.Context, _ dbx.DB, id uuid.UUID, at time.Time) error {
	for correo, u := range s.users {
		if u.ID == id {
			u.EmailVerifiedAt = &at
			s.users[correo] = u
			return nil
		}
	}
	return ErrNotFound
}

func (s *fakeStore) InsertOneTimeToken(_ context.Context, _ dbx.DB, t OneTimeToken) error {
	s.tokens[string(t.TokenSHA256)] = t
	return nil
}

func (s *fakeStore) ConsumeOneTimeToken(_ context.Context, _ dbx.DB, purpose string, hash []byte, now time.Time) (OneTimeToken, error) {
	t, ok := s.tokens[string(hash)]
	if !ok || t.Purpose != purpose || !t.Usable(now) {
		return OneTimeToken{}, ErrTokenInvalid
	}
	t.ConsumedAt = &now
	s.tokens[string(hash)] = t
	return t, nil
}

func (s *fakeStore) InsertSession(context.Context, dbx.DB, Session) error         { return nil }
func (s *fakeStore) RevokeSession(context.Context, dbx.DB, uuid.UUID, time.Time) error { return nil }

type fakeSessions struct{ porToken map[string]Session }

func newFakeSessions() *fakeSessions { return &fakeSessions{porToken: map[string]Session{}} }

func (f *fakeSessions) Create(_ context.Context, token string, s Session, _ time.Duration) error {
	f.porToken[token] = s
	return nil
}

func (f *fakeSessions) Lookup(_ context.Context, token string) (Session, error) {
	s, ok := f.porToken[token]
	if !ok {
		return Session{}, ErrNotFound
	}
	return s, nil
}

func (f *fakeSessions) Delete(_ context.Context, token string) error {
	delete(f.porToken, token)
	return nil
}

func (f *fakeSessions) DeleteAllForUser(context.Context, uuid.UUID) error { return nil }

type fakePublisher struct {
	publicados []JobRef
	fallo      error
}

func (f *fakePublisher) Publish(_ context.Context, j JobRef, _ string) error {
	if f.fallo != nil {
		return f.fallo
	}
	f.publicados = append(f.publicados, j)
	return nil
}

// ---------------------------------------------------------------- montaje --

type banco struct {
	handler  http.Handler
	store    *fakeStore
	sessions *fakeSessions
	queue    *fakePublisher
}

func montar(t *testing.T) *banco {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := newFakeStore()
	sess := newFakeSessions()
	pub := &fakePublisher{}

	svc := NewService(store, sess, pub, audit.NewRecorder(log), Config{
		SessionTTL:     time.Hour,
		VerifyTokenTTL: 24 * time.Hour,
		PublicBaseURL:  "http://localhost:8090",
	}, log)

	mux := http.NewServeMux()
	NewAPI(svc).Routes(mux, Authenticate(svc))

	return &banco{
		handler:  httpx.Chain(mux, httpx.RequestID(), httpx.Recover(log)),
		store:    store,
		sessions: sess,
		queue:    pub,
	}
}

func (b *banco) hacer(t *testing.T, metodo, ruta, cuerpo, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if cuerpo != "" {
		body = strings.NewReader(cuerpo)
	}
	r := httptest.NewRequest(metodo, ruta, body)
	if cuerpo != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	b.handler.ServeHTTP(w, r)
	return w
}

// registrarYVerificar deja una cuenta lista para iniciar sesión.
func (b *banco) registrarYVerificar(t *testing.T, correo string) User {
	t.Helper()
	hash, err := passwords.Hash("contrasena-de-prueba")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ahora := time.Now().UTC()
	u := User{
		ID: ids.New(), Email: correo, EmailVerifiedAt: &ahora, PasswordHash: hash,
		FullName: "Persona de Prueba", Role: RoleStudent, Status: StatusActive,
		CreatedAt: ahora, UpdatedAt: ahora,
	}
	b.store.users[correo] = u
	return u
}

func decodificar(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var d map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatalf("respuesta no es JSON válido: %v\ncuerpo: %s", err, w.Body.String())
	}
	return d
}

// ---------------------------------------------------------------- pruebas --

func TestRegistroDevuelveTodosLosErroresEnProblemJSON(t *testing.T) {
	b := montar(t)

	w := b.hacer(t, "POST", "/api/v1/auth/register",
		`{"email":"no-es-un-correo","password":"corta","full_name":""}`, "")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("código = %d, se esperaba 422. Cuerpo: %s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, se esperaba application/problem+json (RFC 9457)", ct)
	}

	d := decodificar(t, w)
	errs, ok := d["errors"].([]any)
	if !ok {
		t.Fatalf("la respuesta no trae arreglo errors: %s", w.Body)
	}
	if len(errs) != 3 {
		t.Errorf("se esperaban 3 errores acumulados, llegaron %d", len(errs))
	}
	if d["trace_id"] == "" || d["trace_id"] == nil {
		t.Error("falta trace_id: sin él no se puede correlacionar con los logs")
	}
	if d["instance"] != "/api/v1/auth/register" {
		t.Errorf("instance = %v, se esperaba la ruta", d["instance"])
	}
}

// Rechazar campos desconocidos no es cosmético: es el mecanismo que hará que
// POST /progress devuelva 422 cuando el cliente mande progress_percent
// (CA-05, ADR-0012).
func TestCuerpoConCampoDesconocidoSeRechaza(t *testing.T) {
	b := montar(t)

	w := b.hacer(t, "POST", "/api/v1/auth/register",
		`{"email":"ana@mooc.local","password":"contrasena-larga","full_name":"Ana","es_admin":true}`, "")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("código = %d, se esperaba 422 por campo desconocido. Cuerpo: %s", w.Code, w.Body)
	}
	if len(b.store.users) != 0 {
		t.Error("no debe crearse ninguna cuenta cuando el cuerpo trae campos no reconocidos")
	}
}

func TestRegistroExitosoEncolaElCorreoYNoEsperaAlWorker(t *testing.T) {
	b := montar(t)

	w := b.hacer(t, "POST", "/api/v1/auth/register",
		`{"email":"Ana@Mooc.Local","password":"contrasena-larga","full_name":"Ana"}`, "")

	if w.Code != http.StatusAccepted {
		t.Fatalf("código = %d, se esperaba 202. Cuerpo: %s", w.Code, w.Body)
	}
	if _, ok := b.store.users["ana@mooc.local"]; !ok {
		t.Fatal("el correo no se normalizó antes de guardar")
	}
	if len(b.queue.publicados) != 1 {
		t.Fatalf("se esperaba 1 trabajo publicado, hubo %d", len(b.queue.publicados))
	}

	job := b.queue.publicados[0]
	if !strings.HasPrefix(job.Key, "email.send:email_verify:") {
		t.Errorf("job_key = %q; debe ser determinista y derivada del token (ADR-0008)", job.Key)
	}
	if job.Queue != "critical" {
		t.Errorf("cola = %q, un correo de verificación va en critical", job.Queue)
	}
	if b.store.db.execs == 0 {
		t.Error("no se registró nada en la transacción: falta el outbox o la auditoría")
	}
	if strings.Contains(w.Body.String(), "token") {
		t.Error("la respuesta del registro no debe exponer el token de verificación")
	}
}

// Si la cola no está disponible, el registro NO falla: el trabajo ya quedó en
// PostgreSQL y el reaper lo recuperará (ADR-0004).
func TestRegistroNoFallaSiLaColaEstaCaida(t *testing.T) {
	b := montar(t)
	b.queue.fallo = errors.New("redis no responde")

	w := b.hacer(t, "POST", "/api/v1/auth/register",
		`{"email":"ana@mooc.local","password":"contrasena-larga","full_name":"Ana"}`, "")

	if w.Code != http.StatusAccepted {
		t.Fatalf("código = %d, se esperaba 202 aunque la cola falle. Cuerpo: %s", w.Code, w.Body)
	}
	if _, ok := b.store.users["ana@mooc.local"]; !ok {
		t.Error("la cuenta debe quedar creada aunque no se pueda publicar en la cola")
	}
}

func TestRegistroConCorreoDuplicado(t *testing.T) {
	b := montar(t)
	b.registrarYVerificar(t, "ana@mooc.local")

	w := b.hacer(t, "POST", "/api/v1/auth/register",
		`{"email":"ana@mooc.local","password":"contrasena-larga","full_name":"Otra Ana"}`, "")

	if w.Code != http.StatusConflict {
		t.Fatalf("código = %d, se esperaba 409. Cuerpo: %s", w.Code, w.Body)
	}
}

// El mismo 401 y el mismo mensaje para los cuatro casos: no se revela qué
// correos existen ni en qué estado están las cuentas.
func TestLoginRechazadoNoRevelaLaCausa(t *testing.T) {
	casos := []struct {
		nombre   string
		preparar func(*testing.T, *banco)
		cuerpo   string
	}{
		{
			nombre:   "usuario inexistente",
			preparar: func(*testing.T, *banco) {},
			cuerpo:   `{"email":"nadie@mooc.local","password":"contrasena-de-prueba"}`,
		},
		{
			nombre:   "contraseña incorrecta",
			preparar: func(t *testing.T, b *banco) { b.registrarYVerificar(t, "ana@mooc.local") },
			cuerpo:   `{"email":"ana@mooc.local","password":"incorrecta"}`,
		},
		{
			nombre: "cuenta sin verificar",
			preparar: func(t *testing.T, b *banco) {
				u := b.registrarYVerificar(t, "ana@mooc.local")
				u.EmailVerifiedAt = nil
				b.store.users["ana@mooc.local"] = u
			},
			cuerpo: `{"email":"ana@mooc.local","password":"contrasena-de-prueba"}`,
		},
		{
			nombre: "cuenta suspendida",
			preparar: func(t *testing.T, b *banco) {
				u := b.registrarYVerificar(t, "ana@mooc.local")
				u.Status = StatusSuspended
				b.store.users["ana@mooc.local"] = u
			},
			cuerpo: `{"email":"ana@mooc.local","password":"contrasena-de-prueba"}`,
		},
	}

	var mensajes []string
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			b := montar(t)
			c.preparar(t, b)

			w := b.hacer(t, "POST", "/api/v1/auth/login", c.cuerpo, "")

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("código = %d, se esperaba 401. Cuerpo: %s", w.Code, w.Body)
			}
			detalle, _ := decodificar(t, w)["detail"].(string)
			mensajes = append(mensajes, detalle)
		})
	}

	for i := 1; i < len(mensajes); i++ {
		if mensajes[i] != mensajes[0] {
			t.Errorf("los mensajes difieren y filtran la causa:\n  %q (%s)\n  %q (%s)",
				mensajes[0], casos[0].nombre, mensajes[i], casos[i].nombre)
		}
	}
}

func TestLoginYMe(t *testing.T) {
	b := montar(t)
	u := b.registrarYVerificar(t, "ana@mooc.local")

	w := b.hacer(t, "POST", "/api/v1/auth/login",
		`{"email":"ana@mooc.local","password":"contrasena-de-prueba"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login: código = %d. Cuerpo: %s", w.Code, w.Body)
	}
	d := decodificar(t, w)
	token, _ := d["token"].(string)
	if token == "" {
		t.Fatal("el login no devolvió token de sesión")
	}

	w = b.hacer(t, "GET", "/api/v1/me", "", token)
	if w.Code != http.StatusOK {
		t.Fatalf("me: código = %d. Cuerpo: %s", w.Code, w.Body)
	}
	me := decodificar(t, w)
	if me["id"] != u.ID.String() {
		t.Errorf("id = %v, se esperaba %s", me["id"], u.ID)
	}
	if me["role"] != RoleStudent {
		t.Errorf("role = %v, el registro público solo crea estudiantes", me["role"])
	}
	if strings.Contains(w.Body.String(), "password") || strings.Contains(w.Body.String(), "argon2") {
		t.Error("la respuesta expone el hash de la contraseña")
	}
}

func TestEndpointsAutenticados(t *testing.T) {
	casos := []struct {
		nombre string
		bearer string
		codigo int
	}{
		{"sin cabecera", "", http.StatusUnauthorized},
		{"token inexistente", "token-que-no-existe", http.StatusUnauthorized},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			b := montar(t)
			w := b.hacer(t, "GET", "/api/v1/me", "", c.bearer)
			if w.Code != c.codigo {
				t.Fatalf("código = %d, se esperaba %d. Cuerpo: %s", w.Code, c.codigo, w.Body)
			}
			if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
				t.Errorf("Content-Type = %q, incluso los 401 usan problem+json", ct)
			}
		})
	}
}

// Cerrar sesión borra la clave: la petición siguiente falla sin ventana de
// gracia. Es lo que el segmento 1 de la demostración tiene que mostrar.
func TestLogoutRevocaLaSesionDeInmediato(t *testing.T) {
	b := montar(t)
	b.registrarYVerificar(t, "ana@mooc.local")

	w := b.hacer(t, "POST", "/api/v1/auth/login",
		`{"email":"ana@mooc.local","password":"contrasena-de-prueba"}`, "")
	token := decodificar(t, w)["token"].(string)

	if w := b.hacer(t, "POST", "/api/v1/auth/logout", "", token); w.Code != http.StatusNoContent {
		t.Fatalf("logout: código = %d. Cuerpo: %s", w.Code, w.Body)
	}

	if w := b.hacer(t, "GET", "/api/v1/me", "", token); w.Code != http.StatusUnauthorized {
		t.Fatalf("tras cerrar sesión: código = %d, se esperaba 401", w.Code)
	}
	if _, quedó := b.sessions.porToken[token]; quedó {
		t.Error("la sesión sigue en el almacén tras cerrarla")
	}
}

func TestVerificarCorreoConTokenInvalido(t *testing.T) {
	b := montar(t)

	for _, cuerpo := range []string{`{"token":"inventado"}`, `{"token":""}`} {
		w := b.hacer(t, "POST", "/api/v1/auth/verify-email", cuerpo, "")
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("cuerpo %s: código = %d, se esperaba 422", cuerpo, w.Code)
		}
	}
}

func TestFlujoCompletoDeVerificacion(t *testing.T) {
	b := montar(t)

	w := b.hacer(t, "POST", "/api/v1/auth/register",
		`{"email":"ana@mooc.local","password":"contrasena-larga","full_name":"Ana"}`, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("registro: código = %d. Cuerpo: %s", w.Code, w.Body)
	}

	// El token en claro solo existe en el payload del trabajo, nunca en la
	// respuesta ni en la tabla de tokens (que guarda su hash).
	token, _ := b.queue.publicados[0].Payload["token"].(string)
	if token == "" {
		t.Fatal("el trabajo de correo no lleva el token")
	}

	w = b.hacer(t, "POST", "/api/v1/auth/verify-email", `{"token":"`+token+`"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("verificación: código = %d. Cuerpo: %s", w.Code, w.Body)
	}
	if !b.store.users["ana@mooc.local"].IsVerified() {
		t.Error("la cuenta quedó sin marcar como verificada")
	}

	// Un token de un solo uso no se reutiliza.
	w = b.hacer(t, "POST", "/api/v1/auth/verify-email", `{"token":"`+token+`"}`, "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("reutilización: código = %d, se esperaba 422", w.Code)
	}
}
