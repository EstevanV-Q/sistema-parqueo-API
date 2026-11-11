// Package syncer drena la tabla outbox del borde hacia la API central.
package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"parking-backend/internal/platform/outbox"
)

// Sender envía un evento al central. Debe ser idempotente por Event.ID.
type Sender interface {
	Send(ctx context.Context, event outbox.Event) error
}

// Result resume una pasada de sincronización.
type Result struct {
	Sent    int
	Failed  int
	Pending int
}

// Worker procesa la outbox en orden FIFO y respeta el backoff por evento.
type Worker struct {
	store       *outbox.Store
	sender      Sender
	maxAttempts int
	baseBackoff time.Duration
	batch       int
}

func NewWorker(store *outbox.Store, sender Sender, maxAttempts int, baseBackoff time.Duration) *Worker {
	if maxAttempts <= 0 {
		maxAttempts = 8
	}
	if baseBackoff <= 0 {
		baseBackoff = 2 * time.Second
	}
	return &Worker{
		store:       store,
		sender:      sender,
		maxAttempts: maxAttempts,
		baseBackoff: baseBackoff,
		batch:       50,
	}
}

// RunOnce procesa hasta batch eventos pendientes.
func (w *Worker) RunOnce(ctx context.Context) (Result, error) {
	events, err := w.store.Pending(w.batch)
	if err != nil {
		return Result{}, err
	}

	var result Result
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := w.sender.Send(ctx, event); err != nil {
			result.Failed++
			attempts := event.Attempts + 1
			if attempts >= w.maxAttempts {
				_ = w.store.MarkFailed(event.ID, err)
				continue
			}
			_ = w.store.MarkRetry(event.ID, err, time.Now().UTC().Add(w.backoff(attempts)))
			continue
		}
		if err := w.store.MarkSent(event.ID); err != nil {
			return result, err
		}
		result.Sent++
	}
	result.Pending, _ = w.store.CountPending()
	return result, nil
}

// Run ejecuta pasadas periódicas hasta que el contexto se cancele.
func (w *Worker) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if _, err := w.RunOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) backoff(attempt int) time.Duration {
	delay := w.baseBackoff
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > 5*time.Minute {
			return 5 * time.Minute
		}
	}
	return delay
}

// HTTPSender publica eventos en el endpoint central de sincronización.
type HTTPSender struct {
	URL    string
	Token  string
	SiteID string
	Client *http.Client
}

func (s *HTTPSender) Send(ctx context.Context, event outbox.Event) error {
	if s.URL == "" {
		return fmt.Errorf("SYNC_URL no configurado")
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	body, err := json.Marshal(map[string]any{
		"id":             event.ID,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"event_type":     event.EventType,
		"payload":        json.RawMessage(event.Payload),
		"created_at":     event.CreatedAt,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.SiteID != "" {
		req.Header.Set("X-Site-ID", s.SiteID)
	}
	if s.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("central respondió %d: %s", resp.StatusCode, string(raw))
}
