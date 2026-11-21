package queue

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"mooc.local/backend/internal/platform/jobs"
)

// Evento atiende un trabajo que Eventarc entrega a una Cloud Run function como
// un CloudEvent de Pub/Sub (ADR-0021). Es el equivalente de Push para la
// función: el mismo Runner, otra forma de confirmar.
//
// Eventarc confirma el evento cuando Atender devuelve nil y lo reintenta, con
// espera exponencial, cuando devuelve un error:
//
//   - Completado o YaTerminado: nil. Una copia de un trabajo terminado no se
//     repite.
//   - EnCurso: error. Si el que lo tiene muere, su latido vence y una entrega
//     siguiente lo reclama.
//   - Fallido: error, salvo que el trabajo ya haya agotado sus intentos.
//     Entonces queda 'dead' y se devuelve nil para cortar los reintentos.
//   - Ilegible o de otro tipo: nil. Reintentarlo no lo arregla.
type Evento struct {
	runner      *Runner
	tipo        string
	h           Handler
	maxIntentos int
	log         *slog.Logger
}

// ErrEnCurso hace que Eventarc reintente la entrega de un trabajo que otro
// tiene reclamado.
var ErrEnCurso = errors.New("el trabajo está en curso en otra entrega")

// NewEvento atiende un solo tipo de trabajo, el del topic de la función.
// maxIntentos es el límite de reclamos antes de mandarlo a 'dead'. Con 0 no
// hay límite.
func NewEvento(runner *Runner, tipo string, h Handler, maxIntentos int, log *slog.Logger) *Evento {
	return &Evento{runner: runner, tipo: tipo, h: h, maxIntentos: maxIntentos, log: log}
}

// mensajePublicado es el cuerpo del CloudEvent
// google.cloud.pubsub.topic.v1.messagePublished. encoding/json decodifica el
// base64 de `data` al llenar el []byte.
type mensajePublicado struct {
	Message struct {
		Data       []byte            `json:"data"`
		Attributes map[string]string `json:"attributes"`
		MessageID  string            `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// Atender recibe los datos del CloudEvent.
func (e *Evento) Atender(ctx context.Context, datos []byte) error {
	var m mensajePublicado
	if err := json.Unmarshal(datos, &m); err != nil {
		e.log.Error("evento ilegible; se confirma sin ejecutar", "error", err)
		return nil
	}
	tipo := m.Message.Attributes["tipo"]
	log := e.log.With("message_id", m.Message.MessageID, "tipo", tipo)
	if tipo != e.tipo {
		log.Error("evento de otro tipo; se confirma sin ejecutar", "esperado", e.tipo)
		return nil
	}

	res, err := e.runner.Ejecutar(ctx, tipo, e.h, m.Message.Data)
	switch res {
	case Completado, YaTerminado, Ilegible:
		return nil
	case EnCurso:
		return ErrEnCurso
	}

	var p Payload
	_ = json.Unmarshal(m.Message.Data, &p)
	intentos, ierr := jobs.Intentos(ctx, e.runner.db, p.JobKey)
	if ierr == nil && e.agotado(intentos) {
		e.runner.Matar(ctx, tipo, m.Message.Data, intentos, err)
		return nil
	}
	log.Warn("entrega fallida; Eventarc la reintentará", "job_key", p.JobKey,
		"intentos", intentos, "error", err)
	return err
}

// agotado dice si un trabajo ya usó todos sus intentos.
func (e *Evento) agotado(intentos int) bool {
	return e.maxIntentos > 0 && intentos >= e.maxIntentos
}
