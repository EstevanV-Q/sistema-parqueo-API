package reporting

import (
	"testing"

	"parking-backend/internal/platform/apperr"
)

type fakeRepo struct {
	ocupacion    *Ocupacion
	ingresos     []IngresoDiario
	caja         *CajaResumen
	ocupacionErr error
	ingresosErr  error
	cajaErr      error
}

func (f *fakeRepo) Ocupacion(siteID string) (*Ocupacion, error) {
	return f.ocupacion, f.ocupacionErr
}
func (f *fakeRepo) Ingresos(siteID string, from, to string) ([]IngresoDiario, error) {
	return f.ingresos, f.ingresosErr
}
func (f *fakeRepo) CajaDiaria(siteID string, date string) (*CajaResumen, error) {
	return f.caja, f.cajaErr
}

func newService() *Service {
	return NewService(&fakeRepo{})
}

func TestOcupacion_OK(t *testing.T) {
	s := newService()
	s.repo.(*fakeRepo).ocupacion = &Ocupacion{
		SiteID:         "site-1",
		SiteName:       "Plaza",
		TotalSpaces:    100,
		OccupiedSpaces: 30,
		FreeSpaces:     70,
		OccupancyPct:   30.0,
	}
	o, err := s.Ocupacion("site-1")
	if err != nil {
		t.Fatalf("Ocupacion: %v", err)
	}
	if o.SiteID != "site-1" || o.TotalSpaces != 100 || o.OccupancyPct != 30.0 {
		t.Fatalf("datos inesperados: %+v", o)
	}
}

func TestOcupacion_MissingSiteID(t *testing.T) {
	s := newService()
	if _, err := s.Ocupacion(""); err == nil {
		t.Fatal("site_id vacío debe fallar")
	} else if !apperr.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("se esperaba INVALID_INPUT, obtuvo %v", err)
	}
}

func TestIngresos_OK(t *testing.T) {
	s := newService()
	s.repo.(*fakeRepo).ingresos = []IngresoDiario{
		{Date: "2026-01-01", TotalCents: 10000, Count: 5},
	}
	list, err := s.Ingresos("site-1", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("Ingresos: %v", err)
	}
	if len(list) != 1 || list[0].TotalCents != 10000 {
		t.Fatalf("datos inesperados: %+v", list)
	}
}

func TestIngresos_MissingSiteID(t *testing.T) {
	s := newService()
	if _, err := s.Ingresos("", "2026-01-01", "2026-01-31"); err == nil {
		t.Fatal("site_id vacío debe fallar")
	}
}

func TestCajaDiaria_OK(t *testing.T) {
	s := newService()
	s.repo.(*fakeRepo).caja = &CajaResumen{
		SiteID:     "site-1",
		Date:       "2026-01-01",
		TotalCents: 50000,
		Count:      10,
		ByMethod:   map[string]int64{"efectivo": 30000, "tarjeta": 20000},
	}
	c, err := s.CajaDiaria("site-1", "2026-01-01")
	if err != nil {
		t.Fatalf("CajaDiaria: %v", err)
	}
	if c.TotalCents != 50000 || c.Count != 10 || c.ByMethod["tarjeta"] != 20000 {
		t.Fatalf("datos inesperados: %+v", c)
	}
}

func TestCajaDiaria_MissingFields(t *testing.T) {
	s := newService()
	if _, err := s.CajaDiaria("", "2026-01-01"); err == nil {
		t.Fatal("site_id vacío debe fallar")
	}
	if _, err := s.CajaDiaria("site-1", ""); err == nil {
		t.Fatal("date vacío debe fallar")
	}
}
