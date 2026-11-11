package parking

import (
	"fmt"
	"log"

	"parking-backend/internal/parking/pricing"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/clock"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/shared/plate"
)

// Service contiene la lógica de negocio de tickets.
type Service struct {
	repo              Repository
	ratePlanReader    RatePlanReader
	spaceUpdater      SpaceStatusUpdater
	subscriptionCheck SubscriptionChecker
	vehicleTypes      VehicleTypeResolver
	locationValidator LocationValidator
	defaultTypeCode   string
	publisher         EventPublisher
	now               clock.NowFunc
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  clock.Real,
	}
}

// WithRatePlanReader inyecta el lector de tarifas.
func (s *Service) WithRatePlanReader(r RatePlanReader) *Service {
	s.ratePlanReader = r
	return s
}

// WithSpaceUpdater inyecta el actualizador de estado de espacios.
func (s *Service) WithSpaceUpdater(u SpaceStatusUpdater) *Service {
	s.spaceUpdater = u
	return s
}

// WithLocationValidator inyecta la validación de sede/zona/espacio.
func (s *Service) WithLocationValidator(v LocationValidator) *Service {
	s.locationValidator = v
	return s
}

// WithSubscriptionChecker inyecta el validador de abonos.
func (s *Service) WithSubscriptionChecker(c SubscriptionChecker) *Service {
	s.subscriptionCheck = c
	return s
}

// WithEventPublisher inyecta el publicador de eventos en tiempo real.
func (s *Service) WithEventPublisher(p EventPublisher) *Service {
	s.publisher = p
	return s
}

// WithVehicleTypeResolver inyecta la resolución de tipos de vehículo.
func (s *Service) WithVehicleTypeResolver(r VehicleTypeResolver) *Service {
	s.vehicleTypes = r
	return s
}

// WithDefaultVehicleType define el código de tipo usado en entradas LPR.
func (s *Service) WithDefaultVehicleType(code string) *Service {
	s.defaultTypeCode = code
	return s
}

// WithClock permite inyectar un reloj determinista en pruebas.
func (s *Service) WithClock(now clock.NowFunc) *Service {
	s.now = now
	return s
}

func (s *Service) publish(siteID, event string, payload map[string]any) {
	if s.publisher != nil && siteID != "" {
		s.publisher.Publish(siteID, event, payload)
	}
}

// CreateTicket registra la entrada de un vehículo.
// Todo ocurre dentro de una transacción manejada por el llamante.
func (s *Service) CreateTicket(siteID, zoneID, spaceID, vehicleID, plateNormalized, plateRaw, typeID, ratePlanID string) (*Ticket, error) {
	if siteID == "" || plateNormalized == "" || typeID == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "sitio, placa y tipo de vehículo son requeridos", 400, nil)
	}
	plateNormalized = plate.Normalize(plateNormalized)
	if plateNormalized == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "la placa es inválida", 400, nil)
	}
	if plateRaw == "" {
		plateRaw = plateNormalized
	}
	if s.locationValidator != nil {
		if err := s.locationValidator.ValidateLocation(siteID, zoneID, spaceID); err != nil {
			return nil, err
		}
	}

	// Verificar que el espacio no tenga un ticket abierto
	if spaceID != "" {
		existing, err := s.repo.OpenBySpace(spaceID)
		if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
			return nil, err
		}
		if existing != nil {
			return nil, ErrSpaceOccupied
		}
	}

	// Verificar que la placa no esté ya adentro en esta sede
	existing, err := s.repo.OpenByPlateSite(plateNormalized, siteID)
	if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrVehicleAlreadyIn
	}

	// Validar abono vigente (si hay checker)
	var subID *string
	if s.subscriptionCheck != nil {
		if vehicleID != "" {
			sub, err := s.subscriptionCheck.Validate(vehicleID, siteID, s.now())
			if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
				return nil, err
			}
			if sub != nil && sub.Status == SubActive {
				subID = &sub.ID
			}
		}
		if subID == nil && plateNormalized != "" {
			sub, err := s.subscriptionCheck.ValidateByPlate(plateNormalized, siteID, s.now())
			if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
				return nil, err
			}
			if sub != nil && sub.Status == SubActive {
				subID = &sub.ID
				if vehicleID == "" {
					vehicleID = sub.VehicleID
				}
			}
		}
	}

	// Si no se indicó tarifa, se resuelve la tarifa activa de la sede/tipo.
	if ratePlanID == "" && s.ratePlanReader != nil {
		if rp, err := s.ratePlanReader.DefaultRatePlan(siteID, typeID); err == nil && rp != nil {
			ratePlanID = rp.ID
		}
	}

	var rpID *string
	if ratePlanID != "" {
		rpID = &ratePlanID
	}

	t := &Ticket{
		ID:              ids.UUIDv7String(),
		SiteID:          siteID,
		ZoneID:          &zoneID,
		SpaceID:         &spaceID,
		VehicleID:       &vehicleID,
		SubscriptionID:  subID,
		PlateNormalized: plateNormalized,
		PlateRaw:        plateRaw,
		TypeID:          typeID,
		RatePlanID:      rpID,
		EntryAt:         s.now(),
		Status:          TicketOpen,
	}
	if err := s.repo.Create(t); err != nil {
		return nil, err
	}

	// Registrar evento de entrada
	if err := s.repo.CreateEvent(&TicketEvent{
		TicketID:  t.ID,
		EventType: "entry",
		Details:   "vehiculo_entrado",
	}); err != nil {
		log.Printf("parking: no se pudo registrar evento de entrada del ticket %s: %v", t.ID, err)
	}

	// Ocupar el espacio
	if s.spaceUpdater != nil && spaceID != "" {
		if err := s.spaceUpdater.UpdateStatus(spaceID, "occupied"); err != nil {
			log.Printf("parking: no se pudo marcar espacio %s como ocupado: %v", spaceID, err)
		}
	}

	s.publish(siteID, "ticket.entered", map[string]any{
		"ticket_id": t.ID,
		"space_id":  spaceID,
		"plate":     plateNormalized,
		"status":    t.Status,
	})

	return t, nil
}

