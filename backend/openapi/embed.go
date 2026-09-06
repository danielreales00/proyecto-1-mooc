// Package openapi empaqueta el contrato en el binario para poder servirlo.
//
// Que la API sirva su propio contrato evita la deriva silenciosa: si el
// archivo no está, el binario no compila.
package openapi

import _ "embed"

//go:embed openapi.yaml
var Spec []byte
