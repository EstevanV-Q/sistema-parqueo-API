package rateplans

import "time"

// RatePlan representa una tarifa de parqueo.
type RatePlan struct {
	ID               string
	SiteID           string
	Name             string
	VehicleTypeID    *string
	Currency         string
	GraceMinutes     int
	MinimumCents     int64
	FractionMinutes  int
	CentsPerFraction int64
	DailyCapCents    int64
	LostTicketCents  int64
	IsActive         bool
	ValidFrom        time.Time
	ValidUntil       *time.Time
	CreatedAt        time.Time
}
