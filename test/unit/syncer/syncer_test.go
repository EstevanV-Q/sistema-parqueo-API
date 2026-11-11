package syncer_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	platformdb "parking-backend/internal/platform/db"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/outbox"
	syncer "parking-backend/internal/sync"
)

type flakySender struct {
	failFirst bool
	calls     int
	events    []string
}

func (s *flakySender) Send(_ context.Context, event outbox.Event) error {
	s.calls++
	s.events = append(s.events, event.ID)
	if s.failFirst && s.calls == 1 {
		return errors.New("central no disponible")
	}
	return nil
}

func TestWorkerRetriesAndMarksSent(t *testing.T) {
	database, err := platformdb.OpenSQLite(filepath.Join(t.TempDir(), "worker.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := outbox.NewStore(database)
	eventID := ids.UUIDv7String()
	if err := store.Enqueue("ticket", eventID, "ticket.entered", map[string]any{"ticket_id": eventID}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	sender := &flakySender{failFirst: true}
	worker := syncer.NewWorker(store, sender, 3, time.Millisecond)

	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce fallido: %v", err)
	}
	if result.Failed != 1 || result.Sent != 0 {
		t.Fatalf("resultado = %+v, quiere 1 fallo", result)
	}
	pending, err := store.Pending(10)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("el evento debe esperar backoff, pendientes = %d", len(pending))
	}

	time.Sleep(10 * time.Millisecond)

	result, err = worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("segundo RunOnce: %v", err)
	}
	if result.Sent != 1 || result.Failed != 0 {
		t.Fatalf("resultado = %+v, quiere 1 enviado", result)
	}
	count, err := store.CountPending()
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if count != 0 {
		t.Fatalf("pendientes = %d, quiere 0", count)
	}
	if sender.calls != 2 || len(sender.events) != 2 || sender.events[0] != sender.events[1] {
		t.Fatalf("el reintento debe conservar el mismo ID: calls=%d events=%v", sender.calls, sender.events)
	}
}

func TestWorkerGivesUpAfterMaxAttempts(t *testing.T) {
	database, err := platformdb.OpenSQLite(filepath.Join(t.TempDir(), "worker-fail.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := outbox.NewStore(database)
	eventID := ids.UUIDv7String()
	if err := store.Enqueue("ticket", eventID, "ticket.entered", map[string]any{"ticket_id": eventID}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	sender := &alwaysFailSender{}
	worker := syncer.NewWorker(store, sender, 1, time.Millisecond)
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("calls = %d, quiere 1", sender.calls)
	}

	var status string
	if err := database.QueryRow(`SELECT status FROM outbox WHERE aggregate_id = ?`, ids.StringToBin16(eventID)).Scan(&status); err != nil {
		t.Fatalf("leer outbox: %v", err)
	}
	if status != "failed" {
		t.Fatalf("status = %s, quiere failed", status)
	}
	count, err := store.CountPending()
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if count != 0 {
		t.Fatalf("pendientes = %d, quiere 0", count)
	}
}

type alwaysFailSender struct {
	calls int
}

func (s *alwaysFailSender) Send(context.Context, outbox.Event) error {
	s.calls++
	return errors.New("error permanente")
}
