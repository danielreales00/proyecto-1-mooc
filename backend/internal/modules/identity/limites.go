package identity

import (
	"time"

	"mooc/backend/internal/platform/ratelimit"
)

// Límites de los endpoints de identidad. Los números viven aquí, junto al
// módulo que protegen, y no repartidos por la configuración: quien lee el
// módulo ve de un vistazo qué se está frenando y con qué holgura.
var (
	// LimiteLoginCuenta frena la fuerza bruta contra UNA cuenta. Cinco por
	// minuto es cómodo para quien se equivoca al teclear y muy incómodo para
	// un diccionario.
	LimiteLoginCuenta = ratelimit.Regla{Nombre: "login_cuenta", Limite: 5, Ventana: time.Minute}

	// LimiteLoginIP frena el rociado de contraseñas contra MUCHAS cuentas
	// desde una misma máquina. Es holgado a propósito: una universidad entera
	// puede salir por una sola IP.
	LimiteLoginIP = ratelimit.Regla{Nombre: "login_ip", Limite: 60, Ventana: time.Minute}

	// LimiteRegistro evita que alguien llene la tabla de usuarios y sature el
	// envío de correo.
	LimiteRegistro = ratelimit.Regla{Nombre: "register", Limite: 10, Ventana: time.Hour}

	// LimiteVerificacion protege el token de verificación, que son 32 bytes
	// aleatorios: adivinarlo es inviable, pero no hay razón para dejar que lo
	// intenten a ritmo libre.
	LimiteVerificacion = ratelimit.Regla{Nombre: "verify_email", Limite: 20, Ventana: time.Minute}
)
