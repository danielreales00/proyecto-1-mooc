// Package ffmpeg implementa el puerto media.Transcodificador invocando los
// binarios de FFmpeg (ADR-0011).
//
// Es un adaptador, no dominio: aquí vive todo lo que sabe de líneas de órdenes,
// códecs y rarezas de FFmpeg. Qué variantes hay que generar lo decide
// media/hls.go, que no sabe que FFmpeg existe.
//
// El binario solo está en la imagen `worker-media` (ver backend/Dockerfile).
// Ningún otro proceso lo necesita: los trabajos de transcodificación viajan por
// la cola `bulk`, que solo consume ese worker.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"mooc/backend/internal/modules/media"
)

// Nombres de los binarios. Constantes: nunca se construyen con datos de
// entrada, que es lo que haría peligroso lanzar un proceso.
const (
	binFFmpeg  = "ffmpeg"
	binFFprobe = "ffprobe"
)

type Runner struct{}

func New() *Runner { return &Runner{} }

// ErrIlegible marca un original que FFmpeg no puede decodificar: un archivo
// corrupto, truncado o que solo parece vídeo por sus primeros bytes.
//
// Se distingue del resto de errores porque **no se arregla reintentando**. El
// servicio lo traduce a un asset `failed` en vez de dejar que el trabajo agote
// sus tres intentos y ensucie la DLQ, que debe significar "algo va mal en el
// sistema", no "alguien subió un archivo roto".
var ErrIlegible = errors.New("ffmpeg: el archivo no se puede decodificar")

// Inspeccionar lee los metadatos reales del archivo. La altura que devuelve es
// la que decide la escalera, así que sale del contenido, nunca de lo que
// declare quien subió el archivo.
func (r *Runner) Inspeccionar(ctx context.Context, ruta string) (media.Medios, error) {
	salida, err := correr(ctx, binFFprobe,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		ruta,
	)
	if err != nil {
		return media.Medios{}, err
	}

	var doc struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(salida, &doc); err != nil {
		return media.Medios{}, fmt.Errorf("%w: ffprobe devolvió algo que no es JSON: %v", ErrIlegible, err)
	}

	var m media.Medios
	for _, s := range doc.Streams {
		switch s.CodecType {
		case "video":
			// Una carátula incrustada en un MP3 también es un stream de vídeo,
			// pero sin dimensiones utilizables: no convierte el audio en vídeo.
			if s.Width > 0 && s.Height > 0 {
				m.TieneVideo = true
				if s.Height > m.Alto {
					m.Ancho, m.Alto = s.Width, s.Height
				}
			}
		case "audio":
			m.TieneAudio = true
		}
	}
	if !m.TieneVideo && !m.TieneAudio {
		return media.Medios{}, fmt.Errorf("%w: no tiene ni vídeo ni audio", ErrIlegible)
	}
	if d, err := strconv.ParseFloat(doc.Format.Duration, 64); err == nil {
		m.DuracionSegundos = d
	}
	return m, nil
}

// TranscodificarHLS genera una variante completa —playlist y segmentos— dentro
// de `destino`. El original no se toca: se lee y nada más (CA-02).
func (r *Runner) TranscodificarHLS(ctx context.Context, entrada, destino string, v media.Variante) error {
	if err := os.MkdirAll(destino, 0o750); err != nil {
		return err
	}

	args := []string{"-nostdin", "-y", "-i", entrada}
	if v.VideoKbps > 0 {
		args = append(args,
			"-vf", "scale="+v.Escala(),
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-profile:v", "main",
			"-b:v", kbps(v.VideoKbps),
			"-maxrate", kbps(v.VideoKbps*107/100),
			"-bufsize", kbps(v.VideoKbps*15/10),
			// Un keyframe cada segmento: sin esto los cortes no caen donde el
			// troceador los pide y el cambio de calidad da saltos.
			"-g", strconv.Itoa(media.DuracionSegmento*30),
			"-keyint_min", strconv.Itoa(media.DuracionSegmento*30),
			"-sc_threshold", "0",
		)
	} else {
		args = append(args, "-vn")
	}
	args = append(args,
		"-c:a", "aac",
		"-b:a", kbps(v.AudioKbps),
		"-ac", "2",
		"-f", "hls",
		"-hls_time", strconv.Itoa(media.DuracionSegmento),
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(destino, "seg-%03d.ts"),
		filepath.Join(destino, "index.m3u8"),
	)

	_, err := correr(ctx, binFFmpeg, args...)
	return err
}

// Poster extrae el fotograma de portada. Si el vídeo dura menos que el segundo
// pedido, FFmpeg no escribe nada y no falla: por eso se reintenta desde el
// principio en vez de dar por buena una portada que no existe.
func (r *Runner) Poster(ctx context.Context, entrada, salida string, segundo int) error {
	intentar := func(desde int) error {
		_, err := correr(ctx, binFFmpeg,
			"-nostdin", "-y",
			"-ss", strconv.Itoa(desde),
			"-i", entrada,
			"-frames:v", "1",
			"-q:v", "3",
			salida,
		)
		if err != nil {
			return err
		}
		info, err := os.Stat(salida)
		if err != nil || info.Size() == 0 {
			return fmt.Errorf("el fotograma del segundo %d salió vacío", desde)
		}
		return nil
	}

	if err := intentar(segundo); err == nil {
		return nil
	}
	return intentar(0)
}

func kbps(v int) string { return strconv.Itoa(v) + "k" }

// correr ejecuta el binario y devuelve su salida estándar. FFmpeg escribe el
// diagnóstico en stderr, así que es lo que se adjunta al error: sin eso, un
// fallo de codificación llega al log como "exit status 1" y no se puede
// diagnosticar.
func correr(ctx context.Context, binario string, args ...string) ([]byte, error) {
	// #nosec G204 -- el binario es una constante del paquete y los argumentos
	// salen de la escalera del ADR-0011 y de rutas creadas con os.MkdirTemp,
	// nunca de datos de la petición.
	cmd := exec.CommandContext(ctx, binario, args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		detalle := cola(errBuf.String(), 600)
		if esIlegible(detalle) {
			return nil, fmt.Errorf("%w: %s", ErrIlegible, detalle)
		}
		return nil, fmt.Errorf("%s %s: %w: %s", binario, args[0], err, detalle)
	}
	return out.Bytes(), nil
}

// esIlegible reconoce las quejas con las que FFmpeg dice que el archivo está
// roto, frente a las que indican un problema del sistema (disco, permisos).
func esIlegible(detalle string) bool {
	for _, marca := range []string{
		"Invalid data found when processing input",
		"moov atom not found",
		"Invalid argument",
		"does not contain any stream",
		"End of file",
	} {
		if strings.Contains(detalle, marca) {
			return true
		}
	}
	return false
}

func cola(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
