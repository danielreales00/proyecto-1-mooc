package queue

import (
	"reflect"
	"testing"

	"mooc/backend/internal/platform/jobs"
)

func TestParseQueues(t *testing.T) {
	casos := []struct {
		nombre string
		spec   string
		want   map[string]int
	}{
		{
			nombre: "las tres colas del ADR-0004",
			spec:   "critical=6,default=3,bulk=1",
			want:   map[string]int{"critical": 6, "default": 3, "bulk": 1},
		},
		{
			nombre: "tolera espacios",
			spec:   " critical = 6 , default = 3 ",
			want:   map[string]int{"critical": 6, "default": 3},
		},
		{
			nombre: "una sola cola",
			spec:   "bulk=1",
			want:   map[string]int{"bulk": 1},
		},
		{
			// Un peso ilegible no debe dejar al worker sin escuchar nada.
			nombre: "peso no numérico cae al valor por defecto",
			spec:   "critical=alto",
			want:   map[string]int{jobs.QueueDefault: 1},
		},
		{
			nombre: "peso cero se descarta",
			spec:   "critical=0,default=3",
			want:   map[string]int{"default": 3},
		},
		{
			nombre: "cadena vacía cae al valor por defecto",
			spec:   "",
			want:   map[string]int{jobs.QueueDefault: 1},
		},
		{
			nombre: "entradas sin signo igual se ignoran",
			spec:   "critical,default=3",
			want:   map[string]int{"default": 3},
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := ParseQueues(c.spec)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("ParseQueues(%q) = %v, se esperaba %v", c.spec, got, c.want)
			}
		})
	}
}
