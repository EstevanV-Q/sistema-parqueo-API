package offline

import (
	"context"
	"time"
)

// Remote envía un evento a la API. Debe ser idempotente por Event.ID.
type Remote interface {
	Send(ctx context.Context, e Event) error
}

// SyncResult resume una pasada de sincronización.
type SyncResult struct {
	Sent   int
	Failed int
}

// Syncer vacía la cola contra un Remote.
type Syncer struct {
	queue       *Queue
	remote      Remote
	maxAttempts int
	backoff     time.Duration
}

// NewSyncer crea un sincronizador.
func NewSyncer(queue *Queue, remote Remote, maxAttempts int, backoff time.Duration) *Syncer {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if backoff <= 0 {
		backoff = 2 * time.Second
	}
	return &Syncer{queue: queue, remote: remote, maxAttempts: maxAttempts, backoff: backoff}
}

// Run procesa la cola una vez. Los eventos que fallan se reintentan en la
// siguiente pasada; los que superan maxAttempts se dejan para revisión manual.
func (s *Syncer) Run(ctx context.Context) (SyncResult, error) {
	var result SyncResult

	for _, e := range s.queue.Pending() {
		if e.Attempts >= s.maxAttempts {
			continue
		}
		if err := s.remote.Send(ctx, e); err != nil {
			result.Failed++
			_ = s.queue.Fail(e.ID)
			continue
		}
		result.Sent++
		if err := s.queue.Ack(e.ID); err != nil {
			return result, err
		}
	}
	return result, nil
}

// RunWithRetry ejecuta Run hasta vaciar la cola o agotar los reintentos.
func (s *Syncer) RunWithRetry(ctx context.Context) (SyncResult, error) {
	var total SyncResult
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		res, err := s.Run(ctx)
		total.Sent += res.Sent
		total.Failed = res.Failed
		if err != nil {
			return total, err
		}
		if s.queue.Len() == 0 {
			return total, nil
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(s.backoff):
		}
	}
	return total, nil
}
