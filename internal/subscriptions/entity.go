package subscriptions

import "time"

// Subscription representa una suscripción/abono de un vehículo.
type Subscription struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	VehicleID  string    `json:"vehicle_id"`
	SiteID     string    `json:"site_id"`
	PlanName   string    `json:"plan_name"`
	PriceCents int64     `json:"price_cents"`
	StartsAt   time.Time `json:"starts_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Estados de suscripción.
const (
	SubActive    = "active"
	SubExpired   = "expired"
	SubCancelled = "cancelled"
)

// IsActive verifica si la suscripción está vigente en el momento dado.
func (s *Subscription) IsActiveAt(now time.Time) bool {
	return s.Status == SubActive && !now.Before(s.StartsAt) && !now.After(s.ExpiresAt)
}
