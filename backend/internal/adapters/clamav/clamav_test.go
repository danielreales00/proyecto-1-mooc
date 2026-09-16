package clamav

import (
	"errors"
	"testing"

	"mooc/backend/internal/modules/media"
)

// La respuesta de clamd es una línea de texto, y de cómo se lea depende que un
// archivo infectado pase por limpio. Por eso tiene prueba propia.
func TestInterpretar(t *testing.T) {
	casos := []struct {
		nombre    string
		respuesta string
		limpio    bool
		firma     string
		err       error
	}{
		{"limpio", "stream: OK", true, "", nil},
		{"infectado", "stream: Eicar-Test-Signature FOUND", false, "Eicar-Test-Signature", nil},
		{"infectado con ruido alrededor", "1: stream: Win.Test.EICAR_HDB-1 FOUND", false,
			"1: stream: Win.Test.EICAR_HDB-1", nil},
		{"demasiado grande", "INSTREAM size limit exceeded. ERROR", false, "", media.ErrLimiteDelEscaner},
		{"vacía", "", false, "", errSinRespuesta},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			v, err := interpretar(c.respuesta)

			if c.err != nil {
				if !errors.Is(err, c.err) {
					t.Fatalf("error = %v, se esperaba %v", err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if v.Limpio != c.limpio {
				t.Fatalf("limpio = %v, se esperaba %v", v.Limpio, c.limpio)
			}
			if !c.limpio && v.Firma != c.firma {
				t.Fatalf("firma = %q, se esperaba %q", v.Firma, c.firma)
			}
		})
	}
}

// Una respuesta que no se entiende NUNCA puede leerse como "limpio": ante la
// duda, el archivo no pasa.
func TestRespuestaDesconocidaNoEsLimpia(t *testing.T) {
	v, err := interpretar("stream: algo que clamd no debería decir")
	if err == nil {
		t.Fatal("una respuesta desconocida debe ser un error")
	}
	if v.Limpio {
		t.Fatal("una respuesta desconocida no puede dar por limpio el archivo")
	}
}
