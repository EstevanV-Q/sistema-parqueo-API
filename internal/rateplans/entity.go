package rateplans

import "time"

// RatePlan representa una tarifa de parqueo.
type RatePlan struct {
	ID               string     `json:"id"`
	SiteID           string     `json:"site_id"`
	Name             string     `json:"name"`
	VehicleTypeID    *string    `json:"vehicle_type_id,omitempty"`
	Currency         string     `json:"currency"`
	GraceMinutes     int        `json:"grace_minutes"`
	MinimumCents     int64      `json:"minimum_cents"`
	FractionMinutes  int        `json:"fraction_minutes"`
	CentsPerFraction int64      `json:"cents_per_fraction"`
	DailyCapCents    int64      `json:"daily_cap_cents"`
	LostTicketCents  int64      `json:"lost_ticket_cents"`
	IsActive         bool       `json:"is_active"`
	ValidFrom        time.Time  `json:"valid_from"`
	ValidUntil       *time.Time `json:"valid_until,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}
