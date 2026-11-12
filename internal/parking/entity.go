package parking

import "time"

// Ticket representa un registro de entrada/salida de un vehículo.
type Ticket struct {
	ID              string     `json:"id"`
	SiteID          string     `json:"site_id"`
	ZoneID          *string    `json:"zone_id,omitempty"`
	SpaceID         *string    `json:"space_id,omitempty"`
	VehicleID       *string    `json:"vehicle_id,omitempty"`
	SubscriptionID  *string    `json:"subscription_id,omitempty"`
	PlateNormalized string     `json:"plate_normalized"`
	PlateRaw        string     `json:"plate_raw"`
	TypeID          string     `json:"type_id"`
	RatePlanID      *string    `json:"rate_plan_id,omitempty"`
	EntryAt         time.Time  `json:"entry_at"`
	ExitAt          *time.Time `json:"exit_at,omitempty"`
	Status          string     `json:"status"`
	TotalCents      int64      `json:"total_cents"`
	PaidCents       int64      `json:"paid_cents"`
	IsLost          bool       `json:"is_lost"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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
