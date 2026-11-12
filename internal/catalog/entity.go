package catalog

type Site struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Code          string `json:"code"`
	Timezone      string `json:"timezone"`
	Currency      string `json:"currency"`
	CapacityTotal int    `json:"capacity_total"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

type Zone struct {
	ID        string `json:"id"`
	SiteID    string `json:"site_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Capacity  int    `json:"capacity"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at,omitempty"`
}

type Space struct {
	ID        string `json:"id"`
	ZoneID    string `json:"zone_id"`
	Code      string `json:"code"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type VehicleType struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at,omitempty"`
}
