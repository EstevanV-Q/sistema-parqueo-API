package subscriptions

import "time"

// Repository define el contrato de acceso a datos de suscripciones.
type Repository interface {
	Create(s *Subscription) error
	ByID(id string) (*Subscription, error)
	ByVehicle(vehicleID string) (*Subscription, error)
	ActiveByVehicleSite(vehicleID, siteID string, now time.Time) (*Subscription, error)
	ActiveByPlateSite(plateNormalized, siteID string, now time.Time) (*Subscription, error)
	UpdateStatus(id, status string) error
	ListBySite(siteID string, status string, limit, offset int) ([]*Subscription, error)
}
