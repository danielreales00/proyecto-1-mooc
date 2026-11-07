package queue

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
)

// sobrePush es el cuerpo con que la suscripción push de Pub/Sub entrega un
// mensaje. deliveryAttempt solo viene cuando la suscripción tiene topic de
// mensajes muertos.
type sobrePush struct {
	Message struct {
		Data       string            `json:"data"`
		Attributes map[string]string `json:"attributes"`
		MessageID  string            `json:"messageId"`
	} `json:"message"`
	Subscription    string `json:"subscription"`
	DeliveryAttempt int    `json:"deliveryAttempt"`
}

// Push recibe la suscripción push de Pub/Sub (ADR-0020). La respuesta es la
// confirmación: un 2xx confirma el mensaje y cualquier otro código hace que
// Pub/Sub lo entregue de nuevo con backoff.
//
//   - Completado, YaTerminado o Ilegible: 204. El trabajo está persistido, o no
//     hay nada que reintentar.
//   - EnCurso: 409. Otro worker lo tiene; si muere, el latido vence y la
//     siguiente entrega lo reclama.
//   - Fallido: 500. En la última entrega, antes de responder, el trabajo pasa a
//     la DLQ de la base; Pub/Sub manda el mensaje a su topic de muertos.
//
// La autenticación la hace Cloud Run antes de llegar aquí: solo la cuenta de la
// suscripción tiene roles/run.invoker sobre el servicio de workers.
type Push struct {
	runner      *Runner
	manejadores map[string]Handler
	maxEntregas int
	log         *slog.Logger
}

func NewPush(runner *Runner, manejadores map[string]Handler, maxEntregas int, log *slog.Logger) *Push {
	return &Push{runner: runner, manejadores: manejadores, maxEntregas: maxEntregas, log: log}
}

func (p *Push) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var sobre sobrePush
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&sobre); err != nil {
		p.log.Error("sobre push ilegible", "error", err)
		w.WriteHeader(http.StatusNoContent) // reintentar no lo arregla
		return
	}
	cuerpo, err := base64.StdEncoding.DecodeString(sobre.Message.Data)
	tipo := sobre.Message.Attributes["tipo"]
	h, conocido := p.manejadores[tipo]
	if err != nil || !conocido {
		p.log.Error("mensaje sin tipo conocido", "message_id", sobre.Message.MessageID, "tipo", tipo, "error", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	log := p.log.With("message_id", sobre.Message.MessageID, "tipo", tipo, "entrega", sobre.DeliveryAttempt)
	res, err := p.runner.Ejecutar(r.Context(), tipo, h, cuerpo)
	switch res {
	case Completado, YaTerminado, Ilegible:
		w.WriteHeader(http.StatusNoContent)
	case EnCurso:
		w.WriteHeader(http.StatusConflict)
	default:
		if p.ultimaEntrega(sobre.DeliveryAttempt) {
			p.runner.Matar(r.Context(), tipo, cuerpo, sobre.DeliveryAttempt, err)
		}
		log.Warn("entrega fallida; Pub/Sub la reintentará", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (p *Push) ultimaEntrega(intento int) bool {
	return p.maxEntregas > 0 && intento >= p.maxEntregas
}
