package vehicles_test

import (
	"testing"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/vehicles"
)

/* ---------- fakes ---------- */

type fakeRepo struct {
	vehicles map[string]*vehicles.Vehicle
	byPlate  map[string]*vehicles.Vehicle
}

func (f *fakeRepo) Create(v *vehicles.Vehicle) error {
	if v.ID == "" {
		v.ID = ids.UUIDv7String()
	}
	f.vehicles[v.ID] = v
	f.byPlate[v.PlateNormalized] = v
	return nil
}
func (f *fakeRepo) ByID(id string) (*vehicles.Vehicle, error) {
	v, ok := f.vehicles[id]
	if !ok {
		return nil, vehicles.ErrNotFound
	}
	return v, nil
}
func (f *fakeRepo) ByPlate(plateNormalized string) (*vehicles.Vehicle, error) {
	v, ok := f.byPlate[plateNormalized]
	if !ok {
		return nil, vehicles.ErrNotFound
	}
	return v, nil
}
func (f *fakeRepo) ListByCustomer(customerID string) ([]*vehicles.Vehicle, error) {
	out := []*vehicles.Vehicle{}
	for _, v := range f.vehicles {
		if v.CustomerID != nil && *v.CustomerID == customerID {
			out = append(out, v)
		}
	}
	return out, nil
}
func (f *fakeRepo) List(limit, offset int) ([]*vehicles.Vehicle, error) {
	out := []*vehicles.Vehicle{}
	for _, v := range f.vehicles {
		out = append(out, v)
	}
	return out, nil
}
func (f *fakeRepo) Update(v *vehicles.Vehicle) error {
	if _, ok := f.vehicles[v.ID]; !ok {
		return vehicles.ErrNotFound
	}
	f.vehicles[v.ID] = v
	f.byPlate[v.PlateNormalized] = v
	return nil
}

func newVehicleService() *vehicles.Service {
	return vehicles.NewService(&fakeRepo{vehicles: map[string]*vehicles.Vehicle{}, byPlate: map[string]*vehicles.Vehicle{}})
}

/* ---------- tests ---------- */

func TestCreateVehicle_OK(t *testing.T) {
	s := newVehicleService()
	v, err := s.Create("XYZ987", "XYZ-987", "car", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.PlateNormalized != "XYZ987" || v.TypeID != "car" || !v.IsActive {
		t.Fatalf("datos inesperados: %+v", v)
	}
}

func TestCreateVehicle_NormalizesPlate(t *testing.T) {
	s := newVehicleService()
	v, err := s.Create("", "xyz-987", "car", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.PlateNormalized != "XYZ987" {
		t.Fatalf("placa no normalizada: %s", v.PlateNormalized)
	}
}

func TestCreateVehicle_DuplicatePlate(t *testing.T) {
	s := newVehicleService()
	if _, err := s.Create("XYZ987", "XYZ-987", "car", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("XYZ987", "XYZ-987", "car", nil); err == nil {
		t.Fatal("se esperaba error de placa duplicada")
	} else if !apperr.Is(err, apperr.ErrAlreadyExists) {
		t.Fatalf("se esperaba ALREADY_EXISTS, obtuvo %v", err)
	}
}

func TestCreateVehicle_MissingFields(t *testing.T) {
	s := newVehicleService()
	if _, err := s.Create("", "", "car", nil); err == nil {
		t.Fatal("placa y tipo vacíos deben fallar")
	}
	if _, err := s.Create("XYZ987", "", "", nil); err == nil {
		t.Fatal("tipo vacío debe fallar")
	}
}

func TestGetAndListVehicles(t *testing.T) {
	s := newVehicleService()
	cid := "customer-1"
	if _, err := s.Create("AAA111", "AAA-111", "car", &cid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("BBB222", "BBB-222", "moto", &cid); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetByPlate("AAA111")
	if err != nil {
		t.Fatalf("GetByPlate: %v", err)
	}
	list, err := s.List(cid, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("se esperaban 2 vehículos, obtuvo %d", len(list))
	}
	_ = v
}
