package media

import (
	"errors"
	"strings"
	"testing"
)

func TestVariantesParaNoHaceUpscaling(t *testing.T) {
	casos := []struct {
		nombre   string
		alto     int
		esperado []string
	}{
		{"un 1080p da los tres peldaños", 1080, []string{"1080p", "360p", "720p"}},
		{"un 720p no genera 1080p", 720, []string{"360p", "720p"}},
		{"un 360p solo se genera a sí mismo", 360, []string{"360p"}},
		{"una altura intermedia baja al peldaño que cabe", 719, []string{"360p"}},
		{"un original más bajo que el primer peldaño conserva su altura", 240, []string{"240p"}},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			vs := VariantesPara(c.alto, true)
			if got := NombresDeVariante(vs); strings.Join(got, ",") != strings.Join(c.esperado, ",") {
				t.Fatalf("variantes = %v, se esperaba %v", got, c.esperado)
			}
			for _, v := range vs {
				if v.Alto > c.alto {
					t.Fatalf("%s (%dp) es más alta que el original (%dp): eso es upscaling",
						v.Nombre, v.Alto, c.alto)
				}
			}
		})
	}
}

func TestVariantesParaAudio(t *testing.T) {
	vs := VariantesPara(0, false)
	if len(vs) != 1 || vs[0].Nombre != "audio" {
		t.Fatalf("un asset sin imagen debe dar una sola variante de audio, dio %v", vs)
	}
	if vs[0].VideoKbps != 0 {
		t.Fatalf("la variante de audio no debe pedir vídeo")
	}
}

func TestVariantesParaAlturaDesconocida(t *testing.T) {
	if vs := VariantesPara(0, true); vs != nil {
		t.Fatalf("sin altura no se puede decidir la escalera; dio %v", vs)
	}
}

func TestClavesDeterministas(t *testing.T) {
	const id = "018f0000-0000-7000-8000-000000000000"
	if a, b := ClaveMaster(id), ClaveMaster(id); a != b {
		t.Fatal("la clave del master debe ser estable entre llamadas")
	}
	if got := ClavePlaylist(id, "360p"); got != "hls/"+id+"/360p/index.m3u8" {
		t.Fatalf("clave de playlist inesperada: %s", got)
	}
	if !strings.HasPrefix(ClavePlaylist(id, "360p"), PrefijoVariante(id, "360p")) {
		t.Fatal("la playlist debe vivir bajo el prefijo de su variante")
	}
}

func TestMasterM3U8(t *testing.T) {
	m := MasterM3U8(VariantesPara(720, true), func(v Variante) string { return v.Nombre + ".m3u8" })

	if !strings.HasPrefix(m, "#EXTM3U\n") {
		t.Fatal("un manifiesto HLS empieza por #EXTM3U")
	}
	for _, quiero := range []string{"RESOLUTION=640x360", "RESOLUTION=1280x720", "360p.m3u8", "720p.m3u8"} {
		if !strings.Contains(m, quiero) {
			t.Fatalf("el manifiesto no contiene %q:\n%s", quiero, m)
		}
	}
	if strings.Contains(m, "1080") {
		t.Fatalf("el manifiesto anuncia una variante que no se generó:\n%s", m)
	}
}

func TestReescribirSegmentos(t *testing.T) {
	playlist := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-TARGETDURATION:6",
		"#EXTINF:6.000000,",
		"seg-000.ts",
		"#EXTINF:4.000000,",
		"seg-001.ts",
		"#EXT-X-ENDLIST",
		"",
	}, "\n")

	got, err := ReescribirSegmentos(playlist, func(nombre string) (string, error) {
		return "https://almacen.local/" + nombre + "?firma=abc", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "\nseg-000.ts\n") {
		t.Fatal("quedó un segmento sin firmar: el almacén respondería 403")
	}
	for _, quiero := range []string{
		"https://almacen.local/seg-000.ts?firma=abc",
		"https://almacen.local/seg-001.ts?firma=abc",
		"#EXT-X-TARGETDURATION:6",
		"#EXT-X-ENDLIST",
	} {
		if !strings.Contains(got, quiero) {
			t.Fatalf("falta %q en la playlist reescrita:\n%s", quiero, got)
		}
	}
}

func TestReescribirSegmentosPropagaElError(t *testing.T) {
	fallo := errors.New("el almacén no responde")
	_, err := ReescribirSegmentos("#EXTM3U\nseg-000.ts\n", func(string) (string, error) {
		return "", fallo
	})
	if !errors.Is(err, fallo) {
		t.Fatalf("se esperaba el error del firmante, llegó %v", err)
	}
}
