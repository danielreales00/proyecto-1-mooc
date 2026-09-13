package badges

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// SVG, no PNG: se dibuja con la biblioteca estándar y sin fuentes empotradas.
// Un PNG legible exigiría una dependencia de tipografía solo para esto, y el
// ADR-0002 pide justificar cada una. El SVG lo pinta cualquier navegador, pesa
// menos de dos kilobytes y se puede leer en texto plano si hace falta.
const (
	AnchoImagen = 600
	AltoImagen  = 380
	TipoImagen  = "image/svg+xml"
)

// ExtensionImagen es la del objeto en el almacén. La clave se deriva del código
// público, que ya es aleatorio, así que reintentar escribe en el mismo sitio con
// el mismo contenido (ADR-0008).
const ExtensionImagen = ".svg"

// ClaveImagen es determinista a propósito.
func ClaveImagen(codigoPublico string) string {
	return "badges/" + codigoPublico + ExtensionImagen
}

// Imagen dibuja la insignia. Es una función pura: mismos datos, mismos bytes.
//
// NO incluye el correo del estudiante (CA-07). La imagen es pública —el bucket
// tiene lectura anónima— así que cualquier dato que entre aquí queda expuesto.
func Imagen(b Badge) []byte {
	revocada := b.RevokedAt != nil

	franja, etiqueta := "#1f6f4a", "ACREDITACIÓN VERIFICADA"
	if revocada {
		franja, etiqueta = "#8a1c1c", "ACREDITACIÓN REVOCADA"
	}

	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" `+
		`viewBox="0 0 %d %d" role="img" aria-label="%s">`,
		AnchoImagen, AltoImagen, AnchoImagen, AltoImagen,
		esc(etiqueta+": "+b.CourseTitle))

	fmt.Fprintf(&s, `<rect width="%d" height="%d" rx="18" fill="#0f1720"/>`,
		AnchoImagen, AltoImagen)
	fmt.Fprintf(&s, `<rect x="0" y="0" width="%d" height="10" rx="5" fill="%s"/>`,
		AnchoImagen, franja)

	fmt.Fprintf(&s, `<text x="40" y="72" font-family="Helvetica,Arial,sans-serif" `+
		`font-size="15" letter-spacing="2.5" fill="%s">%s</text>`, franja, esc(etiqueta))

	fmt.Fprintf(&s, `<text x="40" y="138" font-family="Helvetica,Arial,sans-serif" `+
		`font-size="30" font-weight="bold" fill="#f4f6f8">%s</text>`,
		esc(recortar(b.StudentName, 34)))

	fmt.Fprintf(&s, `<text x="40" y="186" font-family="Helvetica,Arial,sans-serif" `+
		`font-size="19" fill="#c3ccd6">%s</text>`, esc(recortar(b.CourseTitle, 46)))
	fmt.Fprintf(&s, `<text x="40" y="214" font-family="Helvetica,Arial,sans-serif" `+
		`font-size="15" fill="#8b97a4">versión %d · %s</text>`,
		b.VersionNumber, esc(b.IssuedAt.UTC().Format("2 Jan 2006")))

	fmt.Fprintf(&s, `<line x1="40" y1="256" x2="%d" y2="256" stroke="#243040" stroke-width="1"/>`,
		AnchoImagen-40)

	fmt.Fprintf(&s, `<text x="40" y="294" font-family="Helvetica,Arial,sans-serif" `+
		`font-size="13" fill="#8b97a4">CÓDIGO DE VERIFICACIÓN</text>`)
	fmt.Fprintf(&s, `<text x="40" y="324" font-family="monospace" font-size="18" `+
		`fill="#f4f6f8">%s</text>`, esc(b.PublicCode))

	s.WriteString(`</svg>`)
	return []byte(s.String())
}

func esc(v string) string {
	var b strings.Builder
	// Los títulos de curso los escribe un profesor, así que pueden traer
	// comillas o ángulos: sin escapar romperían el documento o inyectarían
	// marcado en una imagen que se sirve públicamente.
	_ = xml.EscapeText(&b, []byte(v))
	return b.String()
}

func recortar(v string, max int) string {
	r := []rune(v)
	if len(r) <= max {
		return v
	}
	return string(r[:max-1]) + "…"
}
