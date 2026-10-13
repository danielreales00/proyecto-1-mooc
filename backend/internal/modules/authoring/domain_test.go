package authoring

import "testing"

func TestDisponibilidadDespuesDeEscaneo(t *testing.T) {
	for _, c := range []struct{ kind, assetStatus, want string }{
		{TypeImage, "clean", "ready"}, {TypePDF, "clean", "ready"}, {TypeDownload, "clean", "ready"},
		{TypeSlides, "clean", "ready"}, {TypeVideo, "clean", "processing"}, {TypeAudio, "clean", "processing"},
		{TypeImage, "infected", "failed"}, {TypePDF, "scanning", "processing"}, {TypeVideo, "ready", "ready"},
	} {
		t.Run(c.kind+"/"+c.assetStatus, func(t *testing.T) {
			r := Resource{Type: c.kind, AssetStatus: c.assetStatus}
			if got := r.ProcessingStatus(); got != c.want {
				t.Fatalf("estado = %s, se esperaba %s", got, c.want)
			}
		})
	}
}
