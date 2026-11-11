package offline_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"parking-backend/internal/offline"
)

func TestQueue_EnqueuePersistAndAck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.jsonl")

	q, err := offline.OpenQueue(path)
	if err != nil {
		t.Fatalf("OpenQueue: %v", err)
	}

	if err := q.Enqueue(offline.Event{ID: "e1", Type: "entry"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := q.Enqueue(offline.Event{ID: "e2", Type: "exit"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Duplicado por ID: no debe crecer.
	if err := q.Enqueue(offline.Event{ID: "e1", Type: "entry"}); err != nil {
		t.Fatalf("Enqueue duplicado: %v", err)
	}
	if q.Len() != 2 {
		t.Fatalf("Len = %d, quiere 2", q.Len())
	}

	// Reabrir la cola debe recuperar los eventos.
	q2, err := offline.OpenQueue(path)
	if err != nil {
		t.Fatalf("OpenQueue reabrir: %v", err)
	}
	if q2.Len() != 2 {
		t.Fatalf("Len tras reabrir = %d, quiere 2", q2.Len())
	}

	if err := q2.Ack("e1"); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if q2.Len() != 1 {
		t.Fatalf("Len tras Ack = %d, quiere 1", q2.Len())
	}
	pending := q2.Pending()
	if len(pending) != 1 || pending[0].ID != "e2" {
		t.Fatalf("Pending = %+v, quiere solo e2", pending)
	}
}

type fakeRemote struct {
	failIDs map[string]bool
	sent    []string
}

func (f *fakeRemote) Send(_ context.Context, e offline.Event) error {
	if f.failIDs[e.ID] {
		return errors.New("sin red")
	}
	f.sent = append(f.sent, e.ID)
	return nil
}

func TestSyncer_Run(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.jsonl")
	q, _ := offline.OpenQueue(path)
	_ = q.Enqueue(offline.Event{ID: "ok-1", Type: "entry"})
	_ = q.Enqueue(offline.Event{ID: "fail-1", Type: "entry"})
	_ = q.Enqueue(offline.Event{ID: "ok-2", Type: "exit"})

	remote := &fakeRemote{failIDs: map[string]bool{"fail-1": true}}
	syncer := offline.NewSyncer(q, remote, 3, 0)

	res, err := syncer.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Sent != 2 || res.Failed != 1 {
		t.Fatalf("resultado = %+v, quiere Sent=2 Failed=1", res)
	}
	if q.Len() != 1 {
		t.Fatalf("Len = %d, quiere 1 (solo el fallido)", q.Len())
	}

	// Al quitar el fallo, la siguiente pasada lo sincroniza.
	remote.failIDs = map[string]bool{}
	res, err = syncer.Run(context.Background())
	if err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	if res.Sent != 1 || q.Len() != 0 {
		t.Fatalf("segunda pasada = %+v, len=%d", res, q.Len())
	}
}
