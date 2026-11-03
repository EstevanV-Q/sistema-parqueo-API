package reporting

import "time"

// Ocupacion representa el estado de ocupación de una sede.
type Ocupacion struct {
	SiteID         string
	SiteName       string
	TotalSpaces    int
	OccupiedSpaces int
	FreeSpaces     int
	OccupancyPct   float64
	UpdatedAt      time.Time
}

// IngresoDiario representa ingresos agrupados por día.
type IngresoDiario struct {
	Date       string
	TotalCents int64
	Count      int
}

// CajaResumen representa el arqueo de caja de un día.
type CajaResumen struct {
	SiteID     string
	Date       string
	TotalCents int64
	Count      int
	ByMethod   map[string]int64
}
