package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"

	"mooc/backend/internal/modules/audit"
	"mooc/backend/internal/platform/dbx"
	"mooc/backend/internal/platform/ids"
	"mooc/backend/internal/platform/jobs"
)

// Almacen es el puerto hacia el almacenamiento de objetos.
type Almacen interface {
	CrearMultipart(ctx context.Context, bucket, key, contentType string) (string, error)
	PresignPart(ctx context.Context, bucket, key, uploadID string, parte int, ttl time.Duration) (string, error)
	PartesSubidas(ctx context.Context, bucket, key, uploadID string) ([]Parte, error)
	CompletarMultipart(ctx context.Context, bucket, key, uploadID string, partes []Parte) error
	AbortarMultipart(ctx context.Context, bucket, key, uploadID string) error
	PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error)
	Abrir(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Info(ctx context.Context, bucket, key string) (int64, error)
	Mover(ctx context.Context, ob, ok, db, dk string) error
	Subir(ctx context.Context, bucket, key, contentType string, datos []byte) error
}

// Parte se redeclara aquí para que el dominio no importe el adaptador.
type Parte struct {
	Numero int
	ETag   string
	Bytes  int64
}

type Store interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx dbx.DB) error) error
	DB() dbx.DB

	InsertarAsset(ctx context.Context, db dbx.DB, a Asset) error
	AssetPorID(ctx context.Context, db dbx.DB, id uuid.UUID) (Asset, error)
	ActualizarAsset(ctx context.Context, db dbx.DB, a Asset) error

	InsertarCarga(ctx context.Context, db dbx.DB, c Carga) error
	CargaDeAsset(ctx context.Context, db dbx.DB, assetID uuid.UUID) (Carga, error)
	CerrarCarga(ctx context.Context, db dbx.DB, assetID uuid.UUID, abortada bool) error

	// Los derivados se reemplazan en bloque, no se acumulan: una reejecución
	// del trabajo deja el mismo juego, no uno más (CA-03).
	ReemplazarDerivados(ctx context.Context, db dbx.DB, assetID uuid.UUID, ds []Derivado) error
	DerivadosDeAsset(ctx context.Context, db dbx.DB, assetID uuid.UUID) ([]Derivado, error)
}

type Publicador interface {
	Publish(ctx context.Context, key, tipo, cola string, payload map[string]any, traceID string) error
}

type Buckets struct {
	Originales string
	Derivados  string
	Cuarentena string
}

type Service struct {
	store   Store
	almacen Almacen
	queue   Publicador
	audit   *audit.Recorder
	buckets Buckets
	// ffmpeg solo lo tiene el worker de medios: la API no transcodifica.
	ffmpeg Transcodificador
	// antivirus solo lo tiene el worker: la API no escanea.
	antivirus Antivirus
	// repro solo lo tiene la API: el worker no entrega manifiestos.
	repro Reproduccion
	// hostPermitido es el único destino al que se puede redirigir.
	hostPermitido string
	log           *slog.Logger
}

func NewService(s Store, a Almacen, q Publicador, rec *audit.Recorder, b Buckets,
	hostAlmacen string, log *slog.Logger) *Service {
	return &Service{store: s, almacen: a, queue: q, audit: rec, buckets: b,
		hostPermitido: hostAlmacen, log: log}
}

// ConTranscodificador se lo enchufa el worker de medios. La API construye el
// mismo servicio sin él: no tiene FFmpeg en su imagen ni lo necesita.
func (s *Service) ConTranscodificador(t Transcodificador) *Service {
	s.ffmpeg = t
	return s
}

type Actor struct {
	UserID uuid.UUID
	Role   string
	IP     string
	Agent  string
	Trace  string
}

// Ticket es lo que se devuelve al iniciar una carga.
type Ticket struct {
	Asset      Asset
	UploadID   string
	PartSize   int64
	TotalParts int
	ExpiresAt  time.Time
	Partes     []ParteURL
}

type ParteURL struct {
	Numero    int
	URL       string
	ExpiresAt time.Time
}

