package devices

import (
	"fmt"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/security"
	"parking-backend/internal/shared/plate"
)

// Gate permite consultar al módulo de parking si una entrada/salida procede.
type Gate interface {
	AuthorizeEntry(siteID, plateNormalized string) (allow bool, reason string, err error)
	AuthorizeExit(siteID, plateNormalized string) (allow bool, reason string, err error)
}

// TicketCreator permite crear tickets automáticamente al detectar un vehículo.
type TicketCreator interface {
	AutoEnter(siteID, plateNormalized string) (ticketID string, err error)
}

// EventPublisher publica cambios en tiempo real.
type EventPublisher interface {
	Publish(siteID, event string, payload map[string]any)
}

// Service contiene la lógica de negocio de dispositivos.
type Service struct {
	repo          Repository
	gate          Gate
	ticketCreator TicketCreator
	publisher     EventPublisher
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// WithGate inyecta la validación contra parking.
func (s *Service) WithGate(g Gate) *Service {
	s.gate = g
	return s
}

// WithTicketCreator inyecta el creador automático de tickets.
func (s *Service) WithTicketCreator(tc TicketCreator) *Service {
	s.ticketCreator = tc
	return s
}

// WithPublisher inyecta el publicador de eventos en tiempo real.
func (s *Service) WithPublisher(p EventPublisher) *Service {
	s.publisher = p
	return s
}

// RegisterDevice registra un dispositivo. Recibe la API key en claro y guarda
// únicamente su hash SHA-256.
func (s *Service) RegisterDevice(siteID, name, typ, apiKey string) (*Device, error) {
	if siteID == "" || name == "" || typ == "" || apiKey == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "todos los campos son requeridos", 400, nil)
	}
	switch typ {
	case DeviceBarrier, DeviceLPR, DeviceRFID, DevicePrinter:
	default:
		return nil, apperr.New(apperr.ErrInvalidInput, "tipo de dispositivo inválido", 400, nil)
	}

	d := &Device{
		ID:         ids.UUIDv7String(),
		SiteID:     siteID,
		Name:       name,
		Type:       typ,
		APIKeyHash: security.HashToken(apiKey),
		IsActive:   true,
	}
	if err := s.repo.Create(d); err != nil {
		return nil, err
	}
	return d, nil
}

// Authenticate valida la API key de un dispositivo.
func (s *Service) Authenticate(apiKey string) (*Device, error) {
	if apiKey == "" {
		return nil, ErrDeviceUnauthorized
	}
	d, err := s.repo.ByAPIKey(security.HashToken(apiKey))
	if err != nil {
		return nil, ErrDeviceUnauthorized
	}
	if !d.IsActive {
		return nil, ErrDeviceInactive
	}
	return d, nil
}

// DeviceByID devuelve un dispositivo por ID (solo para administración).
func (s *Service) DeviceByID(id string) (*Device, error) {
	return s.repo.ByID(id)
}

// ListBySite lista los dispositivos de una sede.
func (s *Service) ListBySite(siteID string) ([]*Device, error) {
	return s.repo.ListBySite(siteID)
}

// ProcessEvent procesa un evento de dispositivo y devuelve la decisión.
// Es idempotente por `event_id`.
func (s *Service) ProcessEvent(device *Device, eventID, eventType, plateNormalized string) (*DeviceEvent, error) {
	if device == nil {
		return nil, ErrDeviceUnauthorized
	}
	if eventID == "" || eventType == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "event_id y event_type son requeridos", 400, nil)
	}
	if plateNormalized != "" {
		plateNormalized = plate.Normalize(plateNormalized)
		if plateNormalized == "" {
			return nil, apperr.New(apperr.ErrInvalidInput, "la placa es inválida", 400, nil)
		}
	}

	// Deduplicación por event_id
	if existing, err := s.repo.EventByID(eventID); err == nil && existing != nil {
		return existing, nil
	}

	decision := DecisionAllow
	var reason string

	switch eventType {
	case EventEntryDetected:
		if plateNormalized == "" {
			return nil, apperr.New(apperr.ErrInvalidInput, "la placa es requerida para detectar entrada", 400, nil)
		}
		if s.gate != nil {
			allow, r, err := s.gate.AuthorizeEntry(device.SiteID, plateNormalized)
			if err != nil {
				return nil, err
			}
			reason = r
			if !allow {
				decision = DecisionDeny
			} else if s.ticketCreator != nil {
				ticketID, err := s.ticketCreator.AutoEnter(device.SiteID, plateNormalized)
				if err != nil {
					return nil, err
				}
				if ticketID == "" {
					return nil, apperr.New(apperr.ErrInternal, "no se pudo registrar la entrada automática", 500, nil)
				}
				reason = fmt.Sprintf("entrada autorizada (ticket %s)", ticketID)
			}
		} else {
			reason = "entrada autorizada"
		}
	case EventExitDetected:
		if plateNormalized == "" {
			return nil, apperr.New(apperr.ErrInvalidInput, "la placa es requerida para detectar salida", 400, nil)
		}
		if s.gate != nil {
			allow, r, err := s.gate.AuthorizeExit(device.SiteID, plateNormalized)
			if err != nil {
				return nil, err
			}
			reason = r
			if !allow {
				decision = DecisionDeny
			}
		} else {
			reason = "salida autorizada"
		}
	case EventHeartbeat:
		if err := s.repo.UpdateHeartbeat(device.ID, time.Now().UTC()); err != nil {
			return nil, err
		}
		reason = "heartbeat registrado"
	case EventCommandAck:
		reason = "comando confirmado"
	default:
		return nil, ErrInvalidEventType
	}

	e := &DeviceEvent{
		ID:        ids.UUIDv7String(),
		DeviceID:  device.ID,
		EventID:   eventID,
		EventType: eventType,
		Decision:  decision,
		Reason:    &reason,
	}
	if plateNormalized != "" {
		e.PlateNormalized = &plateNormalized
	}
	if err := s.repo.CreateEvent(e); err != nil {
		return nil, err
	}

	if s.publisher != nil {
		s.publisher.Publish(device.SiteID, "device.event", map[string]any{
			"device_id": device.ID,
			"event_id":  eventID,
			"event":     eventType,
			"decision":  decision,
			"plate":     plateNormalized,
		})
	}

	return e, nil
}
