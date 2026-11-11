package catalog

import (
	"parking-backend/internal/platform/apperr"
)

// Service contiene la lógica de negocio del catálogo.
type Service struct {
	siteRepo  SiteRepository
	zoneRepo  ZoneRepository
	spaceRepo SpaceRepository
	vtRepo    VehicleTypeRepository
}

func NewService(siteRepo SiteRepository, zoneRepo ZoneRepository, spaceRepo SpaceRepository, vtRepo VehicleTypeRepository) *Service {
	return &Service{
		siteRepo:  siteRepo,
		zoneRepo:  zoneRepo,
		spaceRepo: spaceRepo,
		vtRepo:    vtRepo,
	}
}

// ---- Sedes ----

func (s *Service) CreateSite(name, code, timezone, currency string, capacity int) (*Site, error) {
	if name == "" || code == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "nombre y código son requeridos", 400, nil)
	}
	if _, err := s.siteRepo.ByCode(code); err == nil {
		return nil, apperr.New(apperr.ErrAlreadyExists, "el código de sede ya existe", 409, nil)
	}
	site := &Site{
		Name:          name,
		Code:          code,
		Timezone:      timezone,
		Currency:      currency,
		CapacityTotal: capacity,
		IsActive:      true,
	}
	if err := s.siteRepo.Create(site); err != nil {
		return nil, err
	}
	return site, nil
}

func (s *Service) ListSites() ([]*Site, error) {
	return s.siteRepo.List()
}

func (s *Service) GetSite(id string) (*Site, error) {
	return s.siteRepo.ByID(id)
}

func (s *Service) UpdateSite(id, name, code, timezone, currency string, capacity int, active bool) (*Site, error) {
	site, err := s.siteRepo.ByID(id)
	if err != nil {
		return nil, err
	}
	if name == "" || code == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "nombre y código son requeridos", 400, nil)
	}
	site.Name = name
	site.Code = code
	site.Timezone = timezone
	site.Currency = currency
	site.CapacityTotal = capacity
	site.IsActive = active
	if err := s.siteRepo.Update(site); err != nil {
		return nil, err
	}
	return site, nil
}

func (s *Service) DeleteSite(id string) error {
	return s.siteRepo.Delete(id)
}

// ---- Zonas ----

func (s *Service) CreateZone(siteID, name, typ string, capacity int) (*Zone, error) {
	if siteID == "" || name == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "sede y nombre son requeridos", 400, nil)
	}
	zone := &Zone{
		SiteID:   siteID,
		Name:     name,
		Type:     typ,
		Capacity: capacity,
		IsActive: true,
	}
	if err := s.zoneRepo.Create(zone); err != nil {
		return nil, err
	}
	return zone, nil
}

func (s *Service) ListZones(siteID string) ([]*Zone, error) {
	return s.zoneRepo.ListBySite(siteID)
}

func (s *Service) GetZone(id string) (*Zone, error) {
	return s.zoneRepo.ByID(id)
}

func (s *Service) UpdateZone(id, name, typ string, capacity int, active bool) (*Zone, error) {
	zone, err := s.zoneRepo.ByID(id)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "el nombre es requerido", 400, nil)
	}
	zone.Name = name
	zone.Type = typ
	zone.Capacity = capacity
	zone.IsActive = active
	if err := s.zoneRepo.Update(zone); err != nil {
		return nil, err
	}
	return zone, nil
}

func (s *Service) DeleteZone(id string) error {
	return s.zoneRepo.Delete(id)
}

// ---- Espacios ----

func (s *Service) CreateSpace(zoneID, code, typ string) (*Space, error) {
	if zoneID == "" || code == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "zona y código son requeridos", 400, nil)
	}
	if _, err := s.spaceRepo.ByCode(zoneID, code); err == nil {
		return nil, apperr.New(apperr.ErrAlreadyExists, "el código de espacio ya existe en esta zona", 409, nil)
	}
	space := &Space{
		ZoneID:   zoneID,
		Code:     code,
		Type:     typ,
		Status:   "free",
		IsActive: true,
	}
	if err := s.spaceRepo.Create(space); err != nil {
		return nil, err
	}
	return space, nil
}

func (s *Service) ListSpaces(zoneID string) ([]*Space, error) {
	return s.spaceRepo.ListByZone(zoneID)
}

func (s *Service) GetSpace(id string) (*Space, error) {
	return s.spaceRepo.ByID(id)
}

