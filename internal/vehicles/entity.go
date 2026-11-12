package vehicles

import "time"

// Vehicle representa un vehículo registrado.
type Vehicle struct {
	ID              string    `json:"id"`
	PlateNormalized string    `json:"plate_normalized"`
	PlateRaw        string    `json:"plate_raw"`
	TypeID          string    `json:"type_id"`
	CustomerID      *string   `json:"customer_id,omitempty"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}
