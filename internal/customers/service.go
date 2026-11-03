package customers

import (
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
)

// Service contiene la lógica de negocio de clientes.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(document, name string, phone, email *string) (*Customer, error) {
	if document == "" || name == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "documento y nombre son requeridos", 400, nil)
	}
	if _, err := s.repo.ByDocument(document); err == nil {
		return nil, apperr.New(apperr.ErrAlreadyExists, "el cliente ya está registrado", 409, nil)
	}
	c := &Customer{
		ID:       ids.UUIDv7String(),
		Document: document,
		Name:     name,
		Phone:    phone,
		Email:    email,
		IsActive: true,
	}
	if err := s.repo.Create(c); err != nil {
		return nil, err
	}
	return c, nil
}

// List devuelve clientes paginados.
func (s *Service) List(limit, offset int) ([]*Customer, error) {
	return s.repo.List(limit, offset)
}

// Get devuelve un cliente por ID.
func (s *Service) Get(id string) (*Customer, error) {
	return s.repo.ByID(id)
}
