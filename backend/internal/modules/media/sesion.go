package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Sesión de reproducción (ADR-0015, D2).
//
// Es la credencial con la que un estudiante reproduce un recurso, emitida una
// sola vez por sesión en lugar de firmar cada segmento. La diferencia importa
// cuando delante hay un CDN: un video de cuarenta minutos son unos cuatrocientos
// segmentos, y cuatrocientas firmas convierten a la API en el cuello de botella
// que el CDN venía a evitar.
//
// Lo que cambia en GCP es **quién** emite la credencial y en qué forma viaja
// —cookie firmada de Cloud CDN en vez de URL firmada—, no este contrato: por eso
// la respuesta lleva `delivery` y `scope`.

// Formas de entrega. La Entrega 1 solo usa la primera.
const (
	EntregaURLFirmada    = "signed_url"
	EntregaCookieFirmada = "signed_cookie"
)

// AlcanceAsset: la credencial vale para un recurso. La alternativa —por versión
// de curso— obligaría a duplicar los derivados HLS en cada versión nueva, que es
// justo lo que el `stable_id` del ADR-0007 evita.
const AlcanceAsset = "asset"

// SesionMedia es lo que se devuelve al abrir la reproducción.
type SesionMedia struct {
	ManifestURL string
	Delivery    string
	Scope       string
	ExpiresAt   time.Time
}

// Inscripciones es el puerto hacia `learning`. media no importa ese módulo: le
// pregunta lo que necesita y la composición hace de puente (ADR-0001).
type Inscripciones interface {
	// AssetDeRecurso comprueba que el actor tenga derecho sobre la inscripción
	// y devuelve el asset del recurso, buscándolo por su `stable_id`.
	AssetDeRecurso(ctx context.Context, enrollmentID, resourceStableID uuid.UUID, a Actor) (uuid.UUID, error)
}

// ConInscripciones y ConBasePublica los enchufa la API. El worker no abre
// sesiones de reproducción.
func (s *Service) ConInscripciones(i Inscripciones) *Service {
	s.inscripciones = i
	return s
}

func (s *Service) ConBasePublica(u string) *Service {
	s.basePublica = strings.TrimRight(u, "/")
	return s
}

// AbrirSesion emite la credencial tras comprobar la inscripción (CA-06).
//
// El recurso se pide por `stable_id`, no por `id` de fila: así el enlace que
// tenga un estudiante sigue funcionando cuando el profesor publica una versión
// nueva del curso (ADR-0007).
func (s *Service) AbrirSesion(ctx context.Context, enrollmentID, resourceStableID uuid.UUID,
	a Actor) (SesionMedia, error) {

	if s.inscripciones == nil || s.repro == nil {
		return SesionMedia{}, fmt.Errorf("media: falta configurar la sesión de reproducción")
	}

	assetID, err := s.inscripciones.AssetDeRecurso(ctx, enrollmentID, resourceStableID, a)
	if err != nil {
		return SesionMedia{}, err
	}

	// Se lee sin `guard`: quien manda aquí es la inscripción, no la propiedad
	// del archivo. El dueño del asset es el profesor, no el estudiante.
	asset, err := s.store.AssetPorID(ctx, s.store.DB(), assetID)
	if err != nil {
		return SesionMedia{}, err
	}
	if !HLSAplicable(asset.Kind) {
		return SesionMedia{}, ErrNotFound
	}
	if asset.Status != EstadoListo {
		return SesionMedia{}, ErrNoListo
	}

	token, err := s.repro.Emitir(ctx, assetID, TTLReproduccion)
	if err != nil {
		return SesionMedia{}, err
	}

	return SesionMedia{
		ManifestURL: fmt.Sprintf("%s/api/v1/media-sessions/%s/master.m3u8", s.basePublica, token),
		Delivery:    EntregaURLFirmada,
		Scope:       AlcanceAsset,
		ExpiresAt:   time.Now().UTC().Add(TTLReproduccion),
	}, nil
}

// MasterDeSesion sirve el manifiesto maestro de una sesión abierta.
//
// No lleva sesión de usuario: autoriza el testigo que va en la ruta, igual que
// en las playlists de variante. Es lo que permite que un reproductor pida el
// manifiesto sin saber nada de cabeceras.
func (s *Service) MasterDeSesion(ctx context.Context, token string) (string, error) {
	if s.repro == nil {
		return "", fmt.Errorf("media: no hay almacén de credenciales de reproducción")
	}
	assetID, err := s.repro.Resolver(ctx, token)
	if err != nil {
		return "", err
	}

	derivados, err := s.store.DerivadosDeAsset(ctx, s.store.DB(), assetID)
	if err != nil {
		return "", err
	}
	variantes := variantesDe(derivados)
	if len(variantes) == 0 {
		return "", ErrNoListo
	}

	// Las variantes se referencian con rutas absolutas de la API, no relativas
	// a esta: la playlist de variante vive bajo el asset y lleva el testigo en
	// la consulta.
	return MasterM3U8(variantes, func(v Variante) string {
		return fmt.Sprintf("%s/api/v1/assets/%s/hls/%s.m3u8?t=%s",
			s.basePublica, assetID, v.Nombre, token)
	}), nil
}
