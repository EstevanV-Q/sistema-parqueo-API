package catalog

type Site struct {
	ID          string
	Name        string
	Code        string
	Timezone    string
	Currency    string
	CapacityTotal int
	IsActive    bool
	CreatedAt   string
	UpdatedAt   string
}

type Zone struct {
	ID       string
	SiteID   string
	Name     string
	Type     string
	Capacity int
	IsActive bool
	CreatedAt string
}

type Space struct {
	ID       string
	ZoneID   string
	Code     string
	Type     string
	Status   string
	IsActive bool
	CreatedAt string
	UpdatedAt string
}

type VehicleType struct {
	ID       string
	Name     string
	Code     string
	IsActive bool
	CreatedAt string
}
