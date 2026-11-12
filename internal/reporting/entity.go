package reporting

import "time"

// Ocupacion representa el estado de ocupación de una sede.
type Ocupacion struct {
	SiteID         string    `json:"site_id"`
	SiteName       string    `json:"site_name"`
	TotalSpaces    int       `json:"total_spaces"`
	OccupiedSpaces int       `json:"occupied_spaces"`
	FreeSpaces     int       `json:"free_spaces"`
	OccupancyPct   float64   `json:"occupancy_pct"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// IngresoDiario representa ingresos agrupados por día.
type IngresoDiario struct {
	Date       string `json:"date"`
	TotalCents int64  `json:"total_cents"`
	Count      int    `json:"count"`
}

// CajaResumen representa el arqueo de caja de un día.
type CajaResumen struct {
	SiteID     string           `json:"site_id"`
	Date       string           `json:"date"`
	TotalCents int64            `json:"total_cents"`
	Count      int              `json:"count"`
	ByMethod   map[string]int64 `json:"by_method"`
}