// Quote salida: calcula cuánto se debe pagar sin modificar el ticket.
func (s *Service) Quote(ticketID string) (*Ticket, *pricing.Quote, error) {
	t, err := s.repo.ByID(ticketID)
	if err != nil {
		return nil, nil, err
	}
	if !t.IsOpen() {
		return nil, nil, ErrTicketNotOpen
	}

	// Abonado: sin cobro
	if (t.SubscriptionID != nil && *t.SubscriptionID != "") || (t.RatePlanID != nil && *t.RatePlanID == "subscription") {
		return t, &pricing.Quote{
			BillableMinutes: 0,
			Amount:          0,
			Breakdown: []pricing.BreakdownLine{
				{Concept: "Abono vigente", Minutes: 0, Amount: 0},
			},
		}, nil
	}

	exitAt := s.now()
	rpID := ""
	if t.RatePlanID != nil {
		rpID = *t.RatePlanID
	}

	// Obtener tarifa desde el módulo de tarifas
	// Fallback solo si no hay ninguna tarifa configurada (por ejemplo, demo sin seed).
	plan := pricing.Plan{
		ID:               rpID,
		GraceMinutes:     10,
		MinimumCents:     1000,
		FractionMinutes:  60,
		CentsPerFraction: 1500,
		DailyCapCents:    20000,
		LostTicketCents:  25000,
	}
	if s.ratePlanReader != nil {
		var rp *RatePlan
		if rpID != "" {
			rp, _ = s.ratePlanReader.GetRatePlan(rpID)
		}
		if rp == nil {
			rp, _ = s.ratePlanReader.DefaultRatePlan(t.SiteID, t.TypeID)
		}
		if rp != nil {
			plan = pricing.Plan{
				ID:               rp.ID,
				GraceMinutes:     rp.GraceMinutes,
				MinimumCents:     rp.MinimumCents,
				FractionMinutes:  rp.FractionMinutes,
				CentsPerFraction: rp.CentsPerFraction,
				DailyCapCents:    rp.DailyCapCents,
				LostTicketCents:  rp.LostTicketCents,
			}
		}
	}

	in := pricing.Input{
		EntryAt:    t.EntryAt,
		ExitAt:     exitAt,
		Plan:       plan,
		LostTicket: t.IsLost,
	}
	quote := pricing.Calculate(in)
	return t, &quote, nil
}

