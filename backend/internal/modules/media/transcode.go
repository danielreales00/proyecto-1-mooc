package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/jobs"
)

// Transcodificación a HLS: el trabajo media.transcode_hls (ADR-0011, RF-06).

// Medios son los metadatos reales del original, leídos de sus bytes.
type Medios struct {
	DuracionSegundos float64
	Ancho, Alto      int
	TieneVideo       bool
	TieneAudio       bool
}

// Transcodificador es el puerto hacia FFmpeg. El servicio no sabe qué binario
// hay detrás ni con qué argumentos se invoca.
type Transcodificador interface {
	Inspeccionar(ctx context.Context, ruta string) (Medios, error)
	TranscodificarHLS(ctx context.Context, entrada, destino string, v Variante) error
	Poster(ctx context.Context, entrada, salida string, segundo int) error
}

// ErrOriginalIlegible lo devuelve el transcodificador cuando el archivo no se
// puede decodificar. Es un fallo del contenido, no del sistema: reintentarlo
// tres veces daría el mismo resultado y dejaría un trabajo en la DLQ, que debe
// significar "hay algo roto aquí dentro".
var ErrOriginalIlegible = errors.New("media: el original no se puede decodificar")

// TiempoMaximoDeTranscodificacion acota el trabajo. Sin tope, un archivo
// patológico deja un worker ocupado indefinidamente.
const TiempoMaximoDeTranscodificacion = 30 * time.Minute

// Transcodificar es el trabajo media.transcode_hls.
//
// Idempotente por construcción (CA-03): las claves de los derivados son
// deterministas, así que reejecutar sobrescribe los mismos objetos, y el
// registro en la base se reemplaza entero dentro de una transacción. Dos
// entregas del mismo trabajo dejan exactamente un juego de derivados.
//
// El original nunca se modifica: se lee a un temporal que muere con el trabajo
// (CA-02, ADR-0001).
func (s *Service) Transcodificar(ctx context.Context, data map[string]any) error {
	raw, _ := data["asset_id"].(string)
	id, err := uuid.Parse(raw)
	if err != nil {
		return fmt.Errorf("media.transcode_hls con asset_id inválido: %q", raw)
	}

	asset, err := s.store.AssetPorID(ctx, s.store.DB(), id)
	if err != nil {
		return err
	}

	if !HLSAplicable(asset.Kind) {
		s.log.Info("el asset no se transcodifica", "asset_id", id, "kind", asset.Kind)
		return nil
	}
	switch asset.Status {
	case EstadoListo:
		// Ya está: solo se rehace si los derivados no están donde deberían,
		// que es lo que repara una reejecución tras perder el bucket.
		ds, err := s.store.DerivadosDeAsset(ctx, s.store.DB(), id)
		if err != nil {
			return err
		}
		if len(ds) > 0 {
			s.log.Info("el asset ya estaba transcodificado", "asset_id", id, "derivados", len(ds))
			return nil
		}
	case EstadoLimpio, EstadoProcesando, EstadoFallido:
		// Adelante.
	default:
		s.log.Warn("transcodificación descartada por el estado del asset",
			"asset_id", id, "status", asset.Status)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, TiempoMaximoDeTranscodificacion)
	defer cancel()

	if err := s.marcar(ctx, &asset, EstadoProcesando, nil); err != nil {
		return err
	}

	derivados, medios, err := s.producirHLS(ctx, asset)
	if err != nil {
		if errors.Is(err, ErrOriginalIlegible) {
			motivo := err.Error()
			s.log.Warn("original ilegible", "asset_id", id, "error", motivo)
			if err := s.marcar(ctx, &asset, EstadoFallido, &motivo); err != nil {
				return err
			}
			// Sin error: el trabajo hizo lo suyo, y lo que falla es el archivo.
			return nil
		}
		return err
	}

	asset.DurationSeconds = &medios.DuracionSegundos
	if medios.TieneVideo {
		ancho, alto := medios.Ancho, medios.Alto
		asset.Width, asset.Height = &ancho, &alto
	}
	asset.Status = EstadoListo
	asset.LastError = nil
	asset.UpdatedAt = time.Now().UTC()

	s.log.Info("asset transcodificado", "asset_id", id,
		"variantes", len(derivados)-1, "duracion_s", medios.DuracionSegundos)

	return s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.ActualizarAsset(ctx, tx, asset); err != nil {
			return err
		}
		if err := s.store.ReemplazarDerivados(ctx, tx, asset.ID, derivados); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &asset.OwnerID, ActorRole: "teacher", Action: "media.transcoded",
			EntityType: "asset", EntityID: &asset.ID,
			Metadata: map[string]any{
				"derivados": len(derivados),
				"duracion":  medios.DuracionSegundos,
				"alto":      medios.Alto,
			},
		})
	})
}

