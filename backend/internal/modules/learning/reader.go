package learning

import (
	"bytes"
	"io"
)

// newReader existe para volver a decodificar el cuerpo ya leído: el JSON se
// inspecciona primero en crudo para poder auditar un intento de manipulación.
func newReader(b []byte) io.Reader { return bytes.NewReader(b) }
