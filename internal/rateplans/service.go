package rateplans

import (
	"parking-backend/internal/parking"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"time"
)

// Service contiene la lógica de negocio de tarifas.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(siteID, name, currency string, graceMinutes, fractionMinutes int, minimumCents, centsPerFraction, dailyCapCents, lostTicketCents int64, vehicleTypeID *string) (*RatePlan, error) {
	if siteID == "" || name == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "sede y nombre son requeridos", 400, nil)
	}
	if fractionMinutes <= 0 {
		return nil, apperr.New(apperr.ErrInvalidInput, "fraction_minutes debe ser mayor a cero", 400, nil)
	}
	if currency == "" {
		currency = "COP"
	}
	rp := &RatePlan{
		ID:               ids.UUIDv7String(),
		SiteID:           siteID,
		Name:             name,
		VehicleTypeID:    vehicleTypeID,
		Currency:         currency,
		GraceMinutes:     graceMinutes,
		MinimumCents:     minimumCents,
		FractionMinutes:  fractionMinutes,
		CentsPerFraction: centsPerFraction,
		DailyCapCents:    dailyCapCents,
		LostTicketCents:  lostTicketCents,
		IsActive:         true,
		ValidFrom:        time.Now().UTC(),
	}
	if err := s.repo.Create(rp); err != nil {
		return nil, err
	}
	return rp, nil
}

// ListBySite devuelve las tarifas de una sede.
func (s *Service) ListBySite(siteID string) ([]*RatePlan, error) {
	return s.repo.BySite(siteID)
}

// Get devuelve una tarifa por ID.
func (s *Service) Get(id string) (*RatePlan, error) {
	return s.repo.ByID(id)
}

// Deactivate desactiva una tarifa (borrado lógico).
func (s *Service) Deactivate(id string) error {
	rp, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	rp.IsActive = false
	return s.repo.Update(rp)
}

// GetRatePlan implements parking.RatePlanReader.
func (s *Service) GetRatePlan(id string) (*parking.RatePlan, error) {
	rp, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	return toParkingRatePlan(rp), nil
}

// DefaultRatePlan devuelve la tarifa activa para una sede y tipo de vehículo.
// Implementa parking.RatePlanReader.
func (s *Service) DefaultRatePlan(siteID, typeID string) (*parking.RatePlan, error) {
	rp, err := s.repo.ActiveBySiteType(siteID, typeID)
	if err != nil {
		return nil, err
	}
	return toParkingRatePlan(rp), nil
}

func toParkingRatePlan(rp *RatePlan) *parking.RatePlan {
	return &parking.RatePlan{
		ID:               rp.ID,
		GraceMinutes:     rp.GraceMinutes,
		MinimumCents:     rp.MinimumCents,
		FractionMinutes:  rp.FractionMinutes,
		CentsPerFraction: rp.CentsPerFraction,
		DailyCapCents:    rp.DailyCapCents,
		LostTicketCents:  rp.LostTicketCents,
	}
}
