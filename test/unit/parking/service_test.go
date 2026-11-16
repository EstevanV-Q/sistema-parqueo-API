package parking_test

import (
	"testing"
	"time"

	. "parking-backend/internal/parking"
	"parking-backend/internal/platform/apperr"
)

// ---------- Fakes ----------

type fakeRepo struct {
	tickets   map[string]*Ticket
	events    []TicketEvent
	spaceOpen map[string]string
	plateOpen map[string]string
	settleErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		tickets:   map[string]*Ticket{},
		spaceOpen: map[string]string{},
		plateOpen: map[string]string{},
	}
}

func (f *fakeRepo) Create(t *Ticket) error {
	if t.ID == "" {
		t.ID = "ticket-" + t.PlateNormalized
	}
	f.tickets[t.ID] = t
	if t.Status == TicketOpen {
		if t.SpaceID != nil && *t.SpaceID != "" {
			f.spaceOpen[*t.SpaceID] = t.ID
		}
		f.plateOpen[t.PlateNormalized+"|"+t.SiteID] = t.ID
	}
	return nil
}

func (f *fakeRepo) ByID(id string) (*Ticket, error) {
	t, ok := f.tickets[id]
	if !ok {
		return nil, apperr.New(apperr.ErrNotFound, "ticket no encontrado", 404, nil)
	}
	return t, nil
}

func (f *fakeRepo) OpenBySpace(spaceID string) (*Ticket, error) {
	id, ok := f.spaceOpen[spaceID]
	if !ok {
		return nil, apperr.New(apperr.ErrNotFound, "espacio libre", 404, nil)
	}
	return f.tickets[id], nil
}

func (f *fakeRepo) OpenByPlateSite(plateNormalized, siteID string) (*Ticket, error) {
	id, ok := f.plateOpen[plateNormalized+"|"+siteID]
	if !ok {
		return nil, apperr.New(apperr.ErrNotFound, "sin ticket abierto", 404, nil)
	}
	return f.tickets[id], nil
}

func (f *fakeRepo) LatestByPlateSite(plateNormalized, siteID string) (*Ticket, error) {
	var latest *Ticket
	for _, t := range f.tickets {
		if t.PlateNormalized == plateNormalized && t.SiteID == siteID {
			if latest == nil || t.EntryAt.After(latest.EntryAt) {
				latest = t
			}
		}
	}
	if latest == nil {
		return nil, apperr.New(apperr.ErrNotFound, "sin tickets", 404, nil)
	}
	return latest, nil
}

func (f *fakeRepo) SettleTicketTx(ticketID string, exitAt time.Time, totalCents, paidCents int64, method string, cashierUserID string) error {
	if f.settleErr != nil {
		return f.settleErr
	}
	t, ok := f.tickets[ticketID]
	if !ok {
		return apperr.New(apperr.ErrNotFound, "ticket no encontrado", 404, nil)
	}
	if !t.IsOpen() {
		return ErrTicketNotOpen
	}
	t.ExitAt = &exitAt
	t.TotalCents = totalCents
	t.PaidCents = paidCents
	t.Status = TicketPaid
	if t.SpaceID != nil {
		delete(f.spaceOpen, *t.SpaceID)
	}
	delete(f.plateOpen, t.PlateNormalized+"|"+t.SiteID)

	event := &TicketEvent{TicketID: t.ID, EventType: "exit", Details: "vehiculo_pagado_salido"}
	if cashierUserID != "" {
		event.ActorUserID = &cashierUserID
	}
	f.events = append(f.events, *event)
	return nil
}

func (f *fakeRepo) CancelTicketTx(ticketID string, exitAt time.Time, actorUserID string) error {
	t, ok := f.tickets[ticketID]
	if !ok {
		return apperr.New(apperr.ErrNotFound, "ticket no encontrado", 404, nil)
	}
	if !t.IsOpen() {
		return ErrTicketNotOpen
	}
	t.ExitAt = &exitAt
	t.TotalCents = 0
	t.PaidCents = 0
	t.Status = TicketCancelled
	if t.SpaceID != nil {
		delete(f.spaceOpen, *t.SpaceID)
	}
	delete(f.plateOpen, t.PlateNormalized+"|"+t.SiteID)

	event := &TicketEvent{TicketID: t.ID, EventType: "cancel", Details: "ticket_cancelado"}
	if actorUserID != "" {
		event.ActorUserID = &actorUserID
	}
	f.events = append(f.events, *event)
	return nil
}

