package main

import "testing"

func TestClaveDeLaSemilla(t *testing.T) {
	casos := []struct {
		nombre   string
		local    bool
		entorno  string
		quiere   string
		conError bool
	}{
		{"local sin variable usa la de demostración", true, "", claveDemo, false},
		{"local con variable usa la variable", true, "otra-clave", "otra-clave", false},
		{"fuera de local sin variable falla", false, "", "", true},
		{"fuera de local rechaza la de demostración", false, claveDemo, "", true},
		{"fuera de local usa la variable", false, "clave-de-secret-manager", "clave-de-secret-manager", false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got, err := claveDeLaSemilla(c.local, c.entorno)
			if (err != nil) != c.conError {
				t.Fatalf("error = %v, se esperaba error: %v", err, c.conError)
			}
			if got != c.quiere {
				t.Fatalf("clave = %q, se esperaba %q", got, c.quiere)
			}
		})
	}
}
