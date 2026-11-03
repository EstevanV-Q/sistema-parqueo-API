package catalog_test

import (
	"testing"

	"parking-backend/internal/catalog"
	"parking-backend/internal/platform/apperr"
)

/* ---------- fakes ---------- */

type fakeSiteRepo struct {
	sites  map[string]*catalog.Site
	byCode map[string]*catalog.Site
}

func (f *fakeSiteRepo) Create(s *catalog.Site) error {
	if s.ID == "" {
		s.ID = "site-" + s.Code
	}
	f.sites[s.ID] = s
	f.byCode[s.Code] = s
	return nil
}
func (f *fakeSiteRepo) ByID(id string) (*catalog.Site, error) {
	s, ok := f.sites[id]
	if !ok {
		return nil, catalog.ErrNotFound
	}
	return s, nil
}
func (f *fakeSiteRepo) ByCode(code string) (*catalog.Site, error) {
	s, ok := f.byCode[code]
	if !ok {
		return nil, catalog.ErrNotFound
	}
	return s, nil
}
func (f *fakeSiteRepo) List() ([]*catalog.Site, error) {
	out := make([]*catalog.Site, 0, len(f.sites))
	for _, s := range f.sites {
		out = append(out, s)
	}
	return out, nil
}
func (f *fakeSiteRepo) Update(s *catalog.Site) error {
	if _, ok := f.sites[s.ID]; !ok {
		return catalog.ErrNotFound
	}
	f.sites[s.ID] = s
	f.byCode[s.Code] = s
	return nil
}
func (f *fakeSiteRepo) Delete(id string) error {
	s, ok := f.sites[id]
	if !ok {
		return catalog.ErrNotFound
	}
	delete(f.byCode, s.Code)
	delete(f.sites, id)
	return nil
}

type fakeZoneRepo struct {
	zones  map[string]*catalog.Zone
	bySite map[string][]*catalog.Zone
}

func (f *fakeZoneRepo) Create(z *catalog.Zone) error {
	if z.ID == "" {
		z.ID = "zone-" + z.Name
	}
	f.zones[z.ID] = z
	f.bySite[z.SiteID] = append(f.bySite[z.SiteID], z)
	return nil
}
func (f *fakeZoneRepo) ByID(id string) (*catalog.Zone, error) {
	z, ok := f.zones[id]
	if !ok {
		return nil, catalog.ErrNotFound
	}
	return z, nil
}
func (f *fakeZoneRepo) ListBySite(siteID string) ([]*catalog.Zone, error) {
	return f.bySite[siteID], nil
}
func (f *fakeZoneRepo) Update(z *catalog.Zone) error {
	if _, ok := f.zones[z.ID]; !ok {
		return catalog.ErrNotFound
	}
	f.zones[z.ID] = z
	return nil
}
func (f *fakeZoneRepo) Delete(id string) error {
	z, ok := f.zones[id]
	if !ok {
		return catalog.ErrNotFound
	}
	delete(f.zones, id)
	out := f.bySite[z.SiteID][:0]
	for _, zz := range f.bySite[z.SiteID] {
		if zz.ID != id {
			out = append(out, zz)
		}
	}
	f.bySite[z.SiteID] = out
	return nil
}

type fakeSpaceRepo struct {
	spaces map[string]*catalog.Space
	byZone map[string][]*catalog.Space
	byCode map[string]*catalog.Space
}

func (f *fakeSpaceRepo) Create(s *catalog.Space) error {
	if s.ID == "" {
		s.ID = "space-" + s.Code
	}
	key := s.ZoneID + "|" + s.Code
	f.spaces[s.ID] = s
	f.byCode[key] = s
	f.byZone[s.ZoneID] = append(f.byZone[s.ZoneID], s)
	return nil
}
func (f *fakeSpaceRepo) ByID(id string) (*catalog.Space, error) {
	s, ok := f.spaces[id]
	if !ok {
		return nil, catalog.ErrNotFound
	}
	return s, nil
}
func (f *fakeSpaceRepo) ByCode(zoneID, code string) (*catalog.Space, error) {
	s, ok := f.byCode[zoneID+"|"+code]
	if !ok {
		return nil, catalog.ErrNotFound
	}
	return s, nil
}
func (f *fakeSpaceRepo) ListByZone(zoneID string) ([]*catalog.Space, error) {
	return f.byZone[zoneID], nil
}
func (f *fakeSpaceRepo) Update(s *catalog.Space) error {
	if _, ok := f.spaces[s.ID]; !ok {
		return catalog.ErrNotFound
	}
	f.spaces[s.ID] = s
	return nil
}
func (f *fakeSpaceRepo) UpdateStatus(id, status string) error {
	s, ok := f.spaces[id]
	if !ok {
		return catalog.ErrNotFound
	}
	s.Status = status
	return nil
}
func (f *fakeSpaceRepo) Delete(id string) error {
	s, ok := f.spaces[id]
	if !ok {
		return catalog.ErrNotFound
	}
	delete(f.spaces, id)
	delete(f.byCode, s.ZoneID+"|"+s.Code)
	out := f.byZone[s.ZoneID][:0]
	for _, sp := range f.byZone[s.ZoneID] {
		if sp.ID != id {
			out = append(out, sp)
		}
	}
	f.byZone[s.ZoneID] = out
	return nil
}

