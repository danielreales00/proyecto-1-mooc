package admin

import "testing"

func ptr(s string) *string { return &s }

// La protección del último administrador es la regla que más fácil se rompe al
// tocar el código, y la que peor se paga: deja el sistema sin nadie que lo
// administre y sin forma de recuperarlo por la API (RF-02).
func TestDejaSinAdministrador(t *testing.T) {
	adminActivo := Usuario{Role: RolAdmin, Status: EstadoActivo}
	adminSuspendido := Usuario{Role: RolAdmin, Status: EstadoSuspendido}
	profesor := Usuario{Role: RolProfesor, Status: EstadoActivo}

	casos := []struct {
		nombre  string
		actual  Usuario
		cambio  CambioDeUsuario
		otros   int
		rechaza bool
	}{
		{
			nombre: "último admin: degradarlo a profesor",
			actual: adminActivo, cambio: CambioDeUsuario{Role: ptr(RolProfesor)}, otros: 0,
			rechaza: true,
		},
		{
			nombre: "último admin: suspenderlo",
			actual: adminActivo, cambio: CambioDeUsuario{Status: ptr(EstadoSuspendido)}, otros: 0,
			rechaza: true,
		},
		{
			nombre: "último admin: eliminarlo",
			actual: adminActivo, cambio: CambioDeUsuario{Status: ptr(EstadoEliminado)}, otros: 0,
			rechaza: true,
		},
		{
			nombre: "último admin: degradar Y suspender a la vez",
			actual: adminActivo, cambio: CambioDeUsuario{Role: ptr(RolEstudiante), Status: ptr(EstadoSuspendido)}, otros: 0,
			rechaza: true,
		},
		{
			// Un cambio que no le quita ni el rol ni la actividad es inofensivo.
			nombre: "último admin: reafirmarlo como admin activo",
			actual: adminActivo, cambio: CambioDeUsuario{Role: ptr(RolAdmin), Status: ptr(EstadoActivo)}, otros: 0,
			rechaza: false,
		},
		{
			nombre: "hay otro admin activo: degradarlo se permite",
			actual: adminActivo, cambio: CambioDeUsuario{Role: ptr(RolProfesor)}, otros: 1,
			rechaza: false,
		},
		{
			// Ya estaba suspendido, así que no es el último ACTIVO.
			nombre: "admin suspendido: cambiarlo no afecta",
			actual: adminSuspendido, cambio: CambioDeUsuario{Role: ptr(RolProfesor)}, otros: 0,
			rechaza: false,
		},
		{
			nombre: "un profesor cualquiera",
			actual: profesor, cambio: CambioDeUsuario{Status: ptr(EstadoSuspendido)}, otros: 0,
			rechaza: false,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := DejaSinAdministrador(c.actual, c.cambio, c.otros); got != c.rechaza {
				t.Fatalf("DejaSinAdministrador() = %v, se esperaba %v", got, c.rechaza)
			}
		})
	}
}

func TestNuevoUsuarioValidate(t *testing.T) {
	casos := []struct {
		nombre string
		in     NuevoUsuario
		code   string
	}{
		{"profesor válido", NuevoUsuario{"ana@mooc.local", "Ana", RolProfesor}, ""},
		{"administrador válido", NuevoUsuario{"ana@mooc.local", "Ana", RolAdmin}, ""},
		{"correo vacío", NuevoUsuario{"", "Ana", RolProfesor}, "email.required"},
		{"correo inválido", NuevoUsuario{"no-es-correo", "Ana", RolProfesor}, "email.invalid"},
		{"nombre vacío", NuevoUsuario{"ana@mooc.local", "  ", RolProfesor}, "full_name.required"},
		{
			// Crear estudiantes por administración saltaría la verificación de
			// correo del registro público.
			nombre: "estudiante: no se crea por administración",
			in:     NuevoUsuario{"ana@mooc.local", "Ana", RolEstudiante},
			code:   "role.invalid",
		},
		{"rol inventado", NuevoUsuario{"ana@mooc.local", "Ana", "rector"}, "role.invalid"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			errs := c.in.Validate()
			if c.code == "" {
				if !errs.Empty() {
					t.Fatalf("se esperaba válido, llegó: %v", errs)
				}
				return
			}
			for _, e := range errs {
				if e.Code == c.code {
					return
				}
			}
			t.Fatalf("se esperaba %q, llegaron: %v", c.code, errs)
		})
	}
}

func TestCambioDeUsuarioValidate(t *testing.T) {
	if errs := (CambioDeUsuario{}).Validate(); errs.Empty() {
		t.Error("un cambio vacío debería rechazarse")
	}
	if errs := (CambioDeUsuario{Role: ptr("rector")}).Validate(); errs.Empty() {
		t.Error("un rol desconocido debería rechazarse")
	}
	if errs := (CambioDeUsuario{Status: ptr(EstadoSuspendido)}).Validate(); !errs.Empty() {
		t.Errorf("suspender debería ser válido: %v", errs)
	}
}
