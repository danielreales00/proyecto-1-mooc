// Package clamav implementa media.Antivirus hablando INSTREAM con clamd
// (ADR-0011).
//
// El protocolo es lo bastante simple como para no añadir una dependencia
// (ADR-0002): se abre un socket, se manda `zINSTREAM\0`, se envían trozos
// precedidos de su longitud en 4 bytes big-endian, se cierra con una longitud
// cero y clamd responde una línea.
//
// Se transmite por trozos a propósito: el archivo puede pesar gigabytes y el
// worker no debe cargarlo en memoria para escanearlo.
package clamav

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"mooc/backend/internal/modules/media"
)

// TamanoTrozo es lo que se envía en cada escritura. 64 KiB es el tamaño con el
// que trabaja clamd internamente; trozos mayores no aceleran nada.
const TamanoTrozo = 64 * 1024

// Tiempos de espera. El escaneo de un archivo grande tarda, pero un clamd que
// no responde en diez minutos está caído, no ocupado.
const (
	TiempoConexion = 10 * time.Second
	TiempoEscaneo  = 10 * time.Minute
)

type Cliente struct {
	addr string
}

func New(addr string) *Cliente { return &Cliente{addr: addr} }

// Escanear transmite el contenido y devuelve el veredicto.
func (c *Cliente) Escanear(ctx context.Context, r io.Reader) (media.Veredicto, error) {
	conn, err := (&net.Dialer{Timeout: TiempoConexion}).DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return media.Veredicto{}, fmt.Errorf("conectar con clamd en %s: %w", c.addr, err)
	}
	defer conn.Close()

	limite := time.Now().Add(TiempoEscaneo)
	if fin, ok := ctx.Deadline(); ok && fin.Before(limite) {
		limite = fin
	}
	if err := conn.SetDeadline(limite); err != nil {
		return media.Veredicto{}, err
	}

	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return media.Veredicto{}, fmt.Errorf("iniciar INSTREAM: %w", err)
	}

	if err := transmitir(conn, r); err != nil {
		// Si clamd corta a media transmisión suele ser porque se pasó el
		// límite; su respuesta lo aclara, así que se intenta leer antes de
		// dar por perdido el escaneo.
		if respuesta, errLectura := leerRespuesta(conn); errLectura == nil {
			return interpretar(respuesta)
		}
		return media.Veredicto{}, err
	}

	respuesta, err := leerRespuesta(conn)
	if err != nil {
		return media.Veredicto{}, err
	}
	return interpretar(respuesta)
}

func transmitir(conn net.Conn, r io.Reader) error {
	buf := make([]byte, TamanoTrozo)
	cabecera := make([]byte, 4)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			// Read nunca devuelve más que len(buf), pero la comprobación está
			// escrita: de ella depende que la longitud declarada y los bytes
			// enviados no se separen, y un desajuste ahí descuadra el protocolo
			// entero.
			if n > len(buf) {
				return fmt.Errorf("lectura de %d bytes en un buffer de %d", n, len(buf))
			}
			// #nosec G115 -- n está entre 1 y len(buf), que son 64 KiB: la
			// conversión a uint32 no puede desbordar. La comprobación de
			// arriba lo garantiza; gosec no la sigue.
			binary.BigEndian.PutUint32(cabecera, uint32(n))
			if _, err := conn.Write(cabecera); err != nil {
				return fmt.Errorf("enviar la longitud del trozo: %w", err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return fmt.Errorf("enviar el trozo: %w", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("leer el archivo por escanear: %w", err)
		}
	}
	// Longitud cero: fin del envío.
	binary.BigEndian.PutUint32(cabecera, 0)
	if _, err := conn.Write(cabecera); err != nil {
		return fmt.Errorf("cerrar el envío: %w", err)
	}
	return nil
}

func leerRespuesta(conn net.Conn) (string, error) {
	linea, err := bufio.NewReader(conn).ReadString(0)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("leer la respuesta de clamd: %w", err)
	}
	return strings.TrimRight(strings.TrimSpace(linea), "\x00"), nil
}

var errSinRespuesta = errors.New("clamav: respuesta vacía")

// interpretar traduce la línea de clamd. Las tres formas que importan son
// `stream: OK`, `stream: <firma> FOUND` y los errores de tamaño.
func interpretar(respuesta string) (media.Veredicto, error) {
	switch {
	case respuesta == "":
		return media.Veredicto{}, errSinRespuesta
	case strings.HasSuffix(respuesta, "OK"):
		return media.Veredicto{Limpio: true}, nil
	case strings.HasSuffix(respuesta, "FOUND"):
		firma := strings.TrimSpace(strings.TrimSuffix(respuesta, "FOUND"))
		firma = strings.TrimSpace(strings.TrimPrefix(firma, "stream:"))
		return media.Veredicto{Limpio: false, Firma: firma}, nil
	case strings.Contains(respuesta, "size limit exceeded"),
		strings.Contains(respuesta, "INSTREAM size limit"):
		// El del dominio, no uno propio: quien decide qué hacer con un archivo
		// que no se puede analizar entero es el servicio (ADR-0001).
		return media.Veredicto{}, media.ErrLimiteDelEscaner
	default:
		return media.Veredicto{}, fmt.Errorf("clamav: respuesta inesperada: %q", respuesta)
	}
}
