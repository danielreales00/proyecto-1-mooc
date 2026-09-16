package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/jobs"
)

// Escaneo antimalware: el trabajo media.scan (ADR-0011, RF-05).
//
// Va entre la verificación y la transcodificación, y esa posición es la
// decisión: **no se procesa lo que no está limpio**. Un archivo infectado no
// llega nunca a FFmpeg.

// Veredicto es lo que responde el antivirus.
type Veredicto struct {
	Limpio bool
	// Firma es el nombre de la amenaza cuando no está limpio.
	Firma string
}

// Antivirus es el puerto hacia el escáner. Recibe un lector para que el archivo
// se transmita por trozos: un vídeo de gigabytes no cabe en memoria.
type Antivirus interface {
	Escanear(ctx context.Context, r io.Reader) (Veredicto, error)
}

// ErrLimiteDelEscaner: el archivo es más grande de lo que el escáner acepta.
// Lo devuelve el adaptador y aquí se traduce a un rechazo explícito, no a un
// aprobado silencioso.
var ErrLimiteDelEscaner = errors.New("media: el archivo supera el límite del escáner")

// ConAntivirus se lo enchufa el worker. La API no escanea.
func (s *Service) ConAntivirus(a Antivirus) *Service {
	s.antivirus = a
	return s
}

// TiempoMaximoDeEscaneo acota el trabajo.
const TiempoMaximoDeEscaneo = 15 * time.Minute

// Escanear es el trabajo media.scan.
//
// Idempotente: volver a escanear el mismo objeto da el mismo veredicto, y el
// reclamo atómico del runner impide dos ejecuciones a la vez (ADR-0008).
//
// **Falla cerrado.** Si clamd no responde, el trabajo devuelve error y se
// reintenta: el asset se queda en `scanning` y no pasa a transcodificarse. Un
// escáner caído no debe convertirse en un permiso de paso.
func (s *Service) Escanear(ctx context.Context, data map[string]any) error {
	raw, _ := data["asset_id"].(string)
	id, err := uuid.Parse(raw)
	if err != nil {
		return fmt.Errorf("media.scan con asset_id inválido: %q", raw)
	}

	asset, err := s.store.AssetPorID(ctx, s.store.DB(), id)
	if err != nil {
		return err
	}
	switch asset.Status {
	case EstadoEscaneando:
		// Adelante.
	case EstadoLimpio, EstadoProcesando, EstadoListo, EstadoInfectado:
		s.log.Info("el asset ya estaba escaneado", "asset_id", id, "status", asset.Status)
		return nil
	default:
		s.log.Warn("escaneo descartado por el estado del asset",
			"asset_id", id, "status", asset.Status)
		return nil
	}

	if s.antivirus == nil {
		return errors.New("media: no hay antivirus configurado")
	}

	ctx, cancel := context.WithTimeout(ctx, TiempoMaximoDeEscaneo)
	defer cancel()

	lector, err := s.almacen.Abrir(ctx, s.buckets.Originales, asset.OriginalKey)
	if err != nil {
		return err
	}
	defer lector.Close()

	veredicto, err := s.antivirus.Escanear(ctx, lector)
	if err != nil {
		if errors.Is(err, ErrLimiteDelEscaner) {
			return s.cuarentena(ctx, asset, EstadoRechazado,
				"supera el tamaño máximo que el antivirus puede analizar",
				"media.scan_skipped")
		}
		// Cualquier otro fallo es del sistema: se reintenta.
		return fmt.Errorf("escanear el asset %s: %w", id, err)
	}

	if !veredicto.Limpio {
		s.log.Warn("asset infectado", "asset_id", id, "firma", veredicto.Firma)
		return s.cuarentena(ctx, asset, EstadoInfectado, veredicto.Firma, "media.infected")
	}

	ahora := time.Now().UTC()
	limpio := "clean"
	asset.Status = EstadoLimpio
	asset.ScanResult = &limpio
	asset.ScannedAt = &ahora
	asset.UpdatedAt = ahora
	s.log.Info("asset limpio", "asset_id", id)

	// Lo limpio sigue a la transcodificación, con el mismo outbox de siempre.
	var claveHLS string
	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.ActualizarAsset(ctx, tx, asset); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, tx, audit.Event{
			ActorID: &asset.OwnerID, ActorRole: "teacher", Action: "media.scanned",
			EntityType: "asset", EntityID: &asset.ID,
			Metadata: map[string]any{"resultado": "clean"},
		}); err != nil {
			return err
		}
		if !HLSAplicable(asset.Kind) {
			return nil
		}
		claveHLS, err = s.EncolarTranscodificacion(ctx, tx, asset.ID)
		return err
	})
	if err != nil || claveHLS == "" {
		return err
	}

	if err := s.queue.Publish(ctx, claveHLS, jobs.TypeMediaTranscode, jobs.QueueBulk,
		map[string]any{"asset_id": asset.ID.String()}, ""); err != nil {
		s.log.Warn("no se pudo publicar la transcodificación; el reaper la recuperará",
			"job_key", claveHLS, "error", err)
	}
	return nil
}

// cuarentena mueve el original fuera del bucket bueno y deja constancia. El
// objeto no se borra: si el veredicto fuera un falso positivo, hay algo que
// revisar.
func (s *Service) cuarentena(ctx context.Context, asset Asset, estado, resultado, accion string) error {
	if err := s.almacen.Mover(ctx, s.buckets.Originales, asset.OriginalKey,
		s.buckets.Cuarentena, asset.OriginalKey); err != nil {
		s.log.Warn("no se pudo mover a cuarentena", "asset_id", asset.ID, "error", err)
	}

	ahora := time.Now().UTC()
	motivo := resultado
	asset.Status = estado
	asset.ScanResult = &resultado
	asset.ScannedAt = &ahora
	asset.LastError = &motivo
	asset.UpdatedAt = ahora

	return s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.ActualizarAsset(ctx, tx, asset); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &asset.OwnerID, ActorRole: "teacher", Action: accion,
			EntityType: "asset", EntityID: &asset.ID,
			Metadata: map[string]any{"resultado": resultado, "estado": estado},
		})
	})
}

// EncolarEscaneo registra el trabajo dentro de la transacción que deja el asset
// en `scanning`. Se publica después del commit (outbox, ADR-0004).
func (s *Service) EncolarEscaneo(ctx context.Context, tx dbx.DB, assetID uuid.UUID) (string, error) {
	clave := fmt.Sprintf("%s:%s", jobs.TypeMediaScan, assetID)
	return clave, jobs.Record(ctx, tx, jobs.Job{
		Key: clave, Type: jobs.TypeMediaScan, Queue: jobs.QueueBulk,
		Payload: map[string]any{"asset_id": assetID.String()},
	})
}
