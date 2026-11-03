package vehicles

import "time"

// Vehicle representa un vehículo registrado.
type Vehicle struct {
	ID              string
	PlateNormalized string
	PlateRaw        string
	TypeID          string
	CustomerID      *string
	IsActive        bool
	CreatedAt       time.Time
}
