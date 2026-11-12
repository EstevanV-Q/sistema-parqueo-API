package billing

import "time"

// Payment representa un pago realizado.
type Payment struct {
	ID             string    `json:"id"`
	TicketID       *string   `json:"ticket_id,omitempty"`
	SubscriptionID *string   `json:"subscription_id,omitempty"`
	AmountCents    int64     `json:"amount_cents"`
	Currency       string    `json:"currency"`
	Method         string    `json:"method"`
	Status         string    `json:"status"`
	Reference      *string   `json:"reference,omitempty"`
	CashierUserID  *string   `json:"cashier_user_id,omitempty"`
	IdempotencyKey *string   `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Estados de pago.
const (
	PaymentPending   = "pending"
	PaymentCompleted = "completed"
	PaymentFailed    = "failed"
	PaymentRefunded  = "refunded"
)

// Métodos de pago.
const (
	MethodCash       = "cash"
	MethodCard       = "card"
	MethodTransfer   = "transfer"
	MethodAbonnement = "abonnement"
	MethodWaived     = "waived"
)

// CajaDiaria representa el resumen de caja de un día.
type CajaDiaria struct {
	SiteID     string           `json:"site_id"`
	Date       string           `json:"date"`
	TotalCents int64            `json:"total_cents"`
	Count      int              `json:"count"`
	ByMethod   map[string]int64 `json:"by_method"`
}
