package syncer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"parking-backend/internal/platform/ids"
)

// Processor consume sync_inbox y consolida los eventos de tickets y pagos en
// las tablas centrales. Es idempotente por id de evento de la bandeja.
type Processor struct {
	db          *sql.DB
	batch       int
	maxAttempts int
}

func NewProcessor(db *sql.DB) *Processor {
	return &Processor{db: db, batch: 50, maxAttempts: 10}
}

type inboxEvent struct {
	ID        string
	EventType string
	Payload   []byte
	CreatedAt time.Time
	Attempts  int
}

func (p *Processor) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := p.RunOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("sync: error procesando sync_inbox", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Processor) RunOnce(ctx context.Context) (int, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, event_type, payload, event_created_at, attempts
		FROM sync_inbox
		WHERE processed_at IS NULL AND attempts < ?
		ORDER BY received_at, id
		LIMIT ?`, p.maxAttempts, p.batch)
	if err != nil {
		return 0, err
	}
	var events []inboxEvent
	for rows.Next() {
		var e inboxEvent
		var idBin []byte
		if err := rows.Scan(&idBin, &e.EventType, &e.Payload, &e.CreatedAt, &e.Attempts); err != nil {
			_ = rows.Close()
			return 0, err
		}
		e.ID = ids.Bin16ToString(idBin)
		events = append(events, e)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	processed := 0
	for _, event := range events {
		if err := p.processOne(ctx, event); err != nil {
			p.markFailure(ctx, event, err)
			continue
		}
		processed++
	}
	return processed, nil
}

func (p *Processor) processOne(ctx context.Context, event inboxEvent) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := p.apply(ctx, tx, event); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE sync_inbox SET processed_at = ?, last_error = NULL WHERE id = ?`,
		time.Now().UTC(), ids.StringToBin16(event.ID)); err != nil {
		return err
	}
	return tx.Commit()
}

func (p *Processor) markFailure(ctx context.Context, event inboxEvent, processErr error) {
	msg := processErr.Error()
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	_, err := p.db.ExecContext(ctx,
		`UPDATE sync_inbox SET attempts = attempts + 1, last_error = ? WHERE id = ?`,
		msg, ids.StringToBin16(event.ID))
	if err != nil {
		slog.Error("sync: no se pudo registrar fallo de sync_inbox", "id", event.ID, "error", err)
	}
}

func (p *Processor) apply(ctx context.Context, tx *sql.Tx, event inboxEvent) error {
	switch event.EventType {
	case "ticket.entered":
		return p.applyTicketEntered(ctx, tx, event)
	case "ticket.settled":
		return p.applyTicketSettled(ctx, tx, event)
	case "ticket.cancelled":
		return p.applyTicketCancelled(ctx, tx, event)
	case "payment.created":
		return p.applyPaymentCreated(ctx, tx, event)
	case "payment.refunded":
		return p.applyPaymentRefunded(ctx, tx, event)
	default:
		// Eventos de catálogo, auditoría o dispositivos aún no se consolidan
		// en tablas centrales; se aceptan y se marcan como procesados.
		slog.Debug("sync: evento sin consolidador central", "event_type", event.EventType)
		return nil
	}
}

type ticketEnteredPayload struct {
	TicketID        string    `json:"ticket_id"`
	SiteID          string    `json:"site_id"`
	ZoneID          *string   `json:"zone_id"`
	SpaceID         *string   `json:"space_id"`
	VehicleID       *string   `json:"vehicle_id"`
	SubscriptionID  *string   `json:"subscription_id"`
	PlateNormalized string    `json:"plate_normalized"`
	PlateRaw        string    `json:"plate_raw"`
	TypeID          string    `json:"type_id"`
	RatePlanID      *string   `json:"rate_plan_id"`
	EntryAt         time.Time `json:"entry_at"`
	Status          string    `json:"status"`
	TotalCents      int64     `json:"total_cents"`
	PaidCents       int64     `json:"paid_cents"`
}

