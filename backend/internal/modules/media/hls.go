package media

import (
	"fmt"
	"sort"
	"strings"
)

// Escalera de transcodificación y armado de manifiestos HLS (ADR-0011).
//
// Todo lo de este archivo es dominio puro: decide qué variantes se generan y
// cómo se escribe un manifiesto, sin saber nada de FFmpeg ni del almacén. Las
// reglas que aquí se codifican son condiciones de aceptación del SEG-4, así que
// tienen prueba propia en hls_test.go.

// Variante es un peldaño de la escalera.
type Variante struct {
	Nombre    string
	Ancho     int
	Alto      int
	VideoKbps int
	AudioKbps int
}

// Escalera son los peldaños del ADR-0011, de menor a mayor.
var Escalera = []Variante{
	{Nombre: "360p", Ancho: 640, Alto: 360, VideoKbps: 800, AudioKbps: 96},
	{Nombre: "720p", Ancho: 1280, Alto: 720, VideoKbps: 2500, AudioKbps: 128},
	{Nombre: "1080p", Ancho: 1920, Alto: 1080, VideoKbps: 5000, AudioKbps: 192},
}

// VarianteAudio es la única salida de un asset sin imagen.
var VarianteAudio = Variante{Nombre: "audio", AudioKbps: 128}

// DuracionSegmento en segundos (ADR-0011).
const DuracionSegmento = 6

// PosterSegundo es el fotograma que se extrae como portada.
const PosterSegundo = 3

// VariantesPara elige los peldaños que corresponden a un original de esta
// altura. **Nunca hacia arriba**: SEG-4 exige demostrar que no hay upscaling, y
// generar 1080p a partir de un 360p no añade información, solo peso y tiempo de
// CPU.
//
// Un original más bajo que el primer peldaño no se queda sin salida: se emite
// una sola variante con su altura real, que sigue sin ser un reescalado hacia
// arriba.
func VariantesPara(alto int, conVideo bool) []Variante {
	if !conVideo {
		return []Variante{VarianteAudio}
	}
	if alto <= 0 {
		return nil
	}

	var out []Variante
	for _, v := range Escalera {
		if v.Alto <= alto {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		v := Escalera[0]
		v.Nombre = fmt.Sprintf("%dp", alto)
		v.Alto = alto
		v.Ancho = 0 // lo calcula FFmpeg conservando la relación de aspecto
		out = append(out, v)
	}
	return out
}

// Escala es el filtro de FFmpeg. Se fija la altura y el ancho sale de ella:
// -2 pide el ancho par más cercano que conserve la relación de aspecto, porque
// un ancho impar rompe a los codificadores.
func (v Variante) Escala() string { return fmt.Sprintf("-2:%d", v.Alto) }

// BandwidthBps es lo que declara el manifiesto maestro: vídeo más audio, con el
// margen del 10 % que recomienda la especificación de HLS para el pico.
func (v Variante) BandwidthBps() int {
	return (v.VideoKbps + v.AudioKbps) * 1100
}

// Claves de los derivados. Deterministas a propósito (ADR-0005): reintentar un
// trabajo reescribe los mismos objetos en vez de acumular copias, que es la
// mitad de la idempotencia del CA-03. La otra mitad la pone la base, con el
// UNIQUE (asset_id, kind, variant) de media.asset_derivatives.
func ClaveMaster(assetID string) string { return "hls/" + assetID + "/master.m3u8" }

func ClavePlaylist(assetID, variante string) string {
	return "hls/" + assetID + "/" + variante + "/index.m3u8"
}

func PrefijoVariante(assetID, variante string) string {
	return "hls/" + assetID + "/" + variante + "/"
}

func ClavePoster(assetID string) string { return "posters/" + assetID + ".jpg" }

// Tipos de derivado, tal como los admite el CHECK de la tabla.
const (
	DerivadoMaster   = "hls_master"
	DerivadoVariante = "hls_variant"
	DerivadoPoster   = "poster"
)

// Derivado es una salida de la transcodificación.
type Derivado struct {
	Kind     string
	Variante string
	Key      string
	Bytes    int64
}

// MasterM3U8 arma el manifiesto maestro. `uri` decide cómo se referencia cada
// variante, porque quien lo sirve no siempre es quien lo guarda: en el almacén
// la referencia es relativa, y a través de la API es una URL firmada.
func MasterM3U8(vs []Variante, uri func(Variante) string) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	for _, v := range vs {
		b.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d", v.BandwidthBps()))
		if v.Alto > 0 && v.Ancho > 0 {
			b.WriteString(fmt.Sprintf(",RESOLUTION=%dx%d", v.Ancho, v.Alto))
		}
		b.WriteString("\n" + uri(v) + "\n")
	}
	return b.String()
}

// ReescribirSegmentos sustituye cada URI de segmento de una playlist de
// variante por lo que devuelva `url`.
//
// Hace falta porque el bucket de derivados es privado (ADR-0005). FFmpeg
// escribe los segmentos como nombres relativos, y un reproductor los resolvería
// contra la URL de la playlist: si esa URL viene firmada, la del segmento sale
// sin firma y el almacén responde 403. Sustituirlos por URLs firmadas absolutas
// es lo que hace que el vídeo se reproduzca sin abrir el bucket.
func ReescribirSegmentos(playlist string, url func(nombre string) (string, error)) (string, error) {
	lineas := strings.Split(playlist, "\n")
	for i, l := range lineas {
		limpia := strings.TrimSpace(l)
		if limpia == "" || strings.HasPrefix(limpia, "#") {
			continue
		}
		firmada, err := url(limpia)
		if err != nil {
			return "", fmt.Errorf("firmar el segmento %q: %w", limpia, err)
		}
		lineas[i] = firmada
	}
	return strings.Join(lineas, "\n"), nil
}

// NombresDeVariante devuelve los nombres ordenados, para comparar sin depender
// del orden en que el almacén liste los objetos.
func NombresDeVariante(vs []Variante) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Nombre)
	}
	sort.Strings(out)
	return out
}
