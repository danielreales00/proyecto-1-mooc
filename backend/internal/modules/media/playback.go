package media

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// Entrega de HLS con el bucket cerrado (ADR-0005, RF-06).
//
// El problema que resuelve este archivo: un manifiesto HLS es una cadena de
// documentos —maestro, playlist de variante, segmentos— y el reproductor pide
// cada uno por su cuenta, sin cabecera de autorización. Si los segmentos
// vivieran en un bucket público bastaría con servir el maestro, pero el bucket
// de derivados es privado y debe seguir siéndolo.
//
// La salida es servir los manifiestos desde la API y firmar lo que hay dentro:
// el maestro apunta a las playlists de esta misma API, y cada playlist se
// reescribe en el momento con URLs firmadas hacia el almacén. Los bytes siguen
// sin pasar por la API —solo los manifiestos, que son texto.

// TTLReproduccion es lo que dura una credencial de reproducción. Corta a
// propósito: es una URL que viaja en texto plano dentro de un manifiesto.
const TTLReproduccion = 30 * time.Minute

// TTLSegmento es la validez de cada URL firmada de segmento. Debe cubrir lo que
// el reproductor tarde en consumir la playlist, no la sesión entera.
const TTLSegmento = 15 * time.Minute

// Reproduccion es el puerto de las credenciales de reproducción: un testigo
// opaco, guardado fuera del proceso para que cualquier instancia de la API lo
// pueda resolver (CE-01, mismo patrón que las sesiones del ADR-0006).
type Reproduccion interface {
	Emitir(ctx context.Context, assetID uuid.UUID, ttl time.Duration) (string, error)
	Resolver(ctx context.Context, token string) (uuid.UUID, error)
}

// ConReproduccion enchufa el almacén de credenciales. Sin él, la entrega de HLS
// responde ErrNoDisponible en vez de fallar de formas raras.
func (s *Service) ConReproduccion(r Reproduccion) *Service {
	s.repro = r
	return s
}

// ErrNoListo lo devuelve la entrega cuando el asset existe pero todavía no ha
// pasado por la transcodificación.
var ErrNoListo = fmt.Errorf("media: el asset aún no está listo para reproducirse")

// MasterFirmado devuelve el manifiesto maestro que sirve la API.
//
// Las variantes se referencian con rutas relativas: el reproductor las resuelve
// contra la URL del maestro, así que el manifiesto no depende de en qué dominio
// esté desplegada la API.
func (s *Service) MasterFirmado(ctx context.Context, assetID uuid.UUID, a Actor) (string, error) {
	asset, err := s.guard(ctx, assetID, a)
	if err != nil {
		return "", err
	}
	if asset.Status != EstadoListo {
		return "", ErrNoListo
	}
	if s.repro == nil {
		return "", fmt.Errorf("media: no hay almacén de credenciales de reproducción")
	}

	derivados, err := s.store.DerivadosDeAsset(ctx, s.store.DB(), assetID)
	if err != nil {
		return "", err
	}
	variantes := variantesDe(derivados)
	if len(variantes) == 0 {
		return "", ErrNoListo
	}

	token, err := s.repro.Emitir(ctx, assetID, TTLReproduccion)
	if err != nil {
		return "", err
	}

	return MasterM3U8(variantes, func(v Variante) string {
		return fmt.Sprintf("%s.m3u8?t=%s", v.Nombre, token)
	}), nil
}

// PlaylistFirmada devuelve la playlist de una variante con cada segmento
// convertido en una URL firmada.
//
// La autorización la lleva el testigo, no una cabecera: es lo único que un
// reproductor puede transportar entre documentos de un manifiesto. Por eso dura
// poco y no identifica a nadie: solo dice "quien lo tenga puede ver este
// asset".
func (s *Service) PlaylistFirmada(ctx context.Context, token, variante string) (string, error) {
	if s.repro == nil {
		return "", fmt.Errorf("media: no hay almacén de credenciales de reproducción")
	}
	assetID, err := s.repro.Resolver(ctx, token)
	if err != nil {
		return "", err
	}

	// La variante se comprueba contra lo que hay registrado, no contra lo que
	// venga en la ruta: así no hay forma de construir una clave de objeto
	// arbitraria desde la URL.
	derivados, err := s.store.DerivadosDeAsset(ctx, s.store.DB(), assetID)
	if err != nil {
		return "", err
	}
	conocida := false
	for _, d := range derivados {
		if d.Kind == DerivadoVariante && d.Variante == variante {
			conocida = true
			break
		}
	}
	if !conocida {
		return "", ErrNotFound
	}

	idStr := assetID.String()
	lector, err := s.almacen.Abrir(ctx, s.buckets.Derivados, ClavePlaylist(idStr, variante))
	if err != nil {
		return "", err
	}
	defer lector.Close()

	crudo, err := io.ReadAll(lector)
	if err != nil {
		return "", err
	}

	prefijo := PrefijoVariante(idStr, variante)
	return ReescribirSegmentos(string(crudo), func(nombre string) (string, error) {
		return s.almacen.PresignGet(ctx, s.buckets.Derivados, prefijo+nombre, TTLSegmento)
	})
}

// Derivados lista lo que produjo la transcodificación. Es la evidencia de que
// el trabajo hizo algo: qué variantes hay y cuánto pesan.
func (s *Service) Derivados(ctx context.Context, assetID uuid.UUID, a Actor) ([]Derivado, error) {
	if _, err := s.guard(ctx, assetID, a); err != nil {
		return nil, err
	}
	return s.store.DerivadosDeAsset(ctx, s.store.DB(), assetID)
}

// variantesDe reconstruye la escalera a partir de lo registrado, para que el
// manifiesto anuncie exactamente lo que existe en el almacén y no lo que la
// tabla del ADR dice que debería existir.
func variantesDe(ds []Derivado) []Variante {
	porNombre := make(map[string]Variante, len(Escalera)+1)
	for _, v := range Escalera {
		porNombre[v.Nombre] = v
	}
	porNombre[VarianteAudio.Nombre] = VarianteAudio

	var out []Variante
	for _, d := range ds {
		if d.Kind != DerivadoVariante {
			continue
		}
		if v, ok := porNombre[d.Variante]; ok {
			out = append(out, v)
			continue
		}
		// Variante a medida: un original más bajo que el primer peldaño.
		v := Escalera[0]
		v.Nombre = d.Variante
		v.Ancho, v.Alto = 0, 0
		out = append(out, v)
	}
	return out
}
