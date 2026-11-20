// Command funcion sirve en local la función de insignias con el mismo marco que
// usa Cloud Run functions, para probarla sin desplegarla (make prueba-funcion).
// En la nube no se usa: el buildpack de Go genera su propio main.
package main

import (
	"log/slog"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"

	funcion "mooc/backend"
)

func main() {
	// El marco sirve la función que nombra FUNCTION_TARGET, como en la nube.
	if os.Getenv("FUNCTION_TARGET") == "" {
		_ = os.Setenv("FUNCTION_TARGET", funcion.Nombre)
	}
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}
	slog.Info("función escuchando", "puerto", puerto, "target", os.Getenv("FUNCTION_TARGET"))
	if err := funcframework.Start(puerto); err != nil {
		slog.Error("la función no pudo arrancar", "error", err)
		os.Exit(1)
	}
}