// producirHLS descarga el original, lo inspecciona, genera la escalera y sube
// cada salida al bucket de derivados.
func (s *Service) producirHLS(ctx context.Context, asset Asset) ([]Derivado, Medios, error) {
	if s.ffmpeg == nil {
		return nil, Medios{}, errors.New("media: no hay transcodificador configurado")
	}

	// Los temporales viven en el contenedor y se borran pase lo que pase: el
	// worker no acumula estado (ADR-0001).
	dir, err := os.MkdirTemp("", "hls-")
	if err != nil {
		return nil, Medios{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	original := filepath.Join(dir, "original")
	if err := s.descargar(ctx, s.buckets.Originales, asset.OriginalKey, original); err != nil {
		return nil, Medios{}, err
	}

	medios, err := s.ffmpeg.Inspeccionar(ctx, original)
	if err != nil {
		return nil, Medios{}, envolverIlegible(err)
	}

	variantes := VariantesPara(medios.Alto, medios.TieneVideo)
	if len(variantes) == 0 {
		return nil, Medios{}, fmt.Errorf("%w: sin dimensiones utilizables", ErrOriginalIlegible)
	}

	idStr := asset.ID.String()
	derivados := make([]Derivado, 0, len(variantes)+2)

	for _, v := range variantes {
		destino := filepath.Join(dir, v.Nombre)
		if err := s.ffmpeg.TranscodificarHLS(ctx, original, destino, v); err != nil {
			return nil, Medios{}, envolverIlegible(err)
		}

		subidos, err := s.subirDirectorio(ctx, destino, PrefijoVariante(idStr, v.Nombre))
		if err != nil {
			return nil, Medios{}, err
		}
		playlist, ok := subidos[ClavePlaylist(idStr, v.Nombre)]
		if !ok {
			return nil, Medios{}, fmt.Errorf("la variante %s no produjo playlist", v.Nombre)
		}
		derivados = append(derivados, Derivado{
			Kind: DerivadoVariante, Variante: v.Nombre,
			Key: ClavePlaylist(idStr, v.Nombre), Bytes: playlist,
		})
	}

	// El master que se guarda referencia a las variantes con rutas relativas:
	// es el manifiesto tal cual, sin firmas que caduquen. El que sirve la API
	// se arma en cada petición (ver MasterFirmado).
	master := MasterM3U8(variantes, func(v Variante) string { return v.Nombre + "/index.m3u8" })
	claveMaster := ClaveMaster(idStr)
	if err := s.almacen.Subir(ctx, s.buckets.Derivados, claveMaster,
		"application/vnd.apple.mpegurl", []byte(master)); err != nil {
		return nil, Medios{}, err
	}
	derivados = append(derivados, Derivado{
		Kind: DerivadoMaster, Key: claveMaster, Bytes: int64(len(master)),
	})

	if medios.TieneVideo {
		poster := filepath.Join(dir, "poster.jpg")
		if err := s.ffmpeg.Poster(ctx, original, poster, PosterSegundo); err != nil {
			// Sin portada el vídeo se reproduce igual: se avisa y se sigue.
			s.log.Warn("no se pudo extraer la portada", "asset_id", asset.ID, "error", err)
		} else {
			bytes, err := s.subirArchivo(ctx, poster, s.buckets.Derivados,
				ClavePoster(idStr), "image/jpeg")
			if err != nil {
				return nil, Medios{}, err
			}
			derivados = append(derivados, Derivado{
				Kind: DerivadoPoster, Key: ClavePoster(idStr), Bytes: bytes,
			})
		}
	}

	return derivados, medios, nil
}

// descargar trae el original a un archivo local. Se transmite en vez de
// cargarlo en memoria: un vídeo de 5 GiB no cabe en el heap del worker.
func (s *Service) descargar(ctx context.Context, bucket, key, destino string) error {
	origen, err := s.almacen.Abrir(ctx, bucket, key)
	if err != nil {
		return err
	}
	defer origen.Close()

	f, err := os.OpenFile(filepath.Clean(destino), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, origen); err != nil {
		return fmt.Errorf("descargar el original: %w", err)
	}
	return f.Sync()
}

// subirDirectorio sube todo lo que FFmpeg dejó en una carpeta y devuelve el
// tamaño de cada objeto, indexado por su clave.
func (s *Service) subirDirectorio(ctx context.Context, dir, prefijo string) (map[string]int64, error) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// Orden estable: el log de una reejecución se puede comparar con el de la
	// anterior.
	sort.Slice(entradas, func(i, j int) bool { return entradas[i].Name() < entradas[j].Name() })

	out := make(map[string]int64, len(entradas))
	for _, e := range entradas {
		if e.IsDir() {
			continue
		}
		clave := prefijo + e.Name()
		n, err := s.subirArchivo(ctx, filepath.Join(dir, e.Name()),
			s.buckets.Derivados, clave, tipoPorExtension(e.Name()))
		if err != nil {
			return nil, err
		}
		out[clave] = n
	}
	return out, nil
}

func (s *Service) subirArchivo(ctx context.Context, ruta, bucket, clave, tipo string) (int64, error) {
	datos, err := os.ReadFile(filepath.Clean(ruta))
	if err != nil {
		return 0, err
	}
	if err := s.almacen.Subir(ctx, bucket, clave, tipo, datos); err != nil {
		return 0, err
	}
	return int64(len(datos)), nil
}

func tipoPorExtension(nombre string) string {
	switch strings.ToLower(filepath.Ext(nombre)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}

// envolverIlegible traduce el error del adaptador al del dominio sin que el
// servicio tenga que importar el paquete del adaptador (ADR-0001).
func envolverIlegible(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "no se puede decodificar") {
		return fmt.Errorf("%w: %s", ErrOriginalIlegible, err.Error())
	}
	return err
}

func (s *Service) marcar(ctx context.Context, a *Asset, estado string, motivo *string) error {
	a.Status = estado
	a.LastError = motivo
	a.UpdatedAt = time.Now().UTC()
	return s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		return s.store.ActualizarAsset(ctx, tx, *a)
	})
}

// HLSAplicable dice si un tipo de recurso pasa por la transcodificación. Un PDF
// o una imagen se sirven tal cual.
func HLSAplicable(kind string) bool { return kind == TipoVideo || kind == TipoAudio }

// EncolarTranscodificacion registra el trabajo en la misma transacción que deja
// el asset limpio y lo publica después del commit: el patrón outbox del
// ADR-0004. La clave lleva la versión de la escalera, como pide el catálogo de
// trabajos: cambiarla es un trabajo distinto, no una reejecución del mismo.
func (s *Service) EncolarTranscodificacion(ctx context.Context, tx dbx.DB, assetID uuid.UUID) (string, error) {
	clave := fmt.Sprintf("%s:%s:%s", jobs.TypeMediaTranscode, assetID, VersionEscalera)
	return clave, jobs.Record(ctx, tx, jobs.Job{
		Key: clave, Type: jobs.TypeMediaTranscode, Queue: jobs.QueueBulk,
		Payload: map[string]any{"asset_id": assetID.String()},
	})
}

// VersionEscalera cambia cuando cambian los peldaños del ADR-0011.
const VersionEscalera = "v1"
