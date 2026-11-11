// Package outbox implementa el patrón transactional outbox: la operación de
// negocio y el evento que se enviará al central se confirman en la misma
// transacción local. Si el proceso se cae, el evento sigue pendiente.
package outbox

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"parking-backend/internal/platform/ids"
)

// Event es un evento local pendiente de sincronización.
type Event struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       json.RawMessage
	CreatedAt     time.Time
	Attempts      int
	NextAttemptAt *time.Time
	Status        string
	LastError     *string
}

// Store envuelve la tabla outbox de la base local.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// EnqueueTx inserta un evento usando una transacción existente.
func (s *Store) EnqueueTx(tx *sql.Tx, aggregateType, aggregateID, eventType string, payload any) error {
	if s == nil || tx == nil {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serializar evento de outbox: %w", err)
	}
	_, err = tx.Exec(`INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		ids.UUIDv7(), aggregateType, ids.StringToBin16(aggregateID), eventType, string(body), time.Now().UTC())
	return err
}

// Enqueue inserta un evento fuera de una transacción explícita. Se usa solo
// para operaciones que ya fueron confirmadas y no tienen una transacción común.
func (s *Store) Enqueue(aggregateType, aggregateID, eventType string, payload any) error {
	if s == nil {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serializar evento de outbox: %w", err)
	}
	_, err = s.db.Exec(`INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		ids.UUIDv7(), aggregateType, ids.StringToBin16(aggregateID), eventType, string(body), time.Now().UTC())
	return err
}

// Pending devuelve eventos listos para enviar, respetando el backoff.
func (s *Store) Pending(limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now().UTC()
	rows, err := s.db.Query(`
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at,
		       attempts, next_attempt_at, status, last_error
		FROM outbox
		WHERE status = 'pending' AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
		ORDER BY created_at, id
		LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []Event
	for rows.Next() {
		var e Event
		var idBin, aggregateBin []byte
		var next sql.NullTime
		var lastError sql.NullString
		var payload string
		if err := rows.Scan(&idBin, &e.AggregateType, &aggregateBin, &e.EventType, &payload, &e.CreatedAt, &e.Attempts, &next, &e.Status, &lastError); err != nil {
			return nil, err
		}
		e.ID = ids.Bin16ToString(idBin)
		e.AggregateID = ids.Bin16ToString(aggregateBin)
		e.Payload = json.RawMessage(payload)
		if next.Valid {
			e.NextAttemptAt = &next.Time
		}
		if lastError.Valid {
			e.LastError = &lastError.String
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// CountPending cuenta los eventos aún no confirmados.
func (s *Store) CountPending() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE status = 'pending'`).Scan(&n)
	return n, err
}

// MarkSent confirma que el central aceptó el evento.
func (s *Store) MarkSent(id string) error {
	_, err := s.db.Exec(`UPDATE outbox SET status='sent', last_error=NULL WHERE id = ?`, ids.StringToBin16(id))
	return err
}

// MarkRetry registra un fallo transitorio y programa el siguiente intento.
func (s *Store) MarkRetry(id string, cause error, next time.Time) error {
	if cause == nil {
		cause = errors.New("error de sincronización")
	}
	_, err := s.db.Exec(`
		UPDATE outbox
		SET attempts = attempts + 1, next_attempt_at = ?, last_error = ?
		WHERE id = ?`, next, cause.Error(), ids.StringToBin16(id))
	return err
}

// MarkFailed deja el evento para revisión manual tras superar los reintentos.
func (s *Store) MarkFailed(id string, cause error) error {
	if cause == nil {
		cause = errors.New("error de sincronización")
	}
	_, err := s.db.Exec(`UPDATE outbox SET status='failed', attempts = attempts + 1, last_error = ? WHERE id = ?`,
		cause.Error(), ids.StringToBin16(id))
	return err
}
