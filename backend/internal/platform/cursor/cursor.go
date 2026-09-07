// Package cursor implementa la paginación por cursor del ADR-0002 y RT-05.
//
// Nunca por desplazamiento: con `OFFSET`, insertar una fila mientras alguien
// pagina le hace ver un elemento dos veces o saltárselo, y el coste crece con
// la profundidad. Un cursor apunta a la última fila entregada y la consulta
// sigue desde ahí.
package cursor

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Cursor señala la última fila de la página anterior.
//
// Lleva dos campos porque ordenar solo por fecha no es determinista: dos filas
// creadas en el mismo instante se devolverían en orden arbitrario, y una de las
// dos se perdería al paginar. El identificador desempata.
type Cursor struct {
	Fecha time.Time
	ID    string
}

// Codificar produce el valor opaco que ve el cliente. Es opaco a propósito: si
// se publicara su forma, alguien construiría uno a mano y quedaríamos atados a
// este esquema de ordenación.
func Codificar(fecha time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(fecha.UTC().Format(time.RFC3339Nano) + "|" + id))
}

// Decodificar interpreta el cursor. Uno inválido se trata como «desde el
// principio»: es preferible devolver la primera página a fallar con un error
// que el cliente no puede arreglar.
func Decodificar(valor string) (Cursor, bool) {
	if valor == "" {
		return Cursor{}, false
	}
	crudo, err := base64.RawURLEncoding.DecodeString(valor)
	if err != nil {
		return Cursor{}, false
	}
	partes := strings.SplitN(string(crudo), "|", 2)
	if len(partes) != 2 {
		return Cursor{}, false
	}
	fecha, err := time.Parse(time.RFC3339Nano, partes[0])
	if err != nil {
		return Cursor{}, false
	}
	return Cursor{Fecha: fecha, ID: partes[1]}, true
}

// Limite acota el tamaño de página.
func Limite(pedido, porDefecto, maximo int) int {
	if pedido <= 0 {
		return porDefecto
	}
	if pedido > maximo {
		return maximo
	}
	return pedido
}

// Pagina recorta la lista al tamaño pedido y devuelve el cursor siguiente.
//
// Se consulta un elemento MÁS de los que se van a devolver: si aparece, hay
// página siguiente. Así se sabe sin contar el total, que en una tabla grande
// es la consulta más cara de todas.
func Pagina[T any](filas []T, limite int, clave func(T) (time.Time, string)) ([]T, *string) {
	if len(filas) <= limite {
		return filas, nil
	}
	filas = filas[:limite]
	fecha, id := clave(filas[len(filas)-1])
	siguiente := Codificar(fecha, id)
	return filas, &siguiente
}

// Condicion devuelve el fragmento SQL para seguir desde el cursor, en orden
// descendente por fecha. Los parámetros van numerados desde `n`.
func Condicion(n int) string {
	return fmt.Sprintf("(created_at, id) < ($%d, $%d)", n, n+1)
}
