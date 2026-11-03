package billing

import (
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
)

// Service contiene la lógica de negocio de pagos.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreatePayment registra un pago (implementa PaymentCreator de parking).
func (s *Service) CreatePayment(ticketID string, amountCents int64, method string, cashierUserID string) (string, error) {
	if amountCents <= 0 {
		return "", apperr.New(apperr.ErrInvalidInput, "el monto debe ser mayor a cero", 400, nil)
	}
	if method == "" {
		method = MethodCash
	}

	p := &Payment{
		ID:            ids.UUIDv7String(),
		TicketID:      &ticketID,
		AmountCents:   amountCents,
		Currency:      "COP",
		Method:        method,
		Status:        PaymentCompleted,
		CashierUserID: &cashierUserID,
	}
	if err := s.repo.Create(p); err != nil {
		return "", err
	}
	return p.ID, nil
}

// RegisterPayment registra un pago con idempotencia.
func (s *Service) RegisterPayment(ticketID, subscriptionID *string, amountCents int64, currency, method, reference string, cashierUserID *string, idempotencyKey *string) (*Payment, error) {
	if amountCents <= 0 {
		return nil, apperr.New(apperr.ErrInvalidInput, "el monto debe ser mayor a cero", 400, nil)
	}
	if method == "" {
		method = MethodCash
	}
	if currency == "" {
		currency = "COP"
	}

	// Verificar idempotencia
	if idempotencyKey != nil && *idempotencyKey != "" {
		existing, err := s.repo.ByIdempotencyKey(*idempotencyKey)
		if err == nil && existing != nil {
			return existing, nil
		}
	}

	var refPtr *string
	if reference != "" {
		refPtr = &reference
	}

	p := &Payment{
		ID:             ids.UUIDv7String(),
		TicketID:       ticketID,
		SubscriptionID: subscriptionID,
		AmountCents:    amountCents,
		Currency:       currency,
		Method:         method,
		Status:         PaymentCompleted,
		Reference:      refPtr,
		CashierUserID:  cashierUserID,
		IdempotencyKey: idempotencyKey,
	}
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// RefundPayment anula o reembolsa un pago existente.
func (s *Service) RefundPayment(paymentID string, reason string) (*Payment, error) {
	if paymentID == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "id de pago requerido", 400, nil)
	}
	p, err := s.repo.ByID(paymentID)
	if err != nil {
		return nil, err
	}
	if p.Status != PaymentCompleted {
		return nil, apperr.New(apperr.ErrInvalidInput, "solo se pueden reembolsar pagos completados", 400, nil)
	}

	if err := s.repo.UpdateStatus(paymentID, PaymentRefunded); err != nil {
		return nil, err
	}
	p.Status = PaymentRefunded
	return p, nil
}

// PaymentByTicket devuelve el pago asociado a un ticket.
func (s *Service) PaymentByTicket(ticketID string) (*Payment, error) {
	return s.repo.ByTicket(ticketID)
}