// Iniciar abre la carga multipart y firma las URLs de las partes. Los bytes no
// pasan por la API: el cliente sube directamente al almacén (enunciado §4).
func (s *Service) Iniciar(ctx context.Context, in NuevaCarga, a Actor) (Ticket, error) {
	if errs := in.Validate(); !errs.Empty() {
		return Ticket{}, errs
	}

	now := time.Now().UTC()
	asset := Asset{
		ID: ids.New(), OwnerID: a.UserID, Kind: in.Kind,
		OriginalFilename: in.Filename, DeclaredMIME: in.ContentType,
		SizeBytes: in.SizeBytes, DeclaredSHA256: in.SHA256,
		Status: EstadoSubiendo, CreatedAt: now, UpdatedAt: now,
	}
	asset.OriginalKey = ClaveOriginal(asset.ID, in.Filename)

	uploadID, err := s.almacen.CrearMultipart(ctx, s.buckets.Originales, asset.OriginalKey, in.ContentType)
	if err != nil {
		return Ticket{}, err
	}

	total := PartesNecesarias(in.SizeBytes, TamañoDeParte)
	carga := Carga{
		ID: ids.New(), AssetID: asset.ID, S3UploadID: uploadID,
		PartSize: TamañoDeParte, TotalParts: total,
		ExpiresAt: now.Add(TTLCarga),
	}

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.InsertarAsset(ctx, tx, asset); err != nil {
			return err
		}
		if err := s.store.InsertarCarga(ctx, tx, carga); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: &a.UserID, ActorRole: a.Role, Action: "media.upload_started",
			EntityType: "asset", EntityID: &asset.ID,
			IP: a.IP, UserAgent: a.Agent, TraceID: a.Trace,
			Metadata: map[string]any{"filename": in.Filename, "size_bytes": in.SizeBytes},
		})
	})
	if err != nil {
		_ = s.almacen.AbortarMultipart(ctx, s.buckets.Originales, asset.OriginalKey, uploadID)
		return Ticket{}, err
	}

	partes, err := s.firmarPartes(ctx, asset, carga, todasLasPartes(total))
	if err != nil {
		return Ticket{}, err
	}
	return Ticket{Asset: asset, UploadID: uploadID, PartSize: carga.PartSize,
		TotalParts: total, ExpiresAt: carga.ExpiresAt, Partes: partes}, nil
}

func todasLasPartes(n int) []int {
	out := make([]int, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, i)
	}
	return out
}

func (s *Service) firmarPartes(ctx context.Context, a Asset, c Carga, numeros []int) ([]ParteURL, error) {
	// Las URL se firman a 24 h, que es lo que el enunciado exige que dure una
	// carga reanudable (RF-05, RNF-07).
	ttl := time.Until(c.ExpiresAt)
	if ttl <= 0 {
		return nil, ErrCargaVencida
	}
	out := make([]ParteURL, 0, len(numeros))
	for _, n := range numeros {
		u, err := s.almacen.PresignPart(ctx, s.buckets.Originales, a.OriginalKey, c.S3UploadID, n, ttl)
		if err != nil {
			return nil, err
		}
		out = append(out, ParteURL{Numero: n, URL: u, ExpiresAt: c.ExpiresAt})
	}
	return out, nil
}

func (s *Service) guard(ctx context.Context, assetID uuid.UUID, a Actor) (Asset, error) {
	asset, err := s.store.AssetPorID(ctx, s.store.DB(), assetID)
	if err != nil {
		return Asset{}, err
	}
	if asset.OwnerID != a.UserID && a.Role != "admin" {
		return Asset{}, ErrProhibido
	}
	return asset, nil
}

// EstadoDeCarga dice qué partes tiene ya el almacén. Es lo que permite
// reanudar: el cliente sube solo lo que falta (RF-05, SEG-3).
func (s *Service) EstadoDeCarga(ctx context.Context, assetID uuid.UUID, a Actor) (Asset, Carga, []Parte, error) {
	asset, err := s.guard(ctx, assetID, a)
	if err != nil {
		return Asset{}, Carga{}, nil, err
	}
	carga, err := s.store.CargaDeAsset(ctx, s.store.DB(), assetID)
	if err != nil {
		return Asset{}, Carga{}, nil, err
	}
	partes, err := s.almacen.PartesSubidas(ctx, s.buckets.Originales, asset.OriginalKey, carga.S3UploadID)
	if err != nil {
		return Asset{}, Carga{}, nil, err
	}
	return asset, carga, partes, nil
}

// RenovarPartes vuelve a firmar las URLs de las partes que falten.
func (s *Service) RenovarPartes(ctx context.Context, assetID uuid.UUID, numeros []int, a Actor) ([]ParteURL, error) {
	asset, carga, _, err := s.EstadoDeCarga(ctx, assetID, a)
	if err != nil {
		return nil, err
	}
	if !carga.Abierta() {
		return nil, ErrEstado
	}
	return s.firmarPartes(ctx, asset, carga, numeros)
}