func (f *fakeRepo) Update(t *Ticket) error {
	f.tickets[t.ID] = t
	return nil
}

func (f *fakeRepo) Close(id string, exitAt time.Time, totalCents, paidCents int64, status string) error {
	t, ok := f.tickets[id]
	if !ok {
		return apperr.New(apperr.ErrNotFound, "ticket no encontrado", 404, nil)
	}
	t.ExitAt = &exitAt
	t.TotalCents = totalCents
	t.PaidCents = paidCents
	t.Status = status
	if t.SpaceID != nil {
		delete(f.spaceOpen, *t.SpaceID)
	}
	delete(f.plateOpen, t.PlateNormalized+"|"+t.SiteID)
	return nil
}

func (f *fakeRepo) Events(ticketID string) ([]TicketEvent, error) {
	var out []TicketEvent
	for _, e := range f.events {
		if e.TicketID == ticketID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRepo) CreateEvent(e *TicketEvent) error {
	f.events = append(f.events, *e)
	return nil
}

func (f *fakeRepo) ListBySite(siteID, status string, limit, offset int) ([]*Ticket, error) {
	var out []*Ticket
	for _, t := range f.tickets {
		if t.SiteID == siteID && (status == "" || t.Status == status) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeRepo) CountBySite(siteID, status string) (int, error) {
	var n int
	for _, t := range f.tickets {
		if t.SiteID == siteID && (status == "" || t.Status == status) {
			n++
		}
	}
	return n, nil
}

type fakeRatePlans struct{}

func (fakeRatePlans) GetRatePlan(id string) (*RatePlan, error) {
	return testPlan(id), nil
}

func (fakeRatePlans) DefaultRatePlan(siteID, typeID string) (*RatePlan, error) {
	return testPlan("rp-default"), nil
}

func testPlan(id string) *RatePlan {
	return &RatePlan{
		ID:               id,
		GraceMinutes:     10,
		MinimumCents:     1000,
		FractionMinutes:  60,
		CentsPerFraction: 1500,
		DailyCapCents:    20000,
		LostTicketCents:  25000,
	}
}

type fakeSpaces struct {
	statuses map[string]string
}

func (f *fakeSpaces) UpdateStatus(spaceID, status string) error {
	f.statuses[spaceID] = status
	return nil
}

type fakeSubscriptions struct {
	active bool
}

func (f *fakeSubscriptions) Validate(vehicleID, siteID string, now time.Time) (*Subscription, error) {
	if !f.active {
		return nil, apperr.New(apperr.ErrNotFound, "sin abono", 404, nil)
	}
	return &Subscription{ID: "sub-1", VehicleID: vehicleID, SiteID: siteID, Status: SubActive}, nil
}

func (f *fakeSubscriptions) ValidateByPlate(plateNormalized, siteID string, now time.Time) (*Subscription, error) {
	if !f.active {
		return nil, apperr.New(apperr.ErrNotFound, "sin abono", 404, nil)
	}
	return &Subscription{ID: "sub-1", VehicleID: "veh-1", SiteID: siteID, Status: SubActive}, nil
}

// ---------- Helpers ----------

var fixedNow = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func newTestService() (*Service, *fakeRepo, *fakeSpaces) {
	repo := newFakeRepo()
	spaces := &fakeSpaces{statuses: map[string]string{}}

	svc := NewService(repo).
		WithRatePlanReader(fakeRatePlans{}).
		WithSpaceUpdater(spaces).
		WithSubscriptionChecker(&fakeSubscriptions{})
	svc.WithClock(func() time.Time { return fixedNow })

	return svc, repo, spaces
}

// ---------- Tests ----------

func TestCreateTicket_UsesDefaultPlanAndOccupiesSpace(t *testing.T) {
	svc, _, spaces := newTestService()

	ticket, err := svc.CreateTicket("site-1", "zone-1", "space-1", "veh-1", "ABC123", "ABC-123", "car", "")
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if ticket.Status != TicketOpen {
		t.Fatalf("status = %s, quiere open", ticket.Status)
	}
	if ticket.RatePlanID == nil || *ticket.RatePlanID != "rp-default" {
		t.Fatalf("rate_plan_id = %v, quiere rp-default", ticket.RatePlanID)
	}
	if spaces.statuses["space-1"] != "occupied" {
		t.Fatalf("estado del espacio = %q, quiere occupied", spaces.statuses["space-1"])
	}
}

func TestCreateTicket_RejectsDuplicatePlate(t *testing.T) {
	svc, _, _ := newTestService()

	if _, err := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", ""); err != nil {
		t.Fatalf("primer ingreso: %v", err)
	}
	_, err := svc.CreateTicket("site-1", "zone-2", "space-2", "", "ABC123", "ABC-123", "car", "")
	if !apperr.Is(err, apperr.ErrVehicleAlreadyIn) {
		t.Fatalf("err = %v, quiere VEHICLE_ALREADY_IN", err)
	}
}

func TestCreateTicket_RejectsOccupiedSpace(t *testing.T) {
	svc, _, _ := newTestService()

	if _, err := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", ""); err != nil {
		t.Fatalf("primer ingreso: %v", err)
	}
	_, err := svc.CreateTicket("site-1", "zone-1", "space-1", "", "XYZ987", "XYZ-987", "car", "")
	if !apperr.Is(err, apperr.ErrSpaceOccupied) {
		t.Fatalf("err = %v, quiere SPACE_OCCUPIED", err)
	}
}

