package billing

// Repository define el contrato de acceso a datos de pagos.
type Repository interface {
	Create(p *Payment) error
	ByID(id string) (*Payment, error)
	ByTicket(ticketID string) (*Payment, error)
	ByIdempotencyKey(key string) (*Payment, error)
	UpdateStatus(id, status string) error
	CajaDiaria(siteID string, date string) (*CajaDiaria, error)
	ListBySite(siteID string, from, to string, limit, offset int) ([]*Payment, error)
}