// Completar cierra la multipart y encola la verificación. Responde sin esperar
// al worker (CA-02).
func (s *Service) Completar(ctx context.Context, assetID uuid.UUID, partes []Parte, a Actor) (Asset, error) {
	asset, err := s.guard(ctx, assetID, a)
	if err != nil {
		return Asset{}, err
	}
	carga, err := s.store.CargaDeAsset(ctx, s.store.DB(), assetID)
	if err != nil {
		return Asset{}, err
	}
	if !carga.Abierta() {
		return Asset{}, ErrEstado
	}
	if carga.Vencida(time.Now().UTC()) {
		return Asset{}, ErrCargaVencida
	}

	if err := s.almacen.CompletarMultipart(ctx, s.buckets.Originales,
		asset.OriginalKey, carga.S3UploadID, partes); err != nil {
		return Asset{}, err
	}

	asset.Status = EstadoSubido
	asset.UpdatedAt = time.Now().UTC()

	clave := fmt.Sprintf("%s:%s", jobs.TypeMediaProbe, asset.ID)
	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.ActualizarAsset(ctx, tx, asset); err != nil {
			return err
		}
		if err := s.store.CerrarCarga(ctx, tx, asset.ID, false); err != nil {
			return err
		}
		return jobs.Record(ctx, tx, jobs.Job{
			Key: clave, Type: jobs.TypeMediaProbe, Queue: jobs.QueueDefault,
			Payload: map[string]any{"asset_id": asset.ID.String()},
		})
	})
	if err != nil {
		return Asset{}, err
	}

	if err := s.queue.Publish(ctx, clave, jobs.TypeMediaProbe, jobs.QueueDefault,
		map[string]any{"asset_id": asset.ID.String()}, a.Trace); err != nil {
		s.log.Warn("no se pudo publicar la verificación; el reaper la recuperará",
			"job_key", clave, "error", err)
	}
	return asset, nil
}

// Abortar descarta una carga a medias.
func (s *Service) Abortar(ctx context.Context, assetID uuid.UUID, a Actor) error {
	asset, err := s.guard(ctx, assetID, a)
	if err != nil {
		return err
	}
	carga, err := s.store.CargaDeAsset(ctx, s.store.DB(), assetID)
	if err != nil {
		return err
	}
	if carga.Abierta() {
		if err := s.almacen.AbortarMultipart(ctx, s.buckets.Originales,
			asset.OriginalKey, carga.S3UploadID); err != nil {
			s.log.Warn("no se pudo abortar en el almacén", "asset_id", assetID, "error", err)
		}
	}
	asset.Status = EstadoRechazado
	asset.UpdatedAt = time.Now().UTC()
	return s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.ActualizarAsset(ctx, tx, asset); err != nil {
			return err
		}
		return s.store.CerrarCarga(ctx, tx, assetID, true)
	})
}

func (s *Service) Asset(ctx context.Context, id uuid.UUID, a Actor) (Asset, error) {
	return s.guard(ctx, id, a)
}

// URLDeDescarga emite una URL firmada de lectura, después de comprobar el
// derecho de acceso (CA-06).
//
// El destino se comprueba antes de devolverlo. Hoy la URL la construye nuestro
// propio adaptador, así que no puede apuntar a otro sitio; la comprobación
// existe para que siga siendo verdad si mañana el endpoint sale de una
// configuración equivocada o de un adaptador nuevo. Redirigir a una URL
// calculada sin mirar a dónde va es como se abren los *open redirect*.
func (s *Service) URLDeDescarga(ctx context.Context, id uuid.UUID, a Actor) (string, error) {
	asset, err := s.guard(ctx, id, a)
	if err != nil {
		return "", err
	}
	if asset.Status != EstadoListo && asset.Status != EstadoLimpio {
		return "", ErrEstado
	}
	u, err := s.almacen.PresignGet(ctx, s.buckets.Originales, asset.OriginalKey, TTLDescarga)
	if err != nil {
		return "", err
	}
	if !s.destinoPermitido(u) {
		return "", fmt.Errorf("la URL firmada apunta fuera del almacén configurado")
	}
	return u, nil
}

// HostDelAlmacen expone el único destino al que la API puede redirigir, para
// que la capa HTTP lo compruebe a la vista de quien lea el redirect.
func (s *Service) HostDelAlmacen() string { return s.hostPermitido }

// destinoPermitido comprueba que la URL vaya al almacén de objetos y a ningún
// otro sitio.
func (s *Service) destinoPermitido(crudo string) bool {
	u, err := url.Parse(crudo)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return u.Host == s.hostPermitido
}

// ---------------------------------------------------------------- worker ---

