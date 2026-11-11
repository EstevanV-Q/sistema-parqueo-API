package parking

import (
	"time"
)

// Repository define el contrato de acceso a datos de tickets.
type Repository interface {
	Create(t *Ticket) error
	ByID(id string) (*Ticket, error)
	OpenBySpace(spaceID string) (*Ticket, error)
	OpenByPlateSite(plateNormalized, siteID string) (*Ticket, error)
	LatestByPlateSite(plateNormalized, siteID string) (*Ticket, error)
	Update(t *Ticket) error
	Close(id string, exitAt time.Time, totalCents, paidCents int64, status string) error
	SettleTicketTx(ticketID string, exitAt time.Time, totalCents, paidCents int64, method string, cashierUserID string) error
	CancelTicketTx(ticketID string, exitAt time.Time, actorUserID string) error
	Events(ticketID string) ([]TicketEvent, error)
	CreateEvent(e *TicketEvent) error
	ListBySite(siteID string, status string, limit, offset int) ([]*Ticket, error)
	CountBySite(siteID string, status string) (int, error)
}

// RatePlanReader lee la tarifa vigente desde el módulo de tarifas.
type RatePlanReader interface {
	GetRatePlan(id string) (*RatePlan, error)
	DefaultRatePlan(siteID, typeID string) (*RatePlan, error)
}

// RatePlan representa una tarifa para cálculo de precios.
type RatePlan struct {
	ID               string
	GraceMinutes     int
	MinimumCents     int64
	FractionMinutes  int
	CentsPerFraction int64
	DailyCapCents    int64
	LostTicketCents  int64
}

// SpaceStatusUpdater cambia el estado de un espacio.
type SpaceStatusUpdater interface {
	UpdateStatus(spaceID, status string) error
}

// VehicleTypeResolver resuelve el ID de un tipo de vehículo activo por código.
type VehicleTypeResolver interface {
	VehicleTypeIDByCode(code string) (string, error)
}

// SubscriptionChecker valida si un vehículo tiene abono vigente.
type SubscriptionChecker interface {
	Validate(vehicleID, siteID string, now time.Time) (*Subscription, error)
	ValidateByPlate(plateNormalized, siteID string, now time.Time) (*Subscription, error)
}

// Subscription representa un abono para validación.
type Subscription struct {
	ID        string
	VehicleID string
	SiteID    string
	PlanName  string
	ExpiresAt time.Time
	Status    string
}

// EventPublisher publica cambios de ocupación en tiempo real.
type EventPublisher interface {
	Publish(siteID, event string, payload map[string]any)
}

// TicketEvent representa un cambio de estado en un ticket.
type TicketEvent struct {
	ID          string
	TicketID    string
	EventType   string
	ActorUserID *string
	Details     string
	CreatedAt   time.Time
}
