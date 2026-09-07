package media

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// El MIME lo decide el servidor a partir de los bytes. Confiar en la extensión
// o en el Content-Type declarado es justo la vía por la que entra un ejecutable
// disfrazado de PDF (RF-05).
func TestDetectarMIME(t *testing.T) {
	casos := []struct {
		nombre   string
		cabecera []byte
		esperado string
	}{
		{"MP4 por ftyp", append([]byte{0, 0, 0, 0x20}, []byte("ftypisom")...), "video/mp4"},
		{"QuickTime", append([]byte{0, 0, 0, 0x14}, []byte("ftypqt  ")...), "video/quicktime"},
		{"audio MP4", append([]byte{0, 0, 0, 0x18}, []byte("ftypM4A ")...), "audio/mp4"},
		{"Matroska", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x01, 0x02}, "video/x-matroska"},
		{"PDF", []byte("%PDF-1.7\n"), "application/pdf"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := DetectarMIME(c.cabecera, ""); got != c.esperado {
				t.Fatalf("DetectarMIME() = %q, se esperaba %q", got, c.esperado)
			}
		})
	}

	t.Run("cae al valor por defecto cuando no reconoce la firma", func(t *testing.T) {
		if got := DetectarMIME([]byte("texto cualquiera"), "image/png"); got != "image/png" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestMIMEAceptable(t *testing.T) {
	casos := []struct {
		kind, mime string
		aceptable  bool
	}{
		{TipoVideo, "video/mp4", true},
		{TipoVideo, "video/mp4; codecs=avc1", true}, // se ignoran los parámetros
		{TipoVideo, "application/pdf", false},
		{TipoPDF, "application/pdf", true},
		{TipoPDF, "image/png", false},
		{TipoImagen, "image/jpeg", true},
		{TipoArchivo, "application/zip", true}, // el tipo genérico admite casi todo
		{TipoArchivo, "application/x-msdownload", false},
		{TipoImagen, "text/x-shellscript", false},
		{"inventado", "image/png", false},
	}
	for _, c := range casos {
		t.Run(c.kind+"/"+c.mime, func(t *testing.T) {
			if got := MIMEAceptable(c.kind, c.mime); got != c.aceptable {
				t.Fatalf("MIMEAceptable(%q,%q) = %v", c.kind, c.mime, got)
			}
		})
	}
}

// Un ejecutable no entra ni siquiera por el tipo genérico, que es donde más
// fácil se colaría.
func TestEjecutablesRechazadosEnTodoTipo(t *testing.T) {
	for _, kind := range []string{TipoVideo, TipoAudio, TipoImagen, TipoPDF, TipoSlides, TipoArchivo} {
		for _, mime := range []string{"application/x-msdownload", "text/x-shellscript", "application/x-executable"} {
			if MIMEAceptable(kind, mime) {
				t.Errorf("%s aceptó %s", kind, mime)
			}
		}
	}
}

func TestPartesNecesarias(t *testing.T) {
	casos := []struct {
		tamaño int64
		partes int
	}{
		{1, 1},
		{TamañoDeParte, 1},
		{TamañoDeParte + 1, 2},
		{TamañoDeParte * 3, 3},
		{TamañoDeParte*3 + 100, 4},
		{0, 1},
	}
	for _, c := range casos {
		if got := PartesNecesarias(c.tamaño, TamañoDeParte); got != c.partes {
			t.Errorf("PartesNecesarias(%d) = %d, se esperaba %d", c.tamaño, got, c.partes)
		}
	}
}

// La clave debe ser determinista: reintentar tiene que escribir en el mismo
// sitio para que la operación sea idempotente (ADR-0005).
func TestClaveOriginalEsDeterminista(t *testing.T) {
	id := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	a := ClaveOriginal(id, "clase.MP4")
	b := ClaveOriginal(id, "clase.MP4")
	if a != b {
		t.Fatalf("dos llamadas dieron claves distintas: %q y %q", a, b)
	}
	if !strings.HasSuffix(a, ".mp4") {
		t.Errorf("la extensión debería normalizarse a minúsculas: %q", a)
	}
	if !strings.Contains(a, id.String()) {
		t.Errorf("la clave debe contener el id del asset: %q", a)
	}
	if sinExt := ClaveOriginal(id, "sin_extension"); strings.HasSuffix(sinExt, ".") {
		t.Errorf("un nombre sin extensión no debe dejar un punto colgando: %q", sinExt)
	}
}

func TestNuevaCargaValidate(t *testing.T) {
	valida := NuevaCarga{
		Filename: "clase.mp4", ContentType: "video/mp4",
		SizeBytes: 1024, Kind: TipoVideo,
		SHA256: strings.Repeat("a", 64),
	}
	if errs := valida.Validate(); !errs.Empty() {
		t.Fatalf("se esperaba válida: %v", errs)
	}

	casos := []struct {
		nombre string
		mutar  func(*NuevaCarga)
		code   string
	}{
		{"sin nombre", func(c *NuevaCarga) { c.Filename = " " }, "filename.required"},
		{"tamaño cero", func(c *NuevaCarga) { c.SizeBytes = 0 }, "size.invalid"},
		{"demasiado grande", func(c *NuevaCarga) { c.SizeBytes = TamañoMaximo + 1 }, "size.too_large"},
		{"checksum corto", func(c *NuevaCarga) { c.SHA256 = "abc" }, "sha256.invalid"},
		{"checksum no hexadecimal", func(c *NuevaCarga) { c.SHA256 = strings.Repeat("z", 64) }, "sha256.invalid"},
		{"tipo desconocido", func(c *NuevaCarga) { c.Kind = "holograma" }, "kind.invalid"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in := valida
			c.mutar(&in)
			for _, e := range in.Validate() {
				if e.Code == c.code {
					return
				}
			}
			t.Fatalf("se esperaba %q, llegaron: %v", c.code, in.Validate())
		})
	}
}
