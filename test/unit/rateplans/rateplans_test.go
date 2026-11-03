package rateplans_test

import (
	"testing"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/rateplans"
)

/* ---------- fakes ---------- */

type fakeRepo struct {
	plans map[string]*rateplans.RatePlan
}

func (f *fakeRepo) Create(r *rateplans.RatePlan) error {
	if r.ID == "" {
		r.ID = ids.UUIDv7String()
	}
	f.plans[r.ID] = r
	return nil
}
func (f *fakeRepo) ByID(id string) (*rateplans.RatePlan, error) {
	r, ok := f.plans[id]
	if !ok {
		return nil, rateplans.ErrNotFound
	}
	return r, nil
}
func (f *fakeRepo) BySite(siteID string) ([]*rateplans.RatePlan, error) {
	out := []*rateplans.RatePlan{}
	for _, r := range f.plans {
		if r.SiteID == siteID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRepo) ActiveBySiteType(siteID, typeID string) (*rateplans.RatePlan, error) {
	for _, r := range f.plans {
		if r.SiteID == siteID && r.VehicleTypeID != nil && *r.VehicleTypeID == typeID && r.IsActive {
			return r, nil
		}
	}
	return nil, rateplans.ErrNotFound
}
func (f *fakeRepo) Update(r *rateplans.RatePlan) error {
	if _, ok := f.plans[r.ID]; !ok {
		return rateplans.ErrNotFound
	}
	f.plans[r.ID] = r
	return nil
}

func newRatePlanService() *rateplans.Service {
	return rateplans.NewService(&fakeRepo{plans: map[string]*rateplans.RatePlan{}})
}

/* ---------- tests ---------- */

func TestCreateRatePlan_OK(t *testing.T) {
	s := newRatePlanService()
	rp, err := s.Create("site-1", "Tarifa Hora", "COP", 15, 15, 2000, 1000, 20000, 5000, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rp.Name != "Tarifa Hora" || rp.Currency != "COP" || !rp.IsActive {
		t.Fatalf("datos inesperados: %+v", rp)
	}
}

func TestCreateRatePlan_DefaultCurrency(t *testing.T) {
	s := newRatePlanService()
	rp, err := s.Create("site-1", "Tarifa", "", 15, 15, 2000, 1000, 20000, 5000, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rp.Currency != "COP" {
		t.Fatalf("moneda por defecto debería ser COP, obtuvo %s", rp.Currency)
	}
}

func TestCreateRatePlan_InvalidFractionMinutes(t *testing.T) {
	s := newRatePlanService()
	if _, err := s.Create("site-1", "Tarifa", "COP", 15, 0, 2000, 1000, 20000, 5000, nil); err == nil {
		t.Fatal("fraction_minutes cero debe fallar")
	} else if !apperr.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("se esperaba INVALID_INPUT, obtuvo %v", err)
	}
}

func TestCreateRatePlan_MissingFields(t *testing.T) {
	s := newRatePlanService()
	if _, err := s.Create("", "Tarifa", "COP", 15, 15, 2000, 1000, 20000, 5000, nil); err == nil {
		t.Fatal("siteID vacío debe fallar")
	}
	if _, err := s.Create("site-1", "", "COP", 15, 15, 2000, 1000, 20000, 5000, nil); err == nil {
		t.Fatal("nombre vacío debe fallar")
	}
}

func TestGetAndDeactivateRatePlan(t *testing.T) {
	s := newRatePlanService()
	rp, _ := s.Create("site-1", "Tarifa", "COP", 15, 15, 2000, 1000, 20000, 5000, nil)
	got, err := s.Get(rp.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != rp.ID {
		t.Fatalf("ID inesperado: %s", got.ID)
	}
	if err := s.Deactivate(rp.ID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	got, err = s.Get(rp.ID)
	if err != nil {
		t.Fatalf("Get after deactivate: %v", err)
	}
	if got.IsActive {
		t.Fatal("la tarifa debería estar inactiva")
	}
}

func TestListBySite(t *testing.T) {
	s := newRatePlanService()
	if _, err := s.Create("site-1", "Tarifa A", "COP", 15, 15, 2000, 1000, 20000, 5000, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("site-2", "Tarifa B", "COP", 15, 15, 2000, 1000, 20000, 5000, nil); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListBySite("site-1")
	if err != nil {
		t.Fatalf("ListBySite: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("se esperaba 1 tarifa para site-1, obtuvo %d", len(list))
	}
}

func TestDefaultRatePlan(t *testing.T) {
	s := newRatePlanService()
	typeID := "car"
	rp, _ := s.Create("site-1", "Tarifa Carro", "COP", 15, 15, 2000, 1000, 20000, 5000, &typeID)
	defaultRP, err := s.DefaultRatePlan("site-1", "car")
	if err != nil {
		t.Fatalf("DefaultRatePlan: %v", err)
	}
	if defaultRP.ID != rp.ID {
		t.Fatalf("ID inesperado: %s", defaultRP.ID)
	}
}
