package billing_test

import (
	"testing"

	. "parking-backend/internal/billing"
	"parking-backend/internal/platform/apperr"
)

type fakeRepo struct {
	byID     map[string]*Payment
	byKey    map[string]*Payment
	byTicket map[string]*Payment
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byID:     map[string]*Payment{},
		byKey:    map[string]*Payment{},
		byTicket: map[string]*Payment{},
	}
}

func (f *fakeRepo) Create(p *Payment) error {
	if p.ID == "" {
		p.ID = "payment-1"
	}
	f.byID[p.ID] = p
	if p.IdempotencyKey != nil {
		f.byKey[*p.IdempotencyKey] = p
	}
	if p.TicketID != nil {
		f.byTicket[*p.TicketID] = p
	}
	return nil
}

func (f *fakeRepo) ByID(id string) (*Payment, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, apperr.New(apperr.ErrNotFound, "pago no encontrado", 404, nil)
	}
	return p, nil
}

func (f *fakeRepo) ByTicket(ticketID string) (*Payment, error) {
	p, ok := f.byTicket[ticketID]
	if !ok {
		return nil, apperr.New(apperr.ErrNotFound, "pago no encontrado", 404, nil)
	}
	return p, nil
}

func (f *fakeRepo) ByIdempotencyKey(key string) (*Payment, error) {
	p, ok := f.byKey[key]
	if !ok {
		return nil, apperr.New(apperr.ErrNotFound, "pago no encontrado", 404, nil)
	}
	return p, nil
}

func (f *fakeRepo) UpdateStatus(id, status string) error {
	p, ok := f.byID[id]
	if !ok {
		return apperr.New(apperr.ErrNotFound, "pago no encontrado", 404, nil)
	}
	p.Status = status
	return nil
}

func (f *fakeRepo) CajaDiaria(siteID, date string) (*CajaDiaria, error) {
	return &CajaDiaria{SiteID: siteID, Date: date, ByMethod: map[string]int64{}}, nil
}

func (f *fakeRepo) ListBySite(siteID, from, to string, limit, offset int) ([]*Payment, error) {
	return nil, nil
}

func TestRegisterPayment(t *testing.T) {
	svc := NewService(newFakeRepo())
	ticketID := "ticket-1"

	p, err := svc.RegisterPayment(&ticketID, nil, 3000, "", "", "", nil, nil)
	if err != nil {
		t.Fatalf("RegisterPayment: %v", err)
	}
	if p.AmountCents != 3000 {
		t.Fatalf("amount = %d, quiere 3000", p.AmountCents)
	}
	if p.Currency != "COP" {
		t.Fatalf("currency = %s, quiere COP", p.Currency)
	}
	if p.Method != MethodCash {
		t.Fatalf("method = %s, quiere cash", p.Method)
	}
	if p.Status != PaymentCompleted {
		t.Fatalf("status = %s, quiere completed", p.Status)
	}
}

func TestRegisterPayment_Idempotent(t *testing.T) {
	svc := NewService(newFakeRepo())
	ticketID := "ticket-1"
	key := "idem-123"

	first, err := svc.RegisterPayment(&ticketID, nil, 3000, "COP", "cash", "", nil, &key)
	if err != nil {
		t.Fatalf("primer pago: %v", err)
	}
	second, err := svc.RegisterPayment(&ticketID, nil, 3000, "COP", "cash", "", nil, &key)
	if err != nil {
		t.Fatalf("segundo pago: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("el pago idempotente devolvió IDs distintos: %s vs %s", first.ID, second.ID)
	}
}

func TestRegisterPayment_InvalidAmount(t *testing.T) {
	svc := NewService(newFakeRepo())
	if _, err := svc.RegisterPayment(nil, nil, 0, "COP", "cash", "", nil, nil); !apperr.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("err = %v, quiere INVALID_INPUT", err)
	}
}

func TestRefundPayment(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ticketID := "ticket-1"

	p, err := svc.RegisterPayment(&ticketID, nil, 3000, "COP", "cash", "", nil, nil)
	if err != nil {
		t.Fatalf("RegisterPayment: %v", err)
	}

	refunded, err := svc.RefundPayment(p.ID, "cobro indebido")
	if err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	if refunded.Status != PaymentRefunded {
		t.Fatalf("status = %s, quiere refunded", refunded.Status)
	}
}

func TestCreatePayment(t *testing.T) {
	svc := NewService(newFakeRepo())
	id, err := svc.CreatePayment("ticket-1", 1500, "", "user-1")
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if id == "" {
		t.Fatal("CreatePayment debe devolver un ID")
	}
	if _, err := svc.CreatePayment("ticket-1", 0, "", "user-1"); !apperr.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("err = %v, quiere INVALID_INPUT", err)
	}
}
