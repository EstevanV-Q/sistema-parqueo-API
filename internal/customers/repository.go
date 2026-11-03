package customers

// Repository define el contrato de acceso a datos de clientes.
type Repository interface {
	Create(c *Customer) error
	ByID(id string) (*Customer, error)
	ByDocument(document string) (*Customer, error)
	List(limit, offset int) ([]*Customer, error)
}
