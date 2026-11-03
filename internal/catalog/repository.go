package catalog

// SiteRepository defines the data access contract for sites.
type SiteRepository interface {
	Create(s *Site) error
	ByID(id string) (*Site, error)
	ByCode(code string) (*Site, error)
	List() ([]*Site, error)
	Update(s *Site) error
	Delete(id string) error
}

// ZoneRepository defines the data access contract for zones.
type ZoneRepository interface {
	Create(z *Zone) error
	ByID(id string) (*Zone, error)
	ListBySite(siteID string) ([]*Zone, error)
	Update(z *Zone) error
	Delete(id string) error
}

// SpaceRepository defines the data access contract for spaces.
type SpaceRepository interface {
	Create(s *Space) error
	ByID(id string) (*Space, error)
	ByCode(zoneID, code string) (*Space, error)
	ListByZone(zoneID string) ([]*Space, error)
	Update(s *Space) error
	UpdateStatus(id, status string) error
	Delete(id string) error
}

// VehicleTypeRepository defines the data access contract for vehicle types.
type VehicleTypeRepository interface {
	Create(v *VehicleType) error
	ByID(id string) (*VehicleType, error)
	ByCode(code string) (*VehicleType, error)
	List() ([]*VehicleType, error)
	Update(v *VehicleType) error
	Delete(id string) error
}
