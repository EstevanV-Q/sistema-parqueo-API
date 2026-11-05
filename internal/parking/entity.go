package parking

import "time"

// Ticket representa un registro de entrada/salida de un vehículo.
type Ticket struct {
	ID              string
	SiteID          string
	ZoneID          *string
	SpaceID         *string
	VehicleID       *string
	SubscriptionID  *string
	PlateNormalized string
	PlateRaw        string
	TypeID          string
	RatePlanID      *string
	EntryAt         time.Time
	ExitAt          *time.Time
	Status          string
	TotalCents      int64
	PaidCents       int64
	IsLost          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Estados válidos de un ticket.
const (
	TicketOpen      = "open"
	TicketPaid      = "paid"
	TicketClosed    = "closed"
	TicketCancelled = "cancelled"
	TicketLost      = "lost"
)

// Transiciones válidas: estado actual -> estados permitidos.
var ValidTransitions = map[string][]string{
	TicketOpen:      {TicketPaid, TicketClosed, TicketCancelled, TicketLost},
	TicketPaid:      {TicketClosed},
	TicketClosed:    {},
	TicketCancelled: {},
	TicketLost:      {},
}

// CanTransition verifica si se puede pasar de from a to.
func CanTransition(from, to string) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}

// IsOpen devuelve true si el ticket está abierto.
func (t *Ticket) IsOpen() bool {
	return t.Status == TicketOpen
}

// IsClosed devuelve true si el ticket está cerrado o pagado.
func (t *Ticket) IsClosed() bool {
	return t.Status == TicketClosed || t.Status == TicketPaid || t.Status == TicketCancelled || t.Status == TicketLost
}
