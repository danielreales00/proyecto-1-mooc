// Package migrations empaqueta el SQL en el binario para que cmd/migrate no
// dependa de rutas del sistema de archivos dentro del contenedor.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
