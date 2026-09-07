// Package authoring cubre cursos, versiones, jerarquía y publicación
// (RF-03, RF-04).
//
// Dominio puro: entidades y reglas. No importa net/http ni pgx (ADR-0001).
package authoring

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

const (
	StatusDraft       = "draft"
	StatusPublished   = "published"
	StatusUnpublished = "unpublished"
	StatusArchived    = "archived"
)

// Tipos de recurso admitidos (enunciado §3).
const (
	TypeRichText = "rich_text"
	TypeImage    = "image"
	TypeVideo    = "video"
	TypeAudio    = "audio"
	TypePDF      = "pdf"
	TypeSlides   = "slides"
	TypeDownload = "download"
	TypeIframe   = "iframe"
	TypeLink     = "link"
	TypeQuiz     = "quiz"
)

var tiposValidos = map[string]bool{
	TypeRichText: true, TypeImage: true, TypeVideo: true, TypeAudio: true,
	TypePDF: true, TypeSlides: true, TypeDownload: true, TypeIframe: true,
	TypeLink: true, TypeQuiz: true,
}

// requiereAsset indica qué tipos necesitan un binario listo para publicarse.
var requiereAsset = map[string]bool{
	TypeImage: true, TypeVideo: true, TypeAudio: true,
	TypePDF: true, TypeSlides: true, TypeDownload: true,
}

// IframesPermitidos es la lista blanca de dominios (RO-02). Se valida al
// guardar y otra vez al publicar.
var IframesPermitidos = []string{
	"www.youtube.com", "youtube.com", "player.vimeo.com",
	"docs.google.com", "www.google.com",
}