func (p *Processor) applyTicketEntered(ctx context.Context, tx *sql.Tx, event inboxEvent) error {
	var payload ticketEnteredPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("payload ticket.entered: %w", err)
	}
	if payload.TicketID == "" || payload.SiteID == "" || payload.TypeID == "" || payload.PlateNormalized == "" {
		return fmt.Errorf("ticket.entered incompleto")
	}
	entryAt := payload.EntryAt
	if entryAt.IsZero() {
		entryAt = event.CreatedAt
	}
	status := payload.Status
	if status == "" {
		status = "open"
	}
	plateRaw := payload.PlateRaw
	if plateRaw == "" {
		plateRaw = payload.PlateNormalized
	}
	now := time.Now().UTC()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tickets
			(id, site_id, zone_id, space_id, vehicle_id, subscription_id, plate_normalized,
			 plate_raw, type_id, rate_plan_id, entry_at, status, total_cents, paid_cents,
			 is_lost, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, FALSE, ?, ?)
		ON DUPLICATE KEY UPDATE
			status=VALUES(status), total_cents=VALUES(total_cents),
			paid_cents=VALUES(paid_cents), updated_at=VALUES(updated_at)`,
		ids.StringToBin16(payload.TicketID), ids.StringToBin16(payload.SiteID),
		optID(payload.ZoneID), optID(payload.SpaceID), optID(payload.VehicleID),
		optID(payload.SubscriptionID), payload.PlateNormalized, plateRaw,
		ids.StringToBin16(payload.TypeID), optID(payload.RatePlanID), entryAt,
		status, payload.TotalCents, payload.PaidCents, entryAt, now)
	if err != nil {
		return fmt.Errorf("insertar ticket %s: %w", payload.TicketID, err)
	}
	return nil
}

type ticketSettledPayload struct {
	TicketID      string    `json:"ticket_id"`
	PaymentID     string    `json:"payment_id"`
	ExitAt        time.Time `json:"exit_at"`
	TotalCents    int64     `json:"total_cents"`
	PaidCents     int64     `json:"paid_cents"`
	Currency      string    `json:"currency"`
	Method        string    `json:"method"`
	CashierUserID string    `json:"cashier_user_id"`
	Status        string    `json:"status"`
}

func (p *Processor) applyTicketSettled(ctx context.Context, tx *sql.Tx, event inboxEvent) error {
	var payload ticketSettledPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("payload ticket.settled: %w", err)
	}
	if payload.TicketID == "" {
		return fmt.Errorf("ticket.settled sin ticket_id")
	}
	exitAt := payload.ExitAt
	if exitAt.IsZero() {
		exitAt = event.CreatedAt
	}
	status := payload.Status
	if status == "" {
		status = "paid"
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE tickets SET exit_at=?, status=?, total_cents=?, paid_cents=?, updated_at=?
		WHERE id=?`,
		exitAt, status, payload.TotalCents, payload.PaidCents, time.Now().UTC(),
		ids.StringToBin16(payload.TicketID))
	if err != nil {
		return fmt.Errorf("actualizar ticket %s: %w", payload.TicketID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("ticket %s no existe en el central", payload.TicketID)
	}
	if payload.PaymentID != "" {
		currency := payload.Currency
		if currency == "" {
			currency = "COP"
		}
		method := payload.Method
		if method == "" {
			method = "cash"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO payments (id, ticket_id, amount_cents, currency, method, status,
				cashier_user_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'completed', ?, ?, ?)
			ON DUPLICATE KEY UPDATE status=VALUES(status), amount_cents=VALUES(amount_cents)`,
			ids.StringToBin16(payload.PaymentID), ids.StringToBin16(payload.TicketID),
			payload.PaidCents, currency, method, optStringID(payload.CashierUserID),
			exitAt, time.Now().UTC()); err != nil {
			return fmt.Errorf("insertar pago %s: %w", payload.PaymentID, err)
		}
	}
	return nil
}

type ticketCancelledPayload struct {
	TicketID string    `json:"ticket_id"`
	ExitAt   time.Time `json:"exit_at"`
}

func (p *Processor) applyTicketCancelled(ctx context.Context, tx *sql.Tx, event inboxEvent) error {
	var payload ticketCancelledPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("payload ticket.cancelled: %w", err)
	}
	if payload.TicketID == "" {
		return fmt.Errorf("ticket.cancelled sin ticket_id")
	}
	exitAt := payload.ExitAt
	if exitAt.IsZero() {
		exitAt = event.CreatedAt
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE tickets SET status='cancelled', exit_at=?, updated_at=? WHERE id=?`,
		exitAt, time.Now().UTC(), ids.StringToBin16(payload.TicketID))
	if err != nil {
		return fmt.Errorf("cancelar ticket %s: %w", payload.TicketID, err)
	}
	return nil
}

type paymentCreatedPayload struct {
	PaymentID      string  `json:"payment_id"`
	TicketID       *string `json:"ticket_id"`
	SubscriptionID *string `json:"subscription_id"`
	AmountCents    int64   `json:"amount_cents"`
	Currency       string  `json:"currency"`
	Method         string  `json:"method"`
	Status         string  `json:"status"`
	Reference      *string `json:"reference"`
	CashierUserID  *string `json:"cashier_user_id"`
}

func (p *Processor) applyPaymentCreated(ctx context.Context, tx *sql.Tx, event inboxEvent) error {
	var payload paymentCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("payload payment.created: %w", err)
	}
	if payload.PaymentID == "" {
		return fmt.Errorf("payment.created sin payment_id")
	}
	currency := payload.Currency
	if currency == "" {
		currency = "COP"
	}
	method := payload.Method
	if method == "" {
		method = "cash"
	}
	status := payload.Status
	if status == "" {
		status = "completed"
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO payments (id, ticket_id, subscription_id, amount_cents, currency,
			method, status, reference, cashier_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE status=VALUES(status), amount_cents=VALUES(amount_cents)`,
		ids.StringToBin16(payload.PaymentID), optID(payload.TicketID), optID(payload.SubscriptionID),
		payload.AmountCents, currency, method, status, optString(payload.Reference),
		optID(payload.CashierUserID), event.CreatedAt, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("insertar pago %s: %w", payload.PaymentID, err)
	}
	return nil
}

type paymentRefundedPayload struct {
	PaymentID string `json:"payment_id"`
}

func (p *Processor) applyPaymentRefunded(ctx context.Context, tx *sql.Tx, event inboxEvent) error {
	var payload paymentRefundedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("payload payment.refunded: %w", err)
	}
	if payload.PaymentID == "" {
		return fmt.Errorf("payment.refunded sin payment_id")
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE payments SET status='refunded', updated_at=? WHERE id=?`,
		time.Now().UTC(), ids.StringToBin16(payload.PaymentID))
	if err != nil {
		return fmt.Errorf("reembolsar pago %s: %w", payload.PaymentID, err)
	}
	return nil
}

func optID(v *string) []byte {
	if v == nil || *v == "" {
		return nil
	}
	return ids.StringToBin16(*v)
}

func optStringID(v string) []byte {
	if v == "" {
		return nil
	}
	return ids.StringToBin16(v)
}

func optString(v *string) any {
	if v == nil || *v == "" {
		return nil
	}
	return *v
}