// ValidateLocation verifica que la zona pertenezca a la sede y que el espacio
// pertenezca a esa zona. Evita tickets con espacios de otra sede.
func (s *Service) ValidateLocation(siteID, zoneID, spaceID string) error {
	if siteID == "" {
		return apperr.New(apperr.ErrInvalidInput, "la sede es requerida", 400, nil)
	}
	if zoneID != "" {
		zone, err := s.zoneRepo.ByID(zoneID)
		if err != nil {
			return err
		}
		if zone.SiteID != siteID {
			return apperr.New(apperr.ErrInvalidInput, "la zona no pertenece a la sede indicada", 400, nil)
		}
	}
	if spaceID != "" {
		space, err := s.spaceRepo.ByID(spaceID)
		if err != nil {
			return err
		}
		if zoneID != "" && space.ZoneID != zoneID {
			return apperr.New(apperr.ErrInvalidInput, "el espacio no pertenece a la zona indicada", 400, nil)
		}
		if zoneID == "" {
			zone, err := s.zoneRepo.ByID(space.ZoneID)
			if err != nil {
				return err
			}
			if zone.SiteID != siteID {
				return apperr.New(apperr.ErrInvalidInput, "el espacio no pertenece a la sede indicada", 400, nil)
			}
		}
	}
	return nil
}

// SiteIDForSpace resuelve la sede de un espacio pasando por su zona.
func (s *Service) SiteIDForSpace(id string) (string, error) {
	space, err := s.spaceRepo.ByID(id)
	if err != nil {
		return "", err
	}
	zone, err := s.zoneRepo.ByID(space.ZoneID)
	if err != nil {
		return "", err
	}
	return zone.SiteID, nil
}

func (s *Service) UpdateSpace(id, code, typ, status string, active bool) (*Space, error) {
	space, err := s.spaceRepo.ByID(id)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "el código es requerido", 400, nil)
	}
	space.Code = code
	space.Type = typ
	if status != "" {
		space.Status = status
	}
	space.IsActive = active
	if err := s.spaceRepo.Update(space); err != nil {
		return nil, err
	}
	return space, nil
}

func (s *Service) UpdateStatus(spaceID, status string) error {
	return s.spaceRepo.UpdateStatus(spaceID, status)
}

func (s *Service) DeleteSpace(id string) error {
	return s.spaceRepo.Delete(id)
}

// ---- Tipos de vehículo ----

func (s *Service) CreateVehicleType(name, code string) (*VehicleType, error) {
	if name == "" || code == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "nombre y código son requeridos", 400, nil)
	}
	if _, err := s.vtRepo.ByCode(code); err == nil {
		return nil, apperr.New(apperr.ErrAlreadyExists, "el código de tipo de vehículo ya existe", 409, nil)
	}
	vt := &VehicleType{
		Name:     name,
		Code:     code,
		IsActive: true,
	}
	if err := s.vtRepo.Create(vt); err != nil {
		return nil, err
	}
	return vt, nil
}

func (s *Service) ListVehicleTypes() ([]*VehicleType, error) {
	return s.vtRepo.List()
}

// VehicleTypeIDByCode resuelve el ID de un tipo de vehículo activo por su código.
// Se usa para entradas automáticas LPR, donde no hay un tipo explícito.
func (s *Service) VehicleTypeIDByCode(code string) (string, error) {
	if code == "" {
		return "", apperr.New(apperr.ErrInvalidInput, "el código de tipo de vehículo es requerido", 400, nil)
	}
	vt, err := s.vtRepo.ByCode(code)
	if err != nil {
		return "", err
	}
	if !vt.IsActive {
		return "", apperr.New(apperr.ErrNotFound, "tipo de vehículo inactivo", 404, nil)
	}
	return vt.ID, nil
}

func (s *Service) UpdateVehicleType(id, name, code string, active bool) (*VehicleType, error) {
	vt, err := s.vtRepo.ByID(id)
	if err != nil {
		return nil, err
	}
	if name == "" || code == "" {
		return nil, apperr.New(apperr.ErrInvalidInput, "nombre y código son requeridos", 400, nil)
	}
	vt.Name = name
	vt.Code = code
	vt.IsActive = active
	if err := s.vtRepo.Update(vt); err != nil {
		return nil, err
	}
	return vt, nil
}

func (s *Service) DeleteVehicleType(id string) error {
	return s.vtRepo.Delete(id)
}