// Verificar es el trabajo media.probe. Recalcula el checksum leyendo del
// almacén y detecta el MIME real por los bytes: la palabra del cliente no
// basta para nada de esto (RF-05).
//
// Idempotente: leer y comparar no cambia nada, y el reclamo atómico del runner
// impide dos ejecuciones simultáneas (ADR-0008).
func (s *Service) Verificar(ctx context.Context, data map[string]any) error {
	raw, _ := data["asset_id"].(string)
	id, err := uuid.Parse(raw)
	if err != nil {
		return fmt.Errorf("media.probe con asset_id inválido: %q", raw)
	}

	asset, err := s.store.AssetPorID(ctx, s.store.DB(), id)
	if err != nil {
		return err
	}
	if asset.Status == EstadoEscaneando || asset.Status == EstadoLimpio ||
		asset.Status == EstadoProcesando || asset.Status == EstadoListo ||
		asset.Status == EstadoRechazado || asset.Status == EstadoInfectado {
		s.log.Info("el asset ya estaba verificado", "asset_id", id, "status", asset.Status)
		return nil
	}

	tamaño, err := s.almacen.Info(ctx, s.buckets.Originales, asset.OriginalKey)
	if err != nil {
		return err
	}

	lector, err := s.almacen.Abrir(ctx, s.buckets.Originales, asset.OriginalKey)
	if err != nil {
		return err
	}
	defer lector.Close()

	// Un solo recorrido: se calcula el hash y se guardan los primeros bytes
	// para identificar el tipo. Leer el archivo dos veces no aporta nada.
	h := sha256.New()
	cabecera := make([]byte, 0, 512)
	buf := make([]byte, 32*1024)
	for {
		n, err := lector.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			if len(cabecera) < 512 {
				falta := 512 - len(cabecera)
				if falta > n {
					falta = n
				}
				cabecera = append(cabecera, buf[:falta]...)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("leer el objeto: %w", err)
		}
	}

	suma := hex.EncodeToString(h.Sum(nil))
	mime := DetectarMIME(cabecera, "")

	motivo := ""
	switch {
	case tamaño != asset.SizeBytes:
		motivo = fmt.Sprintf("el tamaño real (%d) no coincide con el declarado (%d)", tamaño, asset.SizeBytes)
	case suma != asset.DeclaredSHA256:
		motivo = "el checksum real no coincide con el declarado"
	case !MIMEAceptable(asset.Kind, mime):
		motivo = fmt.Sprintf("el tipo real (%s) no se admite para un recurso de tipo %s", mime, asset.Kind)
	}

	asset.SHA256 = &suma
	asset.DetectedMIME = &mime
	asset.UpdatedAt = time.Now().UTC()

	if motivo != "" {
		asset.Status = EstadoRechazado
		asset.LastError = &motivo
		// Lo rechazado va a cuarentena: no se queda junto a los originales
		// buenos ni se borra, por si hay que revisarlo.
		if err := s.almacen.Mover(ctx, s.buckets.Originales, asset.OriginalKey,
			s.buckets.Cuarentena, asset.OriginalKey); err != nil {
			s.log.Warn("no se pudo mover a cuarentena", "asset_id", id, "error", err)
		}
		s.log.Warn("asset rechazado", "asset_id", id, "motivo", motivo)
	} else {
		// Verificado no es lo mismo que limpio: pasa a `scanning` y es el
		// antivirus quien decide si sigue adelante (ADR-0011).
		asset.Status = EstadoEscaneando
		s.log.Info("asset verificado", "asset_id", id, "mime", mime, "bytes", tamaño)
	}

	// Lo que se verificó bien pasa al antivirus. El trabajo se registra dentro
	// de la misma transacción que cambia el estado y se publica después del
	// commit: si el proceso muere en medio, el reaper lo recupera (ADR-0004).
	var claveEscaneo string
	err = s.store.WithinTx(ctx, func(ctx context.Context, tx dbx.DB) error {
		if err := s.store.ActualizarAsset(ctx, tx, asset); err != nil {
			return err
		}
		accion := "media.verified"
		if motivo != "" {
			accion = "media.rejected"
		}
		if err := s.audit.Record(ctx, tx, audit.Event{
			ActorID: &asset.OwnerID, ActorRole: "teacher", Action: accion,
			EntityType: "asset", EntityID: &asset.ID,
			Metadata: map[string]any{"mime": mime, "bytes": tamaño, "motivo": motivo},
		}); err != nil {
			return err
		}
		if motivo != "" {
			return nil
		}
		claveEscaneo, err = s.EncolarEscaneo(ctx, tx, asset.ID)
		return err
	})
	if err != nil || claveEscaneo == "" {
		return err
	}

	if err := s.queue.Publish(ctx, claveEscaneo, jobs.TypeMediaScan, jobs.QueueBulk,
		map[string]any{"asset_id": asset.ID.String()}, ""); err != nil {
		s.log.Warn("no se pudo publicar el escaneo; el reaper lo recuperará",
			"job_key", claveEscaneo, "error", err)
	}
	return nil
}
