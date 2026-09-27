package ratelimit

import "testing"

func TestLimiteDe(t *testing.T) {
	porIP := Regla{Nombre: "login_ip", Limite: 60, PorIP: true}
	porCuenta := Regla{Nombre: "login_cuenta", Limite: 5}

	normal := New(nil)
	if normal.LimiteDe(porIP) != 60 || normal.LimiteDe(porCuenta) != 5 {
		t.Fatal("sin factor, los límites tienen que ser los declarados")
	}

	carga := New(nil).ConFactorIP(100)
	if got := carga.LimiteDe(porIP); got != 6000 {
		t.Fatalf("regla por IP con factor 100: %d, se esperaba 6000", got)
	}
	if got := carga.LimiteDe(porCuenta); got != 5 {
		t.Fatalf("la regla por cuenta no se toca: %d, se esperaba 5", got)
	}

	if got := New(nil).ConFactorIP(0).LimiteDe(porIP); got != 60 {
		t.Fatalf("un factor < 1 no puede bajar el límite: %d", got)
	}
}
