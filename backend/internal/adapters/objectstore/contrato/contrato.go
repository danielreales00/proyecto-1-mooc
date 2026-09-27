// Package contrato es la suite que todo adaptador de objectstore.Almacen debe
// pasar, contra el almacén real: MinIO en local, Cloud Storage en GCP.
//
// Recorre lo que hace un cliente de verdad: sube las partes con las URL
// firmadas por HTTP, sin pasar por el adaptador, se interrumpe, pregunta qué
// falta, reanuda, completa, lee por URL firmada y mueve a cuarentena. Un
// adaptador que pasa esto sostiene RF-05 y el flujo de media sin tocar el
// módulo.
package contrato

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"mooc/backend/internal/adapters/objectstore"
)

type Config struct {
	Bucket     string // donde se sube, como originals
	Cuarentena string // destino de Mover
	// TamParte y Partes definen la carga. S3 exige 5 MiB en toda parte salvo
	// la última; GCS no pone mínimo y conviene pasar de 32 partes para
	// ejercitar la composición por niveles.
	TamParte int
	Partes   int
	// Borrar limpia un objeto al terminar. El puerto no tiene borrado porque
	// el dominio no lo usa; cada adaptador lo aporta con su cliente.
	Borrar func(ctx context.Context, bucket, key string) error
}

func Probar(t *testing.T, a objectstore.Almacen, c Config) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	base := "prueba-contrato/" + aleatorio(t) + "/"

	t.Run("ping", func(t *testing.T) {
		if err := a.Ping(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("carga por partes reanudable", func(t *testing.T) {
		key := base + "video.bin"
		datos := make([]byte, c.TamParte*(c.Partes-1)+c.TamParte/3)
		_, _ = rand.Read(datos)
		trozo := func(n int) []byte {
			ini := (n - 1) * c.TamParte
			return datos[ini:min(ini+c.TamParte, len(datos))]
		}

		id, err := a.CrearMultipart(ctx, c.Bucket, key, "video/mp4")
		if err != nil {
			t.Fatal(err)
		}
		etags := map[int]string{}

		// Primera sesión: se sube todo menos la última parte y el cliente
		// se «cae».
		for n := 1; n < c.Partes; n++ {
			etags[n] = subirParte(ctx, t, a, c.Bucket, key, id, n, trozo(n))
		}

		// Reanudar: el almacén dice qué tiene.
		hay, err := a.PartesSubidas(ctx, c.Bucket, key, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(hay) != c.Partes-1 {
			t.Fatalf("PartesSubidas: %d partes, se esperaban %d", len(hay), c.Partes-1)
		}
		for i, p := range hay {
			if p.Numero != i+1 {
				t.Fatalf("PartesSubidas no viene ordenado: posición %d tiene la parte %d", i, p.Numero)
			}
			if strings.Trim(p.ETag, `"`) != etags[p.Numero] {
				t.Fatalf("parte %d: ETag %q en el almacén, %q al subirla", p.Numero, p.ETag, etags[p.Numero])
			}
			if p.Bytes != int64(len(trozo(p.Numero))) {
				t.Fatalf("parte %d: %d bytes, se subieron %d", p.Numero, p.Bytes, len(trozo(p.Numero)))
			}
		}

		// Segunda sesión: solo lo que falta, con una URL firmada de nuevo.
		etags[c.Partes] = subirParte(ctx, t, a, c.Bucket, key, id, c.Partes, trozo(c.Partes))

		partes := make([]objectstore.Parte, 0, c.Partes)
		for n := c.Partes; n >= 1; n-- { // desordenadas a propósito
			partes = append(partes, objectstore.Parte{Numero: n, ETag: etags[n]})
		}

		// Un ETag que no corresponde no cierra la carga.
		malas := append([]objectstore.Parte(nil), partes...)
		malas[0].ETag = strings.Repeat("0", 32)
		if err := a.CompletarMultipart(ctx, c.Bucket, key, id, malas); err == nil {
			t.Fatal("CompletarMultipart aceptó un ETag que no corresponde")
		}

		if err := a.CompletarMultipart(ctx, c.Bucket, key, id, partes); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.Borrar(ctx, c.Bucket, key) }()

		tam, err := a.Info(ctx, c.Bucket, key)
		if err != nil {
			t.Fatal(err)
		}
		if tam != int64(len(datos)) {
			t.Fatalf("Info: %d bytes, se subieron %d", tam, len(datos))
		}
		igual(t, "Abrir", leer(t, a, c.Bucket, key), datos)

		u, err := a.PresignGet(ctx, c.Bucket, key, 5*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		igual(t, "PresignGet", descargar(ctx, t, u), datos)

		if _, err := a.PartesSubidas(ctx, c.Bucket, key, id); err == nil {
			t.Fatal("la carga sigue abierta después de completarla")
		}
	})

	t.Run("abortar descarta las partes", func(t *testing.T) {
		key := base + "abortado.bin"
		id, err := a.CrearMultipart(ctx, c.Bucket, key, "video/mp4")
		if err != nil {
			t.Fatal(err)
		}
		subirParte(ctx, t, a, c.Bucket, key, id, 1, make([]byte, c.TamParte))
		if err := a.AbortarMultipart(ctx, c.Bucket, key, id); err != nil {
			t.Fatal(err)
		}
		if _, err := a.PartesSubidas(ctx, c.Bucket, key, id); err == nil {
			t.Fatal("la carga sigue abierta después de abortarla")
		}
		if _, err := a.Info(ctx, c.Bucket, key); err == nil {
			t.Fatal("abortar dejó un objeto final")
		}
	})

	t.Run("subir y mover a cuarentena", func(t *testing.T) {
		key := base + "sospechoso.bin"
		datos := []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR")
		if err := a.Subir(ctx, c.Bucket, key, "application/octet-stream", datos); err != nil {
			t.Fatal(err)
		}
		if err := a.Mover(ctx, c.Bucket, key, c.Cuarentena, key); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.Borrar(ctx, c.Cuarentena, key) }()
		if _, err := a.Info(ctx, c.Bucket, key); err == nil {
			t.Fatal("Mover dejó el original")
		}
		igual(t, "Abrir tras Mover", leer(t, a, c.Cuarentena, key), datos)
	})
}

// subirParte hace lo que el cliente: PUT de los bytes a la URL firmada. Devuelve
// el ETag sin comillas, como lo guarda la colección de Postman.
func subirParte(ctx context.Context, t *testing.T, a objectstore.Almacen,
	bucket, key, id string, n int, datos []byte) string {

	t.Helper()
	u, err := a.PresignPart(ctx, bucket, key, id, n, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(datos))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		cuerpo, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		t.Fatalf("subir la parte %d: %s\n%s", n, res.Status, cuerpo)
	}
	etag := strings.Trim(res.Header.Get("ETag"), `"`)
	if etag == "" {
		t.Fatalf("la parte %d no devolvió ETag", n)
	}
	return etag
}

func descargar(ctx context.Context, t *testing.T, u string) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	cuerpo, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET firmado: %s\n%.2048s", res.Status, cuerpo)
	}
	return cuerpo
}

func leer(t *testing.T, a objectstore.Almacen, bucket, key string) []byte {
	t.Helper()
	r, err := a.Abrir(context.Background(), bucket, key)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func igual(t *testing.T, que string, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: %d bytes distintos de los %d subidos", que, len(got), len(want))
	}
}

func aleatorio(t *testing.T) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102T150405"), hex.EncodeToString(b))
}
