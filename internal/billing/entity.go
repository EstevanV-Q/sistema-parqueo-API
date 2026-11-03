package billing

import "time"

// Payment representa un pago realizado.
type Payment struct {
	ID              string
	TicketID        *string
	SubscriptionID  *string
	AmountCents     int64
	Currency        string
	Method          string
	Status          string
	Reference       *string
	CashierUserID   *string
	IdempotencyKey  *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
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
	MethodCash      = "cash"
	MethodCard      = "card"
	MethodTransfer  = "transfer"
	MethodAbonnement = "abonnement"
	MethodWaived    = "waived"
)

// CajaDiaria representa el resumen de caja de un día.
type CajaDiaria struct {
	SiteID         string
	Date           string
	TotalCents     int64
	Count          int
	ByMethod       map[string]int64
}