// Settle paga y cierra el ticket dentro de una transacción atómica.
func (s *Service) Settle(ticketID string, paidCents int64, method string, cashierUserID string) (*Ticket, error) {
	t, err := s.repo.ByID(ticketID)
	if err != nil {
		return nil, err
	}
	if !t.IsOpen() {
		return nil, ErrTicketNotOpen
	}

	// Recalcular monto bajo bloqueo
	_, quote, err := s.Quote(ticketID)
	if err != nil {
		return nil, err
	}

	// Validar que el pago cubre el monto
	if paidCents < int64(quote.Amount) {
		return nil, apperr.New(apperr.ErrInvalidInput, "el monto pagado es menor al total", 400, nil)
	}

	exitAt := s.now()
	totalCents := int64(quote.Amount)

	// Liquidación atómica: ticket, pago y evento se confirman o revierten juntos.
	if err := s.repo.SettleTicketTx(ticketID, exitAt, totalCents, paidCents, method, cashierUserID); err != nil {
		return nil, err
	}

	// Liberar el espacio
	if s.spaceUpdater != nil && t.SpaceID != nil {
		if err := s.spaceUpdater.UpdateStatus(*t.SpaceID, "free"); err != nil {
			log.Printf("parking: no se pudo liberar espacio %s al liquidar ticket %s: %v", *t.SpaceID, t.ID, err)
		}
	}

	s.publish(t.SiteID, "ticket.exited", map[string]any{
		"ticket_id":   t.ID,
		"space_id":    t.SpaceID,
		"plate":       t.PlateNormalized,
		"total_cents": totalCents,
		"paid_cents":  paidCents,
		"status":      TicketPaid,
	})

	t.ExitAt = &exitAt
	t.Status = TicketPaid
	t.TotalCents = totalCents
	t.PaidCents = paidCents
	t.UpdatedAt = exitAt
	return t, nil
}

// Cancel anula un ticket abierto (auditado).
func (s *Service) Cancel(ticketID string, actorUserID string) (*Ticket, error) {
	t, err := s.repo.ByID(ticketID)
	if err != nil {
		return nil, err
	}
	if !t.IsOpen() {
		return nil, ErrTicketNotOpen
	}

	now := s.now()
	// La cancelación y su evento se confirman juntos.
	if err := s.repo.CancelTicketTx(ticketID, now, actorUserID); err != nil {
		return nil, err
	}

	// Liberar el espacio
	if s.spaceUpdater != nil && t.SpaceID != nil {
		if err := s.spaceUpdater.UpdateStatus(*t.SpaceID, "free"); err != nil {
			log.Printf("parking: no se pudo liberar espacio %s al cancelar ticket %s: %v", *t.SpaceID, t.ID, err)
		}
	}

	s.publish(t.SiteID, "ticket.cancelled", map[string]any{
		"ticket_id": t.ID,
		"space_id":  t.SpaceID,
		"plate":     t.PlateNormalized,
		"status":    TicketCancelled,
	})

	t.Status = TicketCancelled
	t.UpdatedAt = now
	return t, nil
}

// AutoEnter registra automáticamente un ticket de entrada para una placa detectada por LPR.
func (s *Service) AutoEnter(siteID, plateNormalized string) (string, error) {
	if siteID == "" || plateNormalized == "" {
		return "", apperr.New(apperr.ErrInvalidInput, "sitio y placa son requeridos", 400, nil)
	}

	existing, err := s.repo.OpenByPlateSite(plateNormalized, siteID)
	if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return "", err
	}
	if existing != nil {
		return existing.ID, nil
	}

	vehicleID := ""
	if s.subscriptionCheck != nil {
		sub, err := s.subscriptionCheck.ValidateByPlate(plateNormalized, siteID, s.now())
		if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
			return "", err
		}
		if sub != nil {
			vehicleID = sub.VehicleID
		}
	}

	if s.vehicleTypes == nil || s.defaultTypeCode == "" {
		return "", apperr.New(apperr.ErrInternal, "resolución de tipo de vehículo no configurada", 500, nil)
	}
	typeID, err := s.vehicleTypes.VehicleTypeIDByCode(s.defaultTypeCode)
	if err != nil {
		return "", apperr.New(apperr.ErrInternal, "no se pudo resolver el tipo de vehículo por defecto", 500, err)
	}
	t, err := s.CreateTicket(siteID, "", "", vehicleID, plateNormalized, plateNormalized, typeID, "")
	if err != nil {
		return "", err
	}
	return t.ID, nil
}

// AuthorizeEntry valida si una barrera puede permitir la entrada.
// Implementa devices.Gate.
func (s *Service) AuthorizeEntry(siteID, plateNormalized string) (bool, string, error) {
	if siteID == "" || plateNormalized == "" {
		return false, "sitio y placa son requeridos", nil
	}
	existing, err := s.repo.OpenByPlateSite(plateNormalized, siteID)
	if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return false, "", err
	}
	if existing != nil {
		return false, "el vehículo ya está dentro", nil
	}
	return true, "entrada autorizada", nil
}

