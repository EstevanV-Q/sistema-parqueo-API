package vehicles

import (
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/shared/plate"
)

// Service contiene la lógica de negocio de vehículos.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create registra un vehículo. Si no llega la placa normalizada se calcula.
func (s *Service) Create(plateNormalized, plateRaw, typeID string, customerID *string) (*Vehicle, error) {
	if plateNormalized == "" {
		plateNormalized = plate.Normalize(plateRaw)
	}
	if plateNormalized == "" || typeID == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "placa y tipo de vehículo son requeridos", 400, nil)
	}
	if plateRaw == "" {
		plateRaw = plateNormalized
	}

	if _, err := s.repo.ByPlate(plateNormalized); err == nil {
		return nil, apperr.New(apperr.ErrAlreadyExists, "el vehículo ya está registrado", 409, nil)
	}

	v := &Vehicle{
		ID:              ids.UUIDv7String(),
		PlateNormalized: plateNormalized,
		PlateRaw:        plateRaw,
		TypeID:          typeID,
		CustomerID:      customerID,
		IsActive:        true,
	}
	if err := s.repo.Create(v); err != nil {
		return nil, err
	}
	return v, nil
}

// List devuelve vehículos paginados, opcionalmente filtrados por cliente.
func (s *Service) List(customerID string, limit, offset int) ([]*Vehicle, error) {
	if customerID != "" {
		return s.repo.ListByCustomer(customerID)
	}
	return s.repo.List(limit, offset)
}

// Get devuelve un vehículo por ID.
func (s *Service) Get(id string) (*Vehicle, error) {
	return s.repo.ByID(id)
}

// GetByPlate devuelve un vehículo por placa normalizada.
func (s *Service) GetByPlate(plateNormalized string) (*Vehicle, error) {
	return s.repo.ByPlate(plate.Normalize(plateNormalized))
}
