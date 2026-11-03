package rateplans

// Repository define el contrato de acceso a datos de tarifas.
type Repository interface {
	Create(r *RatePlan) error
	ByID(id string) (*RatePlan, error)
	BySite(siteID string) ([]*RatePlan, error)
	ActiveBySiteType(siteID, typeID string) (*RatePlan, error)
	Update(r *RatePlan) error
}
