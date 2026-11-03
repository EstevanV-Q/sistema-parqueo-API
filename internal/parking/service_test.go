package parking

import (
	"testing"
	"time"

	"parking-backend/internal/platform/apperr"
)

// ---------- Fakes ----------

type fakeRepo struct {
	tickets   map[string]*Ticket
	events    []TicketEvent
	spaceOpen map[string]string
	plateOpen map[string]string
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
	return apperr.New("UNSUPPORTED", "use fallback in fake", 500, nil)
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

type fakePayments struct {
	calls []string
}

func (f *fakePayments) CreatePayment(ticketID string, amountCents int64, method, cashierUserID string) (string, error) {
	f.calls = append(f.calls, ticketID)
	return "payment-1", nil
}

type fakePublisher struct {
	events []string
}

func (f *fakePublisher) Publish(siteID, event string, payload map[string]any) {
	f.events = append(f.events, event)
}

// ---------- Helpers ----------

var fixedNow = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func newTestService() (*Service, *fakeRepo, *fakeSpaces, *fakePayments, *fakePublisher) {
	repo := newFakeRepo()
	spaces := &fakeSpaces{statuses: map[string]string{}}
	payments := &fakePayments{}
	publisher := &fakePublisher{}

	svc := NewService(repo).
		WithRatePlanReader(fakeRatePlans{}).
		WithSpaceUpdater(spaces).
		WithSubscriptionChecker(&fakeSubscriptions{}).
		WithPaymentCreator(payments).
		WithEventPublisher(publisher)
	svc.now = func() time.Time { return fixedNow }

	return svc, repo, spaces, payments, publisher
}

// ---------- Tests ----------

func TestCreateTicket_UsesDefaultPlanAndOccupiesSpace(t *testing.T) {
	svc, _, spaces, _, publisher := newTestService()

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
	if len(publisher.events) != 1 || publisher.events[0] != "ticket.entered" {
		t.Fatalf("eventos publicados = %v", publisher.events)
	}
}

func TestCreateTicket_RejectsDuplicatePlate(t *testing.T) {
	svc, _, _, _, _ := newTestService()

	if _, err := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", ""); err != nil {
		t.Fatalf("primer ingreso: %v", err)
	}
	_, err := svc.CreateTicket("site-1", "zone-2", "space-2", "", "ABC123", "ABC-123", "car", "")
	if !apperr.Is(err, apperr.ErrVehicleAlreadyIn) {
		t.Fatalf("err = %v, quiere VEHICLE_ALREADY_IN", err)
	}
}

func TestCreateTicket_RejectsOccupiedSpace(t *testing.T) {
	svc, _, _, _, _ := newTestService()

	if _, err := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", ""); err != nil {
		t.Fatalf("primer ingreso: %v", err)
	}
	_, err := svc.CreateTicket("site-1", "zone-1", "space-1", "", "XYZ987", "XYZ-987", "car", "")
	if !apperr.Is(err, apperr.ErrSpaceOccupied) {
		t.Fatalf("err = %v, quiere SPACE_OCCUPIED", err)
	}
}

func TestQuoteAndSettle(t *testing.T) {
	svc, repo, spaces, payments, publisher := newTestService()

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
	if len(payments.calls) != 1 || payments.calls[0] != ticket.ID {
		t.Fatalf("pagos = %v", payments.calls)
	}
	if len(publisher.events) != 2 || publisher.events[1] != "ticket.exited" {
		t.Fatalf("eventos = %v", publisher.events)
	}
}

func TestSettle_RejectsUnderpayment(t *testing.T) {
	svc, repo, _, _, _ := newTestService()

	ticket, _ := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", "")
	repo.tickets[ticket.ID].EntryAt = fixedNow.Add(-2 * time.Hour)

	_, err := svc.Settle(ticket.ID, 1, "cash", "user-1")
	if !apperr.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("err = %v, quiere INVALID_INPUT", err)
	}
}

func TestQuote_SubscriptionIsFree(t *testing.T) {
	svc, _, _, _, _ := newTestService()

	ticket, _ := svc.CreateTicket("site-1", "zone-1", "space-1", "veh-1", "ABC123", "ABC-123", "car", "")
	plan := "subscription"
	svc.repo.(*fakeRepo).tickets[ticket.ID].RatePlanID = &plan

	_, quote, err := svc.Quote(ticket.ID)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if quote.Amount != 0 {
		t.Fatalf("monto de abonado = %d, quiere 0", quote.Amount)
	}
}

func TestAuthorizeEntryAndExit(t *testing.T) {
	svc, _, _, _, _ := newTestService()

	allow, _, err := svc.AuthorizeEntry("site-1", "ABC123")
	if err != nil || !allow {
		t.Fatalf("entrada sin ticket debe permitirse: allow=%v err=%v", allow, err)
	}

	if _, err := svc.CreateTicket("site-1", "zone-1", "space-1", "", "ABC123", "ABC-123", "car", ""); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	allow, _, _ = svc.AuthorizeEntry("site-1", "ABC123")
	if allow {
		t.Fatal("una segunda entrada con la misma placa debe denegarse")
	}

	allow, _, _ = svc.AuthorizeExit("site-1", "ABC123")
	if !allow {
		t.Fatal("la salida con ticket en periodo de gracia debe permitirse")
	}

	// Simular que el vehículo ingresó hace 2 horas sin pagar
	tkt, err := svc.CreateTicket("site-1", "zone-1", "space-1", "", "DEF456", "DEF-456", "car", "")
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	svc.repo.(*fakeRepo).tickets[tkt.ID].EntryAt = fixedNow.Add(-2 * time.Hour)

	allow, reason, _ := svc.AuthorizeExit("site-1", "DEF456")
	if allow {
		t.Fatal("la salida con ticket impago después de gracia debe denegarse")
	}
	if reason == "" {
		t.Fatal("se esperaba motivo de denegación")
	}

	// Settle del ticket
	_, quote, _ := svc.Quote(tkt.ID)
	if _, err := svc.Settle(tkt.ID, int64(quote.Amount), "cash", "user-1"); err != nil {
		t.Fatalf("Settle: %v", err)
	}

	// Ahora con ticket pagado debe permitirse la salida
	allow, _, _ = svc.AuthorizeExit("site-1", "DEF456")
	if !allow {
		t.Fatal("la salida con ticket pagado debe permitirse")
	}

	allow, _, _ = svc.AuthorizeExit("site-1", "XYZ987")
	if allow {
		t.Fatal("la salida sin ticket abierto debe denegarse")
	}
}
