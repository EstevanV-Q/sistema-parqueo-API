package subscriptions

import (
	"time"

	"parking-backend/internal/parking"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
)

// Service contiene la lógica de negocio de suscripciones.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create crea una nueva suscripción.
func (s *Service) Create(customerID, vehicleID, siteID, planName string, priceCents int64, startsAt, expiresAt time.Time) (*Subscription, error) {
	if customerID == "" || vehicleID == "" || siteID == "" || planName == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "cliente, vehículo, sede y plan son requeridos", 400, nil)
	}
	sub := &Subscription{
		ID:         ids.UUIDv7String(),
		CustomerID: customerID,
		VehicleID:  vehicleID,
		SiteID:     siteID,
		PlanName:   planName,
		PriceCents: priceCents,
		StartsAt:   startsAt,
		ExpiresAt:  expiresAt,
		Status:     SubActive,
	}
	if err := s.repo.Create(sub); err != nil {
		return nil, err
	}
	return sub, nil
}

// ListBySite devuelve las suscripciones de una sede.
func (s *Service) ListBySite(siteID, status string, limit, offset int) ([]*Subscription, error) {
	return s.repo.ListBySite(siteID, status, limit, offset)
}

// Get devuelve una suscripción por ID.
func (s *Service) Get(id string) (*Subscription, error) {
	return s.repo.ByID(id)
}

// Cancel cancela una suscripción.
func (s *Service) Cancel(id string) error {
	return s.repo.UpdateStatus(id, SubCancelled)
}

// Validate checks if a vehicle has an active subscription at a site.
// Implements parking.SubscriptionChecker.
func (s *Service) Validate(vehicleID, siteID string, now time.Time) (*parking.Subscription, error) {
	sub, err := s.repo.ActiveByVehicleSite(vehicleID, siteID, now)
	if err != nil {
		return nil, err
	}
	return &parking.Subscription{
		ID:        sub.ID,
		VehicleID: sub.VehicleID,
		SiteID:    sub.SiteID,
		PlanName:  sub.PlanName,
		ExpiresAt: sub.ExpiresAt,
		Status:    sub.Status,
	}, nil
}

// ValidateByPlate checks if a plate has an active subscription at a site.
// Implements parking.SubscriptionChecker.
func (s *Service) ValidateByPlate(plateNormalized, siteID string, now time.Time) (*parking.Subscription, error) {
	sub, err := s.repo.ActiveByPlateSite(plateNormalized, siteID, now)
	if err != nil {
		return nil, err
	}
	return &parking.Subscription{
		ID:        sub.ID,
		VehicleID: sub.VehicleID,
		SiteID:    sub.SiteID,
		PlanName:  sub.PlanName,
		ExpiresAt: sub.ExpiresAt,
		Status:    sub.Status,
	}, nil
}
