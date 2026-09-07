// Package media cubre la carga de binarios y su verificación (RF-05).
//
// Alcance actual: paso 1 del plan en arquitectura/media-plan.md — carga
// multipart directa con reanudación, verificación de integridad y detección
// del MIME real. El escaneo antimalware y la transcodificación a HLS son los
// pasos 2 y 3, y todavía no están.
package media

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("media: no encontrado")
	ErrProhibido    = errors.New("media: sin permiso sobre este archivo")
	ErrEstado       = errors.New("media: la operación no es válida en este estado")
	ErrCargaVencida = errors.New("media: la carga expiró")
)

// Estados del asset. Ver disenos/maquinas-de-estado.md.
const (
	EstadoSubiendo   = "uploading"
	EstadoSubido     = "uploaded"
	EstadoEscaneando = "scanning"
	EstadoLimpio     = "clean"
	EstadoProcesando = "processing"
	EstadoListo      = "ready"
	EstadoRechazado  = "rejected"
	EstadoInfectado  = "infected"
	EstadoFallido    = "failed"
)

const (
	TipoVideo   = "video"
	TipoAudio   = "audio"
	TipoImagen  = "image"
	TipoPDF     = "pdf"
	TipoSlides  = "slides"
	TipoArchivo = "file"
)

const (
	// TamañoDeParte es el de la multipart de S3. Ocho MiB es cómodo para
	// reanudar sin generar miles de partes en un archivo grande.
	TamañoDeParte int64 = 8 << 20
	// TamañoMaximo por archivo.
	TamañoMaximo int64 = 5 << 30
	// TTLCarga es lo que el enunciado exige que dure una carga reanudable.
	TTLCarga = 24 * time.Hour
	// TTLDescarga es la vida de una URL firmada de lectura.
	TTLDescarga = 15 * time.Minute
)

