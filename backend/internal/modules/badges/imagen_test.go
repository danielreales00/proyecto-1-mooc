package badges

import (
	"strings"
	"testing"
	"time"
)

func ejemplo() Badge {
	return Badge{
		PublicCode:    "aaea5V6SReQzOt0XMONDwQ",
		StudentName:   "Estudiante Uno",
		CourseTitle:   "Fundamentos de Cloud",
		VersionNumber: 1,
		IssuedAt:      time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC),
	}
}

func TestImagenEsDeterminista(t *testing.T) {
	// La clave del objeto se deriva del código público, así que reintentar el
	// trabajo reescribe el mismo destino: el contenido tiene que coincidir o
	// dejaría de ser idempotente (ADR-0008).
	a, b := Imagen(ejemplo()), Imagen(ejemplo())
	if string(a) != string(b) {
		t.Fatal("dos llamadas con los mismos datos producen bytes distintos")
	}
}

func TestImagenLlevaLoQueSeVerifica(t *testing.T) {
	got := string(Imagen(ejemplo()))
	for _, quiero := range []string{
		"Estudiante Uno", "Fundamentos de Cloud", "aaea5V6SReQzOt0XMONDwQ",
		"13 Sep 2026", "versión 1",
	} {
		if !strings.Contains(got, quiero) {
			t.Errorf("la imagen no contiene %q", quiero)
		}
	}
}

func TestImagenNuncaLlevaElCorreo(t *testing.T) {
	// CA-07: el bucket es de lectura anónima, así que lo que entre aquí queda
	// expuesto a cualquiera que tenga el enlace.
	b := ejemplo()
	b.StudentName = "Estudiante Uno"
	got := string(Imagen(b))
	if strings.Contains(got, "@") {
		t.Fatalf("la imagen pública contiene una arroba:\n%s", got)
	}
}

func TestImagenEscapaElTitulo(t *testing.T) {
	// El título lo escribe un profesor. Sin escapar, rompería el SVG o
	// inyectaría marcado en un archivo que se sirve público.
	b := ejemplo()
	b.CourseTitle = `Go & <script>alert("x")</script>`
	got := string(Imagen(b))
	if strings.Contains(got, "<script>") {
		t.Fatal("el título se insertó sin escapar")
	}
	if !strings.Contains(got, "&amp;") {
		t.Fatal("el ampersand no se escapó")
	}
}

func TestImagenRevocadaSeDistingue(t *testing.T) {
	b := ejemplo()
	t3 := b.IssuedAt.Add(time.Hour)
	b.RevokedAt = &t3
	if !strings.Contains(string(Imagen(b)), "REVOCADA") {
		t.Fatal("una insignia revocada se dibuja igual que una válida")
	}
}

func TestClaveImagenEsDeterminista(t *testing.T) {
	if ClaveImagen("abc") != "badges/abc.svg" {
		t.Fatalf("clave inesperada: %s", ClaveImagen("abc"))
	}
}

func TestURLImagenVacíaSinAlmacen(t *testing.T) {
	s := NewService(nil, nil, "mooc-badges", "", nil)
	if u := s.URLImagen(ejemplo()); u != "" {
		t.Fatalf("sin origen público la URL debe quedar vacía, no %q", u)
	}
}
