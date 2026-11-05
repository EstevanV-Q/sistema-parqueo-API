package subscriptions

import "time"

// Subscription representa una suscripción/abono de un vehículo.
type Subscription struct {
	ID         string
	CustomerID string
	VehicleID  string
	SiteID     string
	PlanName   string
	PriceCents int64
	StartsAt   time.Time
	ExpiresAt  time.Time
	Status     string
	CreatedAt  time.Time
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