type Asset struct {
	ID               uuid.UUID
	OwnerID          uuid.UUID
	Kind             string
	OriginalKey      string
	OriginalFilename string
	DeclaredMIME     string
	DetectedMIME     *string
	SizeBytes        int64
	DeclaredSHA256   string
	SHA256           *string
	DurationSeconds  *float64
	Width, Height    *int
	Status           string
	LastError        *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Carga struct {
	ID          uuid.UUID
	AssetID     uuid.UUID
	S3UploadID  string
	PartSize    int64
	TotalParts  int
	ExpiresAt   time.Time
	CompletedAt *time.Time
	AbortedAt   *time.Time
}

func (c Carga) Vencida(ahora time.Time) bool { return ahora.After(c.ExpiresAt) }
func (c Carga) Abierta() bool                { return c.CompletedAt == nil && c.AbortedAt == nil }

// ClaveOriginal es determinista: reintentar escribe en el mismo sitio, así que
// la operación es idempotente por construcción (ADR-0005).
func ClaveOriginal(assetID uuid.UUID, nombre string) string {
	ext := ""
	if i := strings.LastIndex(nombre, "."); i >= 0 && len(nombre)-i <= 6 {
		ext = strings.ToLower(nombre[i:])
	}
	return fmt.Sprintf("originals/%s/original%s", assetID, ext)
}

// PartesNecesarias calcula cuántas partes hacen falta.
func PartesNecesarias(tamaño, tamañoDeParte int64) int {
	if tamaño <= 0 {
		return 1
	}
	n := tamaño / tamañoDeParte
	if tamaño%tamañoDeParte != 0 {
		n++
	}
	return int(n)
}

// ---------------------------------------------------------------- MIME -----

// mimesPermitidos es la lista blanca por tipo de recurso. Se compara contra el
// MIME REAL detectado por el servidor, nunca contra la extensión ni contra lo
// que declare el cliente.
var mimesPermitidos = map[string][]string{
	TipoVideo:  {"video/mp4", "video/quicktime", "video/x-matroska", "video/webm"},
	TipoAudio:  {"audio/mpeg", "audio/mp4", "audio/ogg", "audio/wav", "audio/x-wav"},
	TipoImagen: {"image/jpeg", "image/png", "image/gif", "image/webp"},
	TipoPDF:    {"application/pdf"},
	TipoSlides: {
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.oasis.opendocument.presentation",
	},
	TipoArchivo: nil, // cualquiera, salvo los peligrosos de abajo
}

// mimesProhibidos no se admiten en ningún tipo: son ejecutables.
var mimesProhibidos = []string{
	"application/x-msdownload", "application/x-executable",
	"application/x-sharedlib", "application/x-mach-binary",
	"text/x-shellscript", "application/x-msi",
}

// MIMEAceptable decide si el MIME real encaja con el tipo declarado.
func MIMEAceptable(kind, mimeReal string) bool {
	mimeReal = strings.ToLower(strings.TrimSpace(strings.Split(mimeReal, ";")[0]))
	for _, p := range mimesProhibidos {
		if mimeReal == p {
			return false
		}
	}
	permitidos, conocido := mimesPermitidos[kind]
	if !conocido {
		return false
	}
	if permitidos == nil {
		return true
	}
	for _, m := range permitidos {
		if mimeReal == m {
			return true
		}
	}
	return false
}

// DetectarMIME identifica el tipo por los *magic bytes* del principio del
// archivo. `http.DetectContentType` no reconoce MP4 ni Matroska, así que se
// añaden a mano las firmas que el proyecto necesita.
func DetectarMIME(cabecera []byte, porDefecto string) string {
	if len(cabecera) >= 12 {
		// ISO-BMFF: los bytes 4..8 son "ftyp".
		if string(cabecera[4:8]) == "ftyp" {
			marca := string(cabecera[8:12])
			switch {
			case strings.HasPrefix(marca, "qt"):
				return "video/quicktime"
			case strings.HasPrefix(marca, "M4A"):
				return "audio/mp4"
			default:
				return "video/mp4"
			}
		}
	}
	if len(cabecera) >= 4 && cabecera[0] == 0x1A && cabecera[1] == 0x45 &&
		cabecera[2] == 0xDF && cabecera[3] == 0xA3 {
		return "video/x-matroska"
	}
	if len(cabecera) >= 4 && string(cabecera[:4]) == "%PDF" {
		return "application/pdf"
	}
	if porDefecto != "" {
		return porDefecto
	}
	return "application/octet-stream"
}

// ---------------------------------------------------------- validación -----

type ValidationError struct {
	Code   string
	Field  string
	Detail string
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	p := make([]string, 0, len(v))
	for _, e := range v {
		p = append(p, e.Field+": "+e.Detail)
	}
	return strings.Join(p, "; ")
}

func (v ValidationErrors) Empty() bool { return len(v) == 0 }

// NuevaCarga son los datos con los que el cliente pide iniciar una carga.
type NuevaCarga struct {
	Filename    string
	ContentType string
	SizeBytes   int64
	SHA256      string
	Kind        string
}

func (in NuevaCarga) Validate() ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(in.Filename) == "" {
		errs = append(errs, ValidationError{"filename.required", "filename", "El nombre del archivo es obligatorio."})
	}
	if in.SizeBytes <= 0 {
		errs = append(errs, ValidationError{"size.invalid", "size_bytes", "El tamaño debe ser mayor que cero."})
	} else if in.SizeBytes > TamañoMaximo {
		errs = append(errs, ValidationError{"size.too_large", "size_bytes",
			fmt.Sprintf("El archivo supera el máximo de %d GiB.", TamañoMaximo>>30)})
	}
	if len(in.SHA256) != 64 || strings.TrimLeft(strings.ToLower(in.SHA256), "0123456789abcdef") != "" {
		errs = append(errs, ValidationError{"sha256.invalid", "sha256",
			"El checksum debe ser un SHA-256 en hexadecimal de 64 caracteres."})
	}
	if _, ok := mimesPermitidos[in.Kind]; !ok {
		errs = append(errs, ValidationError{"kind.invalid", "kind", "Tipo de archivo desconocido."})
	}

	return errs
}