type Course struct {
	ID               uuid.UUID
	Slug             string
	Title            string
	Summary          string
	Category         string
	Language         string
	OwnerID          uuid.UUID
	Status           string
	CurrentVersionID *uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ApprovalCriteria struct {
	RequiredCompletionPct float64 `json:"required_completion_pct"`
	MinQuizScore          float64 `json:"min_quiz_score"`
}

type CourseVersion struct {
	ID               uuid.UUID
	CourseID         uuid.UUID
	VersionNumber    int
	Status           string
	ApprovalCriteria ApprovalCriteria
	PublishedAt      *time.Time
	CreatedAt        time.Time
	Modules          []Module
}

// Editable dice si el árbol de la versión admite cambios. Una versión
// publicada es inmutable: hay que despublicarla primero (ADR-0007, §5.1).
func (v CourseVersion) Editable() bool {
	return v.Status == StatusDraft || v.Status == StatusUnpublished
}

type Module struct {
	ID              uuid.UUID
	CourseVersionID uuid.UUID
	StableID        uuid.UUID
	Position        int
	Title           string
	Units           []Unit
}

type Unit struct {
	ID              uuid.UUID
	ModuleID        uuid.UUID
	CourseVersionID uuid.UUID
	StableID        uuid.UUID
	Position        int
	Title           string
	Resources       []Resource
}

type Resource struct {
	ID              uuid.UUID
	UnitID          uuid.UUID
	CourseVersionID uuid.UUID
	StableID        uuid.UUID
	Position        int
	Title           string
	Type            string
	Visible         bool
	Required        bool
	Downloadable    bool
	ContentMD       *string
	AssetRefs       []uuid.UUID
	AssetID         *uuid.UUID
	ExternalURL     *string
	UpdatedAt       time.Time

	// AssetStatus lo rellena la consulta al unir con media.assets; se usa solo
	// para validar la publicación.
	AssetStatus string
	// HasQuestions lo rellena la consulta; un quiz sin preguntas no publica.
	HasQuestions bool
	// QuizAllOptionsOK indica si toda pregunta tiene al menos una correcta.
	QuizAllOptionsOK bool
}

// ContenidoETag identifica la versión del contenido por su valor, no por
// cuándo se guardó. Guardar dos veces lo mismo da el mismo ETag (ADR-0014).
func (r Resource) ContenidoETag() string {
	c := ""
	if r.ContentMD != nil {
		c = *r.ContentMD
	}
	return etagDeContenido(r.StableID.String(), c)
}

// ProcessingStatus traduce el estado del binario a lo que ve el cliente.
func (r Resource) ProcessingStatus() string {
	if !requiereAsset[r.Type] {
		return "n/a"
	}
	switch r.AssetStatus {
	case "ready":
		return "ready"
	case "processing", "scanning", "clean":
		return "processing"
	case "rejected", "infected", "failed":
		return "failed"
	case "":
		return "pending"
	default:
		return "pending"
	}
}

// etagDeContenido lo inyecta la capa HTTP para no meter crypto en el dominio.
var etagDeContenido = func(partes ...string) string { return "" }

// UsarCalculoDeETag lo llama la composición al arrancar.
func UsarCalculoDeETag(f func(...string) string) { etagDeContenido = f }

// ---------------------------------------------------------------- errores --

type ValidationError struct {
	Code   string
	Field  string
	Detail string
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	partes := make([]string, 0, len(v))
	for _, e := range v {
		partes = append(partes, e.Field+": "+e.Detail)
	}
	return strings.Join(partes, "; ")
}

func (v ValidationErrors) Empty() bool { return len(v) == 0 }

// ------------------------------------------------------------- validación --

type CourseInput struct {
	Title    string
	Summary  string
	Category string
	Language string
}

func (in CourseInput) Validate() ValidationErrors {
	var errs ValidationErrors
	if strings.TrimSpace(in.Title) == "" {
		errs = append(errs, ValidationError{"title.required", "title", "El título es obligatorio."})
	} else if len([]rune(in.Title)) > 200 {
		errs = append(errs, ValidationError{"title.too_long", "title", "El título no puede superar 200 caracteres."})
	}
	if len([]rune(in.Summary)) > 2000 {
		errs = append(errs, ValidationError{"summary.too_long", "summary", "El resumen no puede superar 2000 caracteres."})
	}
	return errs
}

type ResourceInput struct {
	Title        string
	Type         string
	Visible      *bool
	Required     *bool
	Downloadable *bool
	AssetID      *uuid.UUID
	ExternalURL  *string
	ContentMD    *string
}

func (in ResourceInput) Validate() ValidationErrors {
	var errs ValidationErrors
	if strings.TrimSpace(in.Title) == "" {
		errs = append(errs, ValidationError{"title.required", "title", "El título es obligatorio."})
	}
	if in.Type != "" && !tiposValidos[in.Type] {
		errs = append(errs, ValidationError{"type.invalid", "type",
			fmt.Sprintf("Tipo de recurso desconocido: %q.", in.Type)})
	}
	if in.Type == TypeIframe && in.ExternalURL != nil && !IframePermitido(*in.ExternalURL) {
		errs = append(errs, ValidationError{"resource.iframe_not_allowed", "external_url",
			"El dominio del iframe no está en la lista blanca."})
	}
	return errs
}

// IframePermitido comprueba el dominio contra la lista blanca. Se aplica al
// guardar y otra vez al publicar: la segunda vez cubre los recursos creados
// antes de que la lista cambiara.
func IframePermitido(raw string) bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if !strings.HasPrefix(raw, "https://") {
		return false
	}
	resto := strings.TrimPrefix(raw, "https://")
	host := resto
	if i := strings.IndexAny(resto, "/?#"); i >= 0 {
		host = resto[:i]
	}
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	for _, permitido := range IframesPermitidos {
		if host == permitido {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------ publicación --

// ValidarPublicacion recorre TODAS las reglas y devuelve la lista completa de
// incumplimientos. No para en el primero: es lo que exige CA-01 y lo que el
// segmento 2 de la demostración tiene que evidenciar.
func ValidarPublicacion(c Course, v CourseVersion) ValidationErrors {
	var errs ValidationErrors

	// Metadatos completos.
	var faltantes []string
	if strings.TrimSpace(c.Title) == "" {
		faltantes = append(faltantes, "title")
	}
	if strings.TrimSpace(c.Summary) == "" {
		faltantes = append(faltantes, "summary")
	}
	if strings.TrimSpace(c.Category) == "" {
		faltantes = append(faltantes, "category")
	}
	if strings.TrimSpace(c.Language) == "" {
		faltantes = append(faltantes, "language")
	}
	if len(faltantes) > 0 {
		errs = append(errs, ValidationError{"metadata.missing_fields", "course",
			"Faltan metadatos obligatorios: " + strings.Join(faltantes, ", ") + "."})
	}

	// Criterios de aprobación.
	ac := v.ApprovalCriteria
	if ac.RequiredCompletionPct <= 0 || ac.RequiredCompletionPct > 100 {
		errs = append(errs, ValidationError{"approval.criteria_missing", "approval_criteria",
			"El porcentaje de obligatorios debe estar entre 1 y 100."})
	}
	if ac.MinQuizScore < 0 || ac.MinQuizScore > 100 {
		errs = append(errs, ValidationError{"approval.criteria_missing", "approval_criteria",
			"La nota mínima debe estar entre 0 y 100."})
	}

	// Estructura mínima: curso → módulo → unidad → recurso visible.
	visiblesTotales := 0
	for mi, m := range v.Modules {
		for ui, u := range m.Units {
			for ri, r := range u.Resources {
				if !r.Visible {
					continue
				}
				visiblesTotales++
				campo := fmt.Sprintf("modules[%d].units[%d].resources[%d]", mi, ui, ri)

				if requiereAsset[r.Type] {
					if r.AssetID == nil {
						errs = append(errs, ValidationError{"resource.not_available", campo,
							fmt.Sprintf("«%s» es de tipo %s y no tiene archivo asociado.", r.Title, r.Type)})
					} else if r.ProcessingStatus() != "ready" {
						errs = append(errs, ValidationError{"resource.not_available", campo,
							fmt.Sprintf("«%s» aún no está disponible (estado %s).", r.Title, r.ProcessingStatus())})
					}
				}
				if r.Type == TypeQuiz {
					if !r.HasQuestions {
						errs = append(errs, ValidationError{"quiz.invalid", campo,
							fmt.Sprintf("El quiz «%s» no tiene preguntas.", r.Title)})
					} else if !r.QuizAllOptionsOK {
						errs = append(errs, ValidationError{"quiz.invalid", campo,
							fmt.Sprintf("Alguna pregunta del quiz «%s» no tiene opción correcta.", r.Title)})
					}
				}
				if r.Type == TypeIframe {
					if r.ExternalURL == nil || !IframePermitido(*r.ExternalURL) {
						errs = append(errs, ValidationError{"resource.iframe_not_allowed", campo,
							fmt.Sprintf("El iframe «%s» apunta a un dominio no autorizado.", r.Title)})
					}
				}
				if r.Type == TypeRichText && (r.ContentMD == nil || strings.TrimSpace(*r.ContentMD) == "") {
					errs = append(errs, ValidationError{"resource.not_available", campo,
						fmt.Sprintf("«%s» no tiene contenido.", r.Title)})
				}
			}
		}
	}
	if visiblesTotales == 0 {
		errs = append(errs, ValidationError{"structure.minimum_not_met", "modules",
			"La versión necesita al menos un módulo con una unidad y un recurso visible."})
	}

	return errs
}

// ------------------------------------------------------------------ slug ---

var noAlfanumerico = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify produce el identificador legible del curso para el catálogo.
func Slugify(titulo string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(titulo)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '_':
			b.WriteRune('-')
		default:
			// Se transliteran los acentos más comunes del español; el resto cae.
			if s, ok := translit[r]; ok {
				b.WriteString(s)
			} else {
				b.WriteRune('-')
			}
		}
	}
	s := noAlfanumerico.ReplaceAllString(b.String(), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "curso"
	}
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	return s
}

var translit = map[rune]string{
	'á': "a", 'é': "e", 'í': "i", 'ó': "o", 'ú': "u", 'ü': "u", 'ñ': "n",
}
