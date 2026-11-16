package parking

import (
	"log"

	"parking-backend/internal/parking/pricing"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/clock"
	"parking-backend/internal/platform/ids"
)

// Service contiene la lógica de negocio de tickets.
type Service struct {
	repo              Repository
	ratePlanReader    RatePlanReader
	spaceUpdater      SpaceStatusUpdater
	spaceAllocator    SpaceAllocator
	subscriptionCheck SubscriptionChecker
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

// WithSpaceAllocator habilita la asignación automática del primer espacio libre
// cuando la entrada no indica un espacio.
func (s *Service) WithSpaceAllocator(a SpaceAllocator) *Service {
	s.spaceAllocator = a
	return s
}

// WithSubscriptionChecker inyecta el validador de abonos.
func (s *Service) WithSubscriptionChecker(c SubscriptionChecker) *Service {
	s.subscriptionCheck = c
	return s
}

// WithClock permite inyectar un reloj determinista en pruebas.
func (s *Service) WithClock(now clock.NowFunc) *Service {
	s.now = now
	return s
}

// CreateTicket registra la entrada de un vehículo.
// Todo ocurre dentro de una transacción manejada por el llamante.
func (s *Service) CreateTicket(siteID, zoneID, spaceID, vehicleID, plateNormalized, plateRaw, typeID, ratePlanID string) (*Ticket, error) {
	if siteID == "" || plateNormalized == "" || typeID == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "sitio, placa y tipo de vehículo son requeridos", 400, nil)
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

	// Asignación automática: si no se indicó espacio, se toma el primer espacio
	// libre de la sede compatible con el tipo de vehículo.
	if spaceID == "" && s.spaceAllocator != nil {
		candidates, err := s.spaceAllocator.FreeSpaces(siteID, typeID)
		if err != nil {
			return nil, err
		}
		for _, c := range candidates {
			open, err := s.repo.OpenBySpace(c.SpaceID)
			if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
				return nil, err
			}
			if open == nil {
				spaceID, zoneID = c.SpaceID, c.ZoneID
				break
			}
		}
		if spaceID == "" {
			return nil, ErrParkingFull
		}
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

	// Tarifa por defecto (₡1 000/hora) si la sede no tiene ninguna configurada.
	plan := pricing.Plan{
		ID:               rpID,
		GraceMinutes:     10,
		MinimumCents:     100000,
		FractionMinutes:  60,
		CentsPerFraction: 100000,
		DailyCapCents:    1200000,
		LostTicketCents:  1500000,
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

	t.Status = TicketCancelled
	t.UpdatedAt = now
	return t, nil
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
