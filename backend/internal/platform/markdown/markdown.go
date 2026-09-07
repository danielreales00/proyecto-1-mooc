// Package markdown normaliza y renderiza el contenido textual (ADR-0014).
//
// ALCANCE ACTUAL: la normalización es conservadora —finales de línea, espacios
// sobrantes, líneas en blanco, estilo de viñeta— y no la biyección completa
// AST↔Markdown que describe el ADR-0014. Lo que sí está completo es lo que
// tiene consecuencias de seguridad: el HTML crudo se descarta en el parseo y
// el renderizado se sanea con lista blanca.
//
// Pendiente: el renderizador Markdown→Markdown sobre el AST de goldmark, con
// su prueba de ida y vuelta por tipo de nodo.
package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

const maxLongitud = 256 * 1024

var (
	// Referencias a binarios: asset://{uuid}. Nunca URLs firmadas, que caducan.
	refAsset = regexp.MustCompile(`asset://([0-9a-fA-F-]{36})`)
	// Esquemas peligrosos en enlaces e imágenes.
	esquemaPeligroso = regexp.MustCompile(`(?i)\]\(\s*(javascript|data|vbscript):`)
	espaciosFinales  = regexp.MustCompile(`[ \t]+\n`)
	lineasEnBlanco   = regexp.MustCompile(`\n{3,}`)
	vinetaAlterna    = regexp.MustCompile(`(?m)^(\s*)[*+](\s+)`)
	htmlCrudo        = regexp.MustCompile(`(?i)<\s*(script|iframe|object|embed|style|link|meta|form)\b`)
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(
		// Sin HTML crudo: se descarta en el parseo, antes de sanear. Dos
		// barreras, no una (ADR-0014).
		html.WithXHTML(),
	),
)

var saneador = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").OnElements("code", "pre")
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

// Canonicalizar valida y normaliza el Markdown, y extrae las referencias a
// binarios. Aplicarlo dos veces produce el mismo resultado, lo que hace estable
// el ETag y evita que el autosave genere revisiones falsas.
func Canonicalizar(entrada string) (string, []uuid.UUID, error) {
	if len(entrada) > maxLongitud {
		return "", nil, fmt.Errorf("el contenido supera %d KiB", maxLongitud/1024)
	}
	if m := htmlCrudo.FindString(entrada); m != "" {
		return "", nil, fmt.Errorf("no se admite HTML crudo (%s)", strings.TrimSpace(m))
	}
	if m := esquemaPeligroso.FindString(entrada); m != "" {
		return "", nil, fmt.Errorf("esquema de enlace no permitido en %q", strings.TrimSpace(m))
	}

	s := strings.ReplaceAll(entrada, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	s = espaciosFinales.ReplaceAllString(s, "\n")
	s = vinetaAlterna.ReplaceAllString(s, "$1-$2") // viñeta canónica: "-"
	s = lineasEnBlanco.ReplaceAllString(s, "\n\n") // exactamente una en blanco
	s = strings.TrimSpace(s)
	if s != "" {
		s += "\n"
	}

	// Comprobación de idempotencia: si normalizar dos veces difiere, el
	// contenido tiene algo que no sabemos tratar y es mejor saberlo aquí.
	if s != "" {
		otra := lineasEnBlanco.ReplaceAllString(
			vinetaAlterna.ReplaceAllString(espaciosFinales.ReplaceAllString(s, "\n"), "$1-$2"), "\n\n")
		if strings.TrimSpace(otra)+"\n" != s {
			return "", nil, fmt.Errorf("la normalización no es estable para este contenido")
		}
	}

	var refs []uuid.UUID
	vistos := map[uuid.UUID]bool{}
	for _, m := range refAsset.FindAllStringSubmatch(s, -1) {
		if id, err := uuid.Parse(m[1]); err == nil && !vistos[id] {
			vistos[id] = true
			refs = append(refs, id)
		}
	}
	return s, refs, nil
}

// RenderHTML convierte el Markdown canónico a HTML saneado, para la
// previsualización y para el consumo del estudiante.
func RenderHTML(entrada string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(entrada), &buf); err != nil {
		return "", fmt.Errorf("renderizar markdown: %w", err)
	}
	return saneador.Sanitize(buf.String()), nil
}