func TestQuoteAndSettle(t *testing.T) {
	svc, repo, spaces := newTestService()

	ticket, err := svc.CreateTicket("site-1", "zone-1", "space-1", "veh-1", "ABC123", "ABC-123", "car", "")
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	// Simular dos horas de permanencia.
	repo.tickets[ticket.ID].EntryAt = fixedNow.Add(-2 * time.Hour)

	_, quote, err := svc.Quote(ticket.ID)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if quote.Amount <= 0 {
		t.Fatalf("monto cotizado = %d, quiere > 0", quote.Amount)
	}

	settled, err := svc.Settle(ticket.ID, int64(quote.Amount), "cash", "user-1")
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if settled.Status != TicketPaid {
		t.Fatalf("status = %s, quiere paid", settled.Status)
	}
	if settled.TotalCents != int64(quote.Amount) {
		t.Fatalf("total = %d, quiere %d", settled.TotalCents, quote.Amount)
	}
	if spaces.statuses["space-1"] != "free" {
		t.Fatalf("estado del espacio = %q, quiere free", spaces.statuses["space-1"])
	}
	events, err := repo.Events(ticket.ID)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) == 0 || events[len(events)-1].EventType != "exit" {
		t.Fatalf("eventos del ticket = %+v, quiere un evento exit al final", events)
	}
	last := events[len(events)-1]
	if last.ActorUserID == nil || *last.ActorUserID != "user-1" {
		t.Fatalf("actor del evento = %v, quiere user-1", last.ActorUserID)
	}
}

func TestSettle_RejectsUnderpayment(t *testing.T) {
	svc, repo, _ := newTestService()

	ticket, _ := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", "")
	repo.tickets[ticket.ID].EntryAt = fixedNow.Add(-2 * time.Hour)

	_, err := svc.Settle(ticket.ID, 1, "cash", "user-1")
	if !apperr.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("err = %v, quiere INVALID_INPUT", err)
	}
}

