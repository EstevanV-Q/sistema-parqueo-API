package customers_test

import (
	"testing"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/customers"
)

/* ---------- fakes ---------- */

type fakeRepo struct {
	customers map[string]*customers.Customer
	byDoc     map[string]*customers.Customer
}

func (f *fakeRepo) Create(c *customers.Customer) error {
	if c.ID == "" {
		c.ID = "customer-" + c.Document
	}
	f.customers[c.ID] = c
	f.byDoc[c.Document] = c
	return nil
}
func (f *fakeRepo) ByID(id string) (*customers.Customer, error) {
	c, ok := f.customers[id]
	if !ok {
		return nil, customers.ErrNotFound
	}
	return c, nil
}
func (f *fakeRepo) ByDocument(document string) (*customers.Customer, error) {
	c, ok := f.byDoc[document]
	if !ok {
		return nil, customers.ErrNotFound
	}
	return c, nil
}
func (f *fakeRepo) List(limit, offset int) ([]*customers.Customer, error) {
	out := []*customers.Customer{}
	for _, c := range f.customers {
		out = append(out, c)
	}
	return out, nil
}

func newCustomerService() *customers.Service {
	return customers.NewService(&fakeRepo{customers: map[string]*customers.Customer{}, byDoc: map[string]*customers.Customer{}})
}

/* ---------- tests ---------- */

func TestCreateCustomer_OK(t *testing.T) {
	s := newCustomerService()
	c, err := s.Create("123456789", "Juan Pérez", nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Document != "123456789" || c.Name != "Juan Pérez" || !c.IsActive {
		t.Fatalf("datos inesperados: %+v", c)
	}
}

func TestCreateCustomer_DuplicateDocument(t *testing.T) {
	s := newCustomerService()
	if _, err := s.Create("123456789", "Juan", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("123456789", "Otro", nil, nil); err == nil {
		t.Fatal("se esperaba error de documento duplicado")
	} else if !apperr.Is(err, apperr.ErrAlreadyExists) {
		t.Fatalf("se esperaba ALREADY_EXISTS, obtuvo %v", err)
	}
}

func TestCreateCustomer_MissingFields(t *testing.T) {
	s := newCustomerService()
	if _, err := s.Create("", "Juan", nil, nil); err == nil {
		t.Fatal("documento vacío debe fallar")
	}
	if _, err := s.Create("123", "", nil, nil); err == nil {
		t.Fatal("nombre vacío debe fallar")
	}
}

func TestGetAndListCustomers(t *testing.T) {
	s := newCustomerService()
	c1, err := s.Create("1", "Juan", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("2", "María", nil, nil); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("se esperaban 2 clientes, obtuvo %d", len(list))
	}
	got, err := s.Get(c1.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Juan" {
		t.Fatalf("nombre inesperado: %s", got.Name)
	}
}
