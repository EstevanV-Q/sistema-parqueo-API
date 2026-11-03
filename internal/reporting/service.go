package reporting

import (
	"parking-backend/internal/platform/apperr"
)

// Service contiene la lógica de reportes.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Ocupacion(siteID string) (*Ocupacion, error) {
	if siteID == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "site_id requerido", 400, nil)
	}
	return s.repo.Ocupacion(siteID)
}

func (s *Service) Ingresos(siteID string, from, to string) ([]IngresoDiario, error) {
	if siteID == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "site_id requerido", 400, nil)
	}
	return s.repo.Ingresos(siteID, from, to)
}

func (s *Service) CajaDiaria(siteID string, date string) (*CajaResumen, error) {
	if siteID == "" || date == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "site_id y date son requeridos", 400, nil)
	}
	return s.repo.CajaDiaria(siteID, date)
}