// AuthorizeExit valida si una barrera puede permitir la salida.
// Implementa devices.Gate.
func (s *Service) AuthorizeExit(siteID, plateNormalized string) (bool, string, error) {
	if siteID == "" || plateNormalized == "" {
		return false, "sitio y placa son requeridos", nil
	}

	// 1. Validar si el vehículo tiene abono activo en esta sede
	if s.subscriptionCheck != nil {
		sub, err := s.subscriptionCheck.ValidateByPlate(plateNormalized, siteID, s.now())
		if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
			return false, "", err
		}
		if err == nil && sub != nil && sub.Status == SubActive {
			// Si tiene un ticket abierto, cerrarlo
			if openTicket, err := s.repo.OpenByPlateSite(plateNormalized, siteID); err == nil && openTicket != nil {
				now := s.now()
				_ = s.repo.Close(openTicket.ID, now, 0, 0, TicketClosed)
				if s.spaceUpdater != nil && openTicket.SpaceID != nil {
					if err := s.spaceUpdater.UpdateStatus(*openTicket.SpaceID, "free"); err != nil {
						log.Printf("parking: no se pudo liberar espacio %s al autorizar salida de abonado: %v", *openTicket.SpaceID, err)
					}
				}
				s.publish(siteID, "ticket.exited", map[string]any{
					"ticket_id": openTicket.ID,
					"space_id":  openTicket.SpaceID,
					"plate":     plateNormalized,
					"status":    TicketClosed,
				})
			}
			return true, "salida autorizada (abonado)", nil
		}
	}

	// 2. Buscar el último ticket del vehículo en esta sede
	latest, err := s.repo.LatestByPlateSite(plateNormalized, siteID)
	if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return false, "", err
	}
	if latest == nil {
		// Fallback: verificar si hay ticket abierto
		openTicket, err := s.repo.OpenByPlateSite(plateNormalized, siteID)
		if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
			return false, "", err
		}
		latest = openTicket
	}
	if latest == nil {
		return false, "no hay ticket abierto para la placa", nil
	}

	// Si el ticket ya fue pagado previamente en caja
	if latest.Status == TicketPaid {
		now := s.now()
		if err := s.repo.Close(latest.ID, now, latest.TotalCents, latest.PaidCents, TicketClosed); err != nil {
			return false, "error al cerrar ticket", err
		}
		if s.spaceUpdater != nil && latest.SpaceID != nil {
			if err := s.spaceUpdater.UpdateStatus(*latest.SpaceID, "free"); err != nil {
				log.Printf("parking: no se pudo liberar espacio %s al cerrar ticket pagado %s: %v", *latest.SpaceID, latest.ID, err)
			}
		}
		s.publish(siteID, "ticket.exited", map[string]any{
			"ticket_id":   latest.ID,
			"space_id":    latest.SpaceID,
			"plate":       plateNormalized,
			"total_cents": latest.TotalCents,
			"paid_cents":  latest.PaidCents,
			"status":      TicketClosed,
		})
		return true, "salida autorizada (ticket pagado)", nil
	}

	// Si el ticket está abierto, verificar si aplica salida gratuita (abono o periodo de gracia)
	if latest.Status == TicketOpen {
		if latest.SubscriptionID != nil && *latest.SubscriptionID != "" {
			now := s.now()
			_ = s.repo.Close(latest.ID, now, 0, 0, TicketClosed)
			if s.spaceUpdater != nil && latest.SpaceID != nil {
				if err := s.spaceUpdater.UpdateStatus(*latest.SpaceID, "free"); err != nil {
					log.Printf("parking: no se pudo liberar espacio %s al autorizar salida de abonado: %v", *latest.SpaceID, err)
				}
			}
			s.publish(siteID, "ticket.exited", map[string]any{
				"ticket_id": latest.ID,
				"space_id":  latest.SpaceID,
				"plate":     plateNormalized,
				"status":    TicketClosed,
			})
			return true, "salida autorizada (abonado)", nil
		}

		_, quote, err := s.Quote(latest.ID)
		if err != nil {
			return false, "", err
		}
		if quote.Amount == 0 {
			now := s.now()
			_ = s.repo.Close(latest.ID, now, 0, 0, TicketClosed)
			if s.spaceUpdater != nil && latest.SpaceID != nil {
				_ = s.spaceUpdater.UpdateStatus(*latest.SpaceID, "free")
			}
			s.publish(siteID, "ticket.exited", map[string]any{
				"ticket_id": latest.ID,
				"space_id":  latest.SpaceID,
				"plate":     plateNormalized,
				"status":    TicketClosed,
			})
			return true, "salida autorizada (periodo de gracia)", nil
		}

		return false, fmt.Sprintf("ticket pendiente de pago (%d centavos)", quote.Amount), nil
	}

	return false, "el vehículo no tiene ticket activo en el parqueo", nil
}

// SubActive es el estado activo de suscripción.
const SubActive = "active"

// TicketSite devuelve el siteID del ticket indicado.
// Implementa billing.SiteResolver.
func (s *Service) TicketSite(ticketID string) (string, error) {
	t, err := s.repo.ByID(ticketID)
	if err != nil {
		return "", err
	}
	return t.SiteID, nil
}
