// Package logging expone el logger estructurado del ADR-0009.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New devuelve el logger del proceso.
//
// `formato` decide el dialecto: `json` es el de siempre, y `gcp` renombra los
// campos que Cloud Logging interpreta. No se elige por entorno adivinado sino
// por variable, para que un despliegue distinto sea solo configuración distinta
// (ADR-0010).
func New(level, formato string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: nivel(level)}

	if strings.ToLower(formato) == "gcp" {
		opts.ReplaceAttr = aCloudLogging
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func nivel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// aCloudLogging traduce los nombres que slog escribe por defecto a los que
// Cloud Logging reconoce.
//
// Sin esto, todas las entradas llegan con gravedad `default` y un WARN no se
// distingue de un INFO en la consola ni en una alerta: el texto está, pero deja
// de ser consultable. Es el único acomodo al proveedor en todo el código, y
// vive en un solo sitio a propósito.
func aCloudLogging(grupos []string, a slog.Attr) slog.Attr {
	if len(grupos) > 0 {
		return a
	}
	switch a.Key {
	case slog.LevelKey:
		return slog.Attr{Key: "severity", Value: a.Value}
	case slog.MessageKey:
		return slog.Attr{Key: "message", Value: a.Value}
	case slog.TimeKey:
		return slog.Attr{Key: "timestamp", Value: a.Value}
	}
	return a
}
