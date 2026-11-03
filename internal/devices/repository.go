package devices

import "time"

// Repository define el contrato de acceso a datos de dispositivos.
type Repository interface {
	Create(d *Device) error
	ByID(id string) (*Device, error)
	ByAPIKey(apiKeyHash string) (*Device, error)
	UpdateHeartbeat(id string, at time.Time) error
	CreateEvent(e *DeviceEvent) error
	EventByID(eventID string) (*DeviceEvent, error)
	ListBySite(siteID string) ([]*Device, error)
}