type fakeVTRepo struct {
	vts    map[string]*catalog.VehicleType
	byCode map[string]*catalog.VehicleType
}

func (f *fakeVTRepo) Create(v *catalog.VehicleType) error {
	if v.ID == "" {
		v.ID = "vt-" + v.Code
	}
	f.vts[v.ID] = v
	f.byCode[v.Code] = v
	return nil
}
func (f *fakeVTRepo) ByID(id string) (*catalog.VehicleType, error) {
	v, ok := f.vts[id]
	if !ok {
		return nil, catalog.ErrNotFound
	}
	return v, nil
}
func (f *fakeVTRepo) ByCode(code string) (*catalog.VehicleType, error) {
	v, ok := f.byCode[code]
	if !ok {
		return nil, catalog.ErrNotFound
	}
	return v, nil
}
func (f *fakeVTRepo) List() ([]*catalog.VehicleType, error) {
	out := make([]*catalog.VehicleType, 0, len(f.vts))
	for _, v := range f.vts {
		out = append(out, v)
	}
	return out, nil
}
func (f *fakeVTRepo) Update(v *catalog.VehicleType) error {
	if _, ok := f.vts[v.ID]; !ok {
		return catalog.ErrNotFound
	}
	f.vts[v.ID] = v
	f.byCode[v.Code] = v
	return nil
}
func (f *fakeVTRepo) Delete(id string) error {
	v, ok := f.vts[id]
	if !ok {
		return catalog.ErrNotFound
	}
	delete(f.vts, id)
	delete(f.byCode, v.Code)
	return nil
}

func newCatalogService() *catalog.Service {
	return catalog.NewService(
		&fakeSiteRepo{sites: map[string]*catalog.Site{}, byCode: map[string]*catalog.Site{}},
		&fakeZoneRepo{zones: map[string]*catalog.Zone{}, bySite: map[string][]*catalog.Zone{}},
		&fakeSpaceRepo{spaces: map[string]*catalog.Space{}, byZone: map[string][]*catalog.Space{}, byCode: map[string]*catalog.Space{}},
		&fakeVTRepo{vts: map[string]*catalog.VehicleType{}, byCode: map[string]*catalog.VehicleType{}},
	)
}

/* ---------- tests ---------- */

func TestCreateSite_OK(t *testing.T) {
	s := newCatalogService()
	site, err := s.CreateSite("Centro Comercial", "PLAZA-01", "America/Bogota", "COP", 100)
	if err != nil {
		t.Fatalf("CreateSite: %v", err)
	}
	if site.Code != "PLAZA-01" || !site.IsActive {
		t.Fatalf("datos inesperados: %+v", site)
	}
}

func TestCreateSite_DuplicateCode(t *testing.T) {
	s := newCatalogService()
	if _, err := s.CreateSite("Centro", "PLAZA-01", "America/Bogota", "COP", 100); err != nil {
		t.Fatalf("CreateSite: %v", err)
	}
	if _, err := s.CreateSite("Otro", "PLAZA-01", "America/Bogota", "COP", 50); err == nil {
		t.Fatal("se esperaba error de código duplicado")
	} else if !apperr.Is(err, apperr.ErrAlreadyExists) {
		t.Fatalf("se esperaba ALREADY_EXISTS, obtuvo %v", err)
	}
}

func TestCreateSite_InvalidInput(t *testing.T) {
	s := newCatalogService()
	if _, err := s.CreateSite("", "PLAZA-01", "America/Bogota", "COP", 100); err == nil {
		t.Fatal("nombre vacío debe fallar")
	}
	if _, err := s.CreateSite("Centro", "", "America/Bogota", "COP", 100); err == nil {
		t.Fatal("código vacío debe fallar")
	}
}

