package vehicles

// Repository define el contrato de acceso a datos de vehículos.
type Repository interface {
	Create(v *Vehicle) error
	ByID(id string) (*Vehicle, error)
	ByPlate(plateNormalized string) (*Vehicle, error)
	ListByCustomer(customerID string) ([]*Vehicle, error)
	List(limit, offset int) ([]*Vehicle, error)
	Update(v *Vehicle) error
}