func TestSettle_DoesNotFallbackWhenAtomicTransactionFails(t *testing.T) {
	svc, repo, spaces := newTestService()

	ticket, _ := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", "")
	repo.tickets[ticket.ID].EntryAt = fixedNow.Add(-2 * time.Hour)
	repo.settleErr = apperr.New("DB_ERROR", "fallo simulado", 500, nil)

	_, quote, err := svc.Quote(ticket.ID)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if _, err := svc.Settle(ticket.ID, int64(quote.Amount), "cash", "user-1"); err == nil {
		t.Fatal("Settle debió fallar y no aplicar un cierre parcial")
	}
	if repo.tickets[ticket.ID].Status != TicketOpen {
		t.Fatalf("status = %s, el ticket debe seguir abierto", repo.tickets[ticket.ID].Status)
	}
	if spaces.statuses["space-1"] != "occupied" {
		t.Fatalf("estado del espacio = %q, debe seguir occupied", spaces.statuses["space-1"])
	}
}

func TestCancel_IsAtomicAndRecordsActor(t *testing.T) {
	svc, repo, spaces := newTestService()

	ticket, _ := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", "")
	cancelled, err := svc.Cancel(ticket.ID, "user-1")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if cancelled.Status != TicketCancelled {
		t.Fatalf("status = %s, quiere cancelled", cancelled.Status)
	}
	if spaces.statuses["space-1"] != "free" {
		t.Fatalf("estado del espacio = %q, quiere free", spaces.statuses["space-1"])
	}
	events, err := repo.Events(ticket.ID)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) == 0 || events[len(events)-1].EventType != "cancel" {
		t.Fatalf("eventos = %+v, quiere un evento cancel al final", events)
	}
	last := events[len(events)-1]
	if last.ActorUserID == nil || *last.ActorUserID != "user-1" {
		t.Fatalf("actor = %v, quiere user-1", last.ActorUserID)
	}
}

func TestQuote_SubscriptionIsFree(t *testing.T) {
	svc, repo, _ := newTestService()

	ticket, _ := svc.CreateTicket("site-1", "zone-1", "space-1", "veh-1", "ABC123", "ABC-123", "car", "")
	plan := "subscription"
	repo.tickets[ticket.ID].RatePlanID = &plan

	_, quote, err := svc.Quote(ticket.ID)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if quote.Amount != 0 {
		t.Fatalf("monto de abonado = %d, quiere 0", quote.Amount)
	}
}

type fakeAllocator struct {
	candidates []SpaceCandidate
}

func (f fakeAllocator) FreeSpaces(siteID, vehicleTypeID string) ([]SpaceCandidate, error) {
	return f.candidates, nil
}

func TestCreateTicket_AssignsFirstFreeSpace(t *testing.T) {
	svc, _, spaces := newTestService()
	svc.WithSpaceAllocator(fakeAllocator{candidates: []SpaceCandidate{
		{SpaceID: "space-1", ZoneID: "zone-1"},
		{SpaceID: "space-2", ZoneID: "zone-1"},
	}})

	first, err := svc.CreateTicket("site-1", "", "", "", "AAA111", "AAA111", "car", "")
	if err != nil {
		t.Fatalf("primer ingreso: %v", err)
	}
	if first.SpaceID == nil || *first.SpaceID != "space-1" {
		t.Fatalf("espacio asignado = %v, quiere space-1", first.SpaceID)
	}
	// El asignador sigue reportando space-1 libre: el servicio debe saltarlo
	// porque ya tiene un ticket abierto.
	second, err := svc.CreateTicket("site-1", "", "", "", "BBB222", "BBB222", "car", "")
	if err != nil {
		t.Fatalf("segundo ingreso: %v", err)
	}
	if second.SpaceID == nil || *second.SpaceID != "space-2" {
		t.Fatalf("espacio asignado = %v, quiere space-2", second.SpaceID)
	}
	if spaces.statuses["space-2"] != "occupied" {
		t.Fatalf("estado de space-2 = %q, quiere occupied", spaces.statuses["space-2"])
	}
	_, err = svc.CreateTicket("site-1", "", "", "", "CCC333", "CCC333", "car", "")
	if !apperr.Is(err, apperr.ErrConflict) {
		t.Fatalf("err = %v, quiere CONFLICT por parqueo lleno", err)
	}
}