func TestListSites(t *testing.T) {
	s := newCatalogService()
	if _, err := s.CreateSite("A", "A-01", "America/Bogota", "COP", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSite("B", "B-01", "America/Bogota", "COP", 20); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSites()
	if err != nil {
		t.Fatalf("ListSites: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("se esperaban 2 sedes, obtuvo %d", len(list))
	}
}

func TestGetAndUpdateSite(t *testing.T) {
	s := newCatalogService()
	site, err := s.CreateSite("Centro", "PLAZA-01", "America/Bogota", "COP", 100)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSite(site.ID)
	if err != nil {
		t.Fatalf("GetSite: %v", err)
	}
	if got.Code != "PLAZA-01" {
		t.Fatalf("código inesperado: %s", got.Code)
	}
	upd, err := s.UpdateSite(site.ID, "Centro Actualizado", "PLAZA-02", "America/Bogota", "COP", 200, false)
	if err != nil {
		t.Fatalf("UpdateSite: %v", err)
	}
	if upd.Name != "Centro Actualizado" || upd.CapacityTotal != 200 || upd.IsActive {
		t.Fatalf("actualización incorrecta: %+v", upd)
	}
}

func TestDeleteSite(t *testing.T) {
	s := newCatalogService()
	site, err := s.CreateSite("Centro", "PLAZA-01", "America/Bogota", "COP", 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSite(site.ID); err != nil {
		t.Fatalf("DeleteSite: %v", err)
	}
	if _, err := s.GetSite(site.ID); err == nil {
		t.Fatal("la sede borrada aún existe")
	}
}

func TestCreateZone_OK(t *testing.T) {
	s := newCatalogService()
	site, _ := s.CreateSite("Centro", "PLAZA-01", "America/Bogota", "COP", 100)
	zone, err := s.CreateZone(site.ID, "General", "car", 50)
	if err != nil {
		t.Fatalf("CreateZone: %v", err)
	}
	if zone.SiteID != site.ID || zone.Type != "car" || zone.Capacity != 50 {
		t.Fatalf("datos inesperados: %+v", zone)
	}
}

func TestListZonesBySite(t *testing.T) {
	s := newCatalogService()
	site, _ := s.CreateSite("Centro", "PLAZA-01", "America/Bogota", "COP", 100)
	if _, err := s.CreateZone(site.ID, "Zona A", "car", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateZone(site.ID, "Zona B", "moto", 5); err != nil {
		t.Fatal(err)
	}
	zones, err := s.ListZones(site.ID)
	if err != nil {
		t.Fatalf("ListZones: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("se esperaban 2 zonas, obtuvo %d", len(zones))
	}
}

func TestCreateSpace_OK(t *testing.T) {
	s := newCatalogService()
	site, _ := s.CreateSite("Centro", "PLAZA-01", "America/Bogota", "COP", 100)
	zone, _ := s.CreateZone(site.ID, "General", "car", 10)
	space, err := s.CreateSpace(zone.ID, "A-01", "car")
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}
	if space.Code != "A-01" || space.Status != "free" {
		t.Fatalf("datos inesperados: %+v", space)
	}
}

func TestCreateSpace_DuplicateCode(t *testing.T) {
	s := newCatalogService()
	site, _ := s.CreateSite("Centro", "PLAZA-01", "America/Bogota", "COP", 100)
	zone, _ := s.CreateZone(site.ID, "General", "car", 10)
	if _, err := s.CreateSpace(zone.ID, "A-01", "car"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSpace(zone.ID, "A-01", "car"); err == nil {
		t.Fatal("se esperaba error de espacio duplicado")
	}
}

func TestUpdateSpaceStatus(t *testing.T) {
	s := newCatalogService()
	site, _ := s.CreateSite("Centro", "PLAZA-01", "America/Bogota", "COP", 100)
	zone, _ := s.CreateZone(site.ID, "General", "car", 10)
	space, _ := s.CreateSpace(zone.ID, "A-01", "car")
	if err := s.UpdateStatus(space.ID, "occupied"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, err := s.ListSpaces(zone.ID)
	if err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	for _, sp := range got {
		if sp.ID == space.ID && sp.Status != "occupied" {
			t.Fatalf("estado no actualizado: %s", sp.Status)
		}
	}
}

func TestCreateVehicleType_OK(t *testing.T) {
	s := newCatalogService()
	vt, err := s.CreateVehicleType("Automóvil", "car")
	if err != nil {
		t.Fatalf("CreateVehicleType: %v", err)
	}
	if vt.Code != "car" || !vt.IsActive {
		t.Fatalf("datos inesperados: %+v", vt)
	}
}

func TestListVehicleTypes(t *testing.T) {
	s := newCatalogService()
	if _, err := s.CreateVehicleType("Carro", "car"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVehicleType("Moto", "moto"); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListVehicleTypes()
	if err != nil {
		t.Fatalf("ListVehicleTypes: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("se esperaban 2 tipos, obtuvo %d", len(list))
	}
}
