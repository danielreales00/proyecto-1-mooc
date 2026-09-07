// Package idempotency implementa la cabecera `Idempotency-Key` del ADR-0008.
//
// Reintentar una operación no repetible —inscribirse, publicar, enviar un
// intento— no debe crear una segunda. El cliente elige una clave; si repite la
// petición con la misma clave y el mismo cuerpo, se le devuelve la respuesta
// que ya se calculó, sin volver a ejecutar nada.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/platform/httpx"
	"mooc/backend/internal/platform/problem"
)

// TamañoMaximoRespuesta acota lo que se guarda. Una respuesta más grande se
// ejecuta igual, pero no se memoriza: repetirla volvería a ejecutarla.
const TamañoMaximoRespuesta = 256 * 1024

type Middleware struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

func New(db *pgxpool.Pool, log *slog.Logger) *Middleware {
	return &Middleware{db: db, log: log}
}

// grabadora captura la respuesta para poder guardarla.
type grabadora struct {
	http.ResponseWriter
	status int
	cuerpo bytes.Buffer
	exceso bool
}

func (g *grabadora) WriteHeader(c int) {
	g.status = c
	g.ResponseWriter.WriteHeader(c)
}

func (g *grabadora) Write(b []byte) (int, error) {
	if g.status == 0 {
		g.status = http.StatusOK
	}
	if g.cuerpo.Len()+len(b) <= TamañoMaximoRespuesta {
		g.cuerpo.Write(b)
	} else {
		g.exceso = true
	}
	return g.ResponseWriter.Write(b)
}

// Requerida exige la cabecera y memoriza la respuesta.
func (m *Middleware) Requerida() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clave := r.Header.Get("Idempotency-Key")
			if len(clave) < 8 || len(clave) > 255 {
				problem.Write(w, r, problem.New(http.StatusBadRequest, "idempotency_key_required",
					"Falta la clave de idempotencia",
					"Esta operación exige la cabecera `Idempotency-Key` con un valor de entre 8 y 255 caracteres."))
				return
			}

			p, ok := httpx.PrincipalFrom(r.Context())
			if !ok {
				problem.Write(w, r, problem.Unauthorized("Se requiere una sesión activa."))
				return
			}

			crudo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if err != nil {
				problem.Write(w, r, problem.Validation("No se pudo leer el cuerpo."))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(crudo))
			hash := sha256.Sum256(crudo)

			// El patrón de la ruta, no la URL: dos cursos distintos con la
			// misma clave son operaciones distintas, pero la ruta es la misma.
			endpoint := r.Method + " " + r.Pattern

			estado, respStatus, respCuerpo, err := m.reclamar(r.Context(), clave, p.UserID, endpoint, hash[:])
			if err != nil {
				m.log.Error("el registro de idempotencia falló", "error", err)
				// Sin registro no hay garantía, pero negar el servicio es peor:
				// se ejecuta y se avisa.
				next.ServeHTTP(w, r)
				return
			}

			switch estado {
			case reclamado:
				g := &grabadora{ResponseWriter: w}
				next.ServeHTTP(g, r)
				if g.status == 0 {
					g.status = http.StatusOK
				}
				m.guardar(r.Context(), clave, p.UserID, endpoint, g)

			case repetida:
				w.Header().Set("Idempotency-Replayed", "true")
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(respStatus)
				_, _ = w.Write(respCuerpo)

			case enCurso:
				problem.Write(w, r, problem.Conflict("request_in_flight",
					"Una petición con esta misma clave se está procesando ahora mismo."))

			case cuerpoDistinto:
				problem.Write(w, r, problem.New(http.StatusUnprocessableEntity,
					"idempotency_key_reuse", "Clave de idempotencia reutilizada",
					"Ya se usó esta clave con un cuerpo distinto. Usa una clave nueva."))
			}
		})
	}
}

type resultado int

const (
	reclamado resultado = iota
	repetida
	enCurso
	cuerpoDistinto
)

// reclamar intenta apropiarse de la clave con un INSERT. La unicidad de
// (user_id, endpoint, key) es la que decide quién gana; consultar antes de
// insertar tendría una carrera justo bajo la concurrencia que esto protege.
func (m *Middleware) reclamar(ctx context.Context, clave string, user uuid.UUID,
	endpoint string, hash []byte) (resultado, int, []byte, error) {

	tag, err := m.db.Exec(ctx, `
		INSERT INTO platform.idempotency_keys (key, user_id, endpoint, request_hash, status)
		VALUES ($1,$2,$3,$4,'in_progress')
		ON CONFLICT (user_id, endpoint, key) DO NOTHING`, clave, user, endpoint, hash)
	if err != nil {
		return 0, 0, nil, err
	}
	if tag.RowsAffected() == 1 {
		return reclamado, 0, nil, nil
	}

	var estado string
	var hashGuardado []byte
	var status *int
	var cuerpo []byte
	err = m.db.QueryRow(ctx, `
		SELECT status, request_hash, response_status, response_body
		  FROM platform.idempotency_keys
		 WHERE user_id=$1 AND endpoint=$2 AND key=$3`, user, endpoint, clave).
		Scan(&estado, &hashGuardado, &status, &cuerpo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Se podó entre el INSERT y la lectura; se ejecuta.
			return reclamado, 0, nil, nil
		}
		return 0, 0, nil, err
	}

	if !bytes.Equal(hashGuardado, hash) {
		return cuerpoDistinto, 0, nil, nil
	}
	if estado != "completed" || status == nil {
		return enCurso, 0, nil, nil
	}
	return repetida, *status, cuerpo, nil
}

func (m *Middleware) guardar(ctx context.Context, clave string, user uuid.UUID,
	endpoint string, g *grabadora) {

	// Solo se memorizan las respuestas correctas. Repetir un error debe volver
	// a intentarlo: el fallo puede haber sido transitorio.
	if g.status >= 400 || g.exceso {
		_, err := m.db.Exec(context.WithoutCancel(ctx), `
			DELETE FROM platform.idempotency_keys
			 WHERE user_id=$1 AND endpoint=$2 AND key=$3`, user, endpoint, clave)
		if err != nil {
			m.log.Error("no se pudo liberar la clave de idempotencia", "error", err)
		}
		return
	}

	if _, err := m.db.Exec(context.WithoutCancel(ctx), `
		UPDATE platform.idempotency_keys
		   SET status='completed', response_status=$4, response_body=$5
		 WHERE user_id=$1 AND endpoint=$2 AND key=$3`,
		user, endpoint, clave, g.status, g.cuerpo.Bytes()); err != nil {
		m.log.Error("no se pudo guardar la respuesta idempotente", "error", err)
	}
}

// Podar borra las claves vencidas. Se conservan 24 h (ADR-0008).
func (m *Middleware) Podar(ctx context.Context) (int64, error) {
	tag, err := m.db.Exec(ctx,
		`DELETE FROM platform.idempotency_keys WHERE created_at < now() - interval '24 hours'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
