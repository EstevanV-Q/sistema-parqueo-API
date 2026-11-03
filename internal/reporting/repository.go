package reporting

// Repository define el acceso de solo lectura para reportes.
type Repository interface {
	Ocupacion(siteID string) (*Ocupacion, error)
	Ingresos(siteID string, from, to string) ([]IngresoDiario, error)
	CajaDiaria(siteID string, date string) (*CajaResumen, error)
}
