package media

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"mooc/backend/internal/platform/dbx"
	"testing"
	"time"
)

type sessionStore struct {
	Store
	asset Asset
}

func (s sessionStore) DB() dbx.DB { return nil }
func (s sessionStore) AssetPorID(context.Context, dbx.DB, uuid.UUID) (Asset, error) {
	return s.asset, nil
}

type sessionEnrollment struct {
	asset uuid.UUID
	err   error
}

func (s sessionEnrollment) AssetDeRecurso(context.Context, uuid.UUID, uuid.UUID, Actor) (uuid.UUID, error) {
	return s.asset, s.err
}

type sessionObjects struct {
	Almacen
	url   string
	calls *int
}

func (s sessionObjects) PresignGet(context.Context, string, string, time.Duration) (string, error) {
	*s.calls++
	return s.url, nil
}
func TestSesionArchivoRespetaInscripcionEstadoYDestino(t *testing.T) {
	for _, c := range []struct {
		name, status, url string
		denied            bool
		wantError         bool
	}{
		{"PDF limpio", EstadoLimpio, "https://objects.example/private?signature=x", false, false},
		{"imagen lista", EstadoListo, "https://objects.example/private?signature=x", false, false},
		{"sin inscripción", EstadoListo, "https://objects.example/private", true, true},
		{"infectado", EstadoInfectado, "https://objects.example/private", false, true},
		{"en escaneo", EstadoEscaneando, "https://objects.example/private", false, true},
		{"host ajeno", EstadoListo, "https://evil.example/private", false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			count := 0
			id := uuid.New()
			var denied error
			if c.denied {
				denied = ErrProhibido
			}
			s := &Service{store: sessionStore{asset: Asset{ID: id, Kind: TipoPDF, Status: c.status, OriginalKey: "private"}}, almacen: sessionObjects{url: c.url, calls: &count}, inscripciones: sessionEnrollment{asset: id, err: denied}, repro: sessionPlayback{}, hostPermitido: "objects.example", buckets: Buckets{Originales: "originals"}}
			result, err := s.AbrirSesion(context.Background(), uuid.New(), uuid.New(), Actor{})
			if (err != nil) != c.wantError {
				t.Fatalf("error=%v", err)
			}
			if c.denied && !errors.Is(err, ErrProhibido) {
				t.Fatal("debe conservar rechazo de inscripción")
			}
			if c.denied || c.status == EstadoInfectado || c.status == EstadoEscaneando {
				if count != 0 {
					t.Fatal("no se debe firmar contenido no autorizado")
				}
			}
			if !c.wantError && (result.ContentURL != c.url || result.ManifestURL != c.url || result.Delivery != EntregaURLFirmada) {
				t.Fatalf("sesión incorrecta: %+v", result)
			}
		})
	}
}

type sessionPlayback struct{ Reproduccion }
