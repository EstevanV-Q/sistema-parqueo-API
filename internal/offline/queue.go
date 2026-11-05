package offline

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"parking-backend/internal/platform/ids"
)

// Queue es una cola FIFO persistida como JSON Lines.
type Queue struct {
	mu     sync.Mutex
	path   string
	events map[string]*Event
	order  []string
}

// OpenQueue abre (o crea) la cola en la ruta indicada.
func OpenQueue(path string) (*Queue, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	q := &Queue{
		path:   path,
		events: make(map[string]*Event),
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return q, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			// Una línea corrupta no debe impedir operar: se ignora.
			continue
		}
		q.events[e.ID] = &e
		q.order = append(q.order, e.ID)
	}
	return q, scanner.Err()
}

// Enqueue agrega un evento al final de la cola.
func (q *Queue) Enqueue(e Event) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if e.ID == "" {
		e.ID = ids.UUIDv7String()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if _, exists := q.events[e.ID]; exists {
		return nil // idempotente por ID
	}

	q.events[e.ID] = &e
	q.order = append(q.order, e.ID)
	return q.persist()
}

// Pending devuelve una copia de los eventos pendientes en orden FIFO.
func (q *Queue) Pending() []Event {
	q.mu.Lock()
	defer q.mu.Unlock()

	out := make([]Event, 0, len(q.order))
	for _, id := range q.order {
		if e, ok := q.events[id]; ok {
			out = append(out, *e)
		}
	}
	return out
}

// Ack elimina un evento sincronizado de la cola.
func (q *Queue) Ack(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, ok := q.events[id]; !ok {
		return nil
	}
	delete(q.events, id)
	q.removeFromOrder(id)
	return q.persist()
}

// Fail incrementa el contador de intentos de un evento.
func (q *Queue) Fail(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	e, ok := q.events[id]
	if !ok {
		return nil
	}
	e.Attempts++
	return q.persist()
}

// Len devuelve cuántos eventos hay pendientes.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.order)
}

func (q *Queue) removeFromOrder(id string) {
	for i, v := range q.order {
		if v == id {
			q.order = append(q.order[:i], q.order[i+1:]...)
			return
		}
	}
}

func (q *Queue) persist() error {
	tmp := q.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)
	for _, id := range q.order {
		e, ok := q.events[id]
		if !ok {
			continue
		}
		data, err := json.Marshal(e)
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, q.path)
}
