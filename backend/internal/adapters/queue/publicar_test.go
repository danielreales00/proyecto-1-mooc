package queue

import (
	"context"
	"os"
	"testing"

	"github.com/hibiken/asynq"

	"mooc/backend/internal/platform/jobs"
)

// Contra un Redis real (make prueba-cola). Usa la base 15 y una cola propia
// para no tocar las del stack.
func TestPublicarNoDuplicaYReactivaLoArchivado(t *testing.T) {
	addr := os.Getenv("REDIS_PRUEBA_ADDR")
	if addr == "" {
		t.Skip("REDIS_PRUEBA_ADDR no definida: prueba de integración (make prueba-cola)")
	}
	opt := OpcionesRedis(addr, "", 15, false)
	p := &Publisher{client: asynq.NewClient(opt), inspector: asynq.NewInspector(opt)}
	defer p.Close()
	const cola = "prueba-publicar"
	defer func() { _ = p.inspector.DeleteQueue(cola, true) }()

	ctx := context.Background()
	j := jobs.Job{Key: "media.transcode:prueba", Type: "prueba", Queue: cola}

	if nueva, err := p.Republicar(ctx, j, ""); err != nil || !nueva {
		t.Fatalf("primera publicación: nueva=%v err=%v", nueva, err)
	}
	// Lo que hacía el reaper con un trabajo que esperaba turno (DT-01).
	for range 3 {
		if nueva, err := p.Republicar(ctx, j, ""); err != nil || nueva {
			t.Fatalf("republicación: nueva=%v err=%v, no debía crear otra tarea", nueva, err)
		}
	}
	if n := pendientes(t, p, cola); n != 1 {
		t.Fatalf("%d tareas pendientes, se esperaba 1", n)
	}

	// Una tarea en la DLQ conserva su TaskID: el reencolado la reactiva.
	if err := p.inspector.ArchiveTask(cola, j.Key); err != nil {
		t.Fatal(err)
	}
	if nueva, err := p.Republicar(ctx, j, ""); err != nil || !nueva {
		t.Fatalf("reencolar lo archivado: nueva=%v err=%v", nueva, err)
	}
	info, err := p.inspector.GetTaskInfo(cola, j.Key)
	if err != nil {
		t.Fatal(err)
	}
	if info.State != asynq.TaskStatePending {
		t.Fatalf("estado tras reencolar: %v, se esperaba pending", info.State)
	}
}

func pendientes(t *testing.T, p *Publisher, cola string) int {
	t.Helper()
	q, err := p.inspector.GetQueueInfo(cola)
	if err != nil {
		t.Fatal(err)
	}
	return q.Pending
}
