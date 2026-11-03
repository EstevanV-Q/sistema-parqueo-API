package audit

import (
	"context"
	"net/http"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	ctxpkg "parking-backend/internal/platform/security/ctx"
)

// Service registra y consulta la auditoría.
type Service struct {
	repo Repository
}

// NewService crea el servicio de auditoría.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Record guarda una entrada tomando actor, IP y User-Agent del contexto.
// Si el registro falla, devuelve el error para que el llamante decida si lo
// ignora (la auditoría nunca debe tumbar la operación de negocio principal).
func (s *Service) Record(ctx context.Context, action, entityType, entityID string, oldValues, newValues map[string]any) error {
	if action == "" || entityType == "" {
		return apperr.New(apperr.ErrInvalidInput, "acción y tipo de entidad son requeridos", http.StatusBadRequest, nil)
	}

	e := &Entry{
		ID:         ids.UUIDv7String(),
		Action:     action,
		EntityType: entityType,
		OldValues:  oldValues,
		NewValues:  newValues,
	}

	if uid, ok := ctxpkg.UserIDFromCtx(ctx); ok && uid != "" {
		e.UserID = &uid
	}
	if ip, ok := ctxpkg.IPFromCtx(ctx); ok && ip != "" {
		e.IPAddress = &ip
	}
	if ua, ok := ctxpkg.UserAgentFromCtx(ctx); ok && ua != "" {
		e.UserAgent = &ua
	}
	if entityID != "" {
		e.EntityID = &entityID
	}

	return s.repo.Insert(e)
}

// List devuelve entradas de auditoría filtradas y paginadas.
func (s *Service) List(f Filter, limit, offset int) ([]*Entry, int, error) {
	entries, err := s.repo.List(f, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(f)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}
