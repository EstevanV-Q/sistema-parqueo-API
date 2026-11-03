package subscriptions_test

import (
	"testing"
	"time"

	"parking-backend/internal/platform/ids"
	"parking-backend/internal/subscriptions"
)

/* ---------- fakes ---------- */

type fakeRepo struct {
	subs       map[string]*subscriptions.Subscription
	byVehicle  map[string]*subscriptions.Subscription
}

func (f *fakeRepo) Create(s *subscriptions.Subscription) error {
	if s.ID == "" {
		s.ID = ids.UUIDv7String()
	}
	f.subs[s.ID] = s
	f.byVehicle[s.VehicleID] = s
	return nil
}
func (f *fakeRepo) ByID(id string) (*subscriptions.Subscription, error) {
	s, ok := f.subs[id]
	if !ok {
		return nil, subscriptions.ErrNotFound
	}
	return s, nil
}
func (f *fakeRepo) ByVehicle(vehicleID string) (*subscriptions.Subscription, error) {
	s, ok := f.byVehicle[vehicleID]
	if !ok {
		return nil, subscriptions.ErrNotFound
	}
	return s, nil
}
func (f *fakeRepo) ActiveByVehicleSite(vehicleID, siteID string, now time.Time) (*subscriptions.Subscription, error) {
	s, ok := f.byVehicle[vehicleID]
	if !ok {
		return nil, subscriptions.ErrNotFound
	}
	if s.SiteID != siteID || !s.IsActiveAt(now) {
		return nil, subscriptions.ErrNotFound
	}
	return s, nil
}
func (f *fakeRepo) ActiveByPlateSite(plateNormalized, siteID string, now time.Time) (*subscriptions.Subscription, error) {
	return nil, subscriptions.ErrNotFound
}
func (f *fakeRepo) UpdateStatus(id, status string) error {
	s, ok := f.subs[id]
	if !ok {
		return subscriptions.ErrNotFound
	}
	s.Status = status
	return nil
}
func (f *fakeRepo) ListBySite(siteID string, status string, limit, offset int) ([]*subscriptions.Subscription, error) {
	out := []*subscriptions.Subscription{}
	for _, s := range f.subs {
		if s.SiteID == siteID && (status == "" || s.Status == status) {
			out = append(out, s)
		}
	}
	return out, nil
}

func newSubscriptionService() (*subscriptions.Service, *fakeRepo) {
	repo := &fakeRepo{subs: map[string]*subscriptions.Subscription{}, byVehicle: map[string]*subscriptions.Subscription{}}
	return subscriptions.NewService(repo), repo
}

/* ---------- tests ---------- */

func TestCreateSubscription_OK(t *testing.T) {
	s, _ := newSubscriptionService()
	now := time.Now()
	sub, err := s.Create("customer-1", "vehicle-1", "site-1", "Mensual", 50000, now, now.AddDate(0, 1, 0))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sub.PlanName != "Mensual" || sub.PriceCents != 50000 || sub.Status != subscriptions.SubActive {
		t.Fatalf("datos inesperados: %+v", sub)
	}
}

func TestCreateSubscription_MissingFields(t *testing.T) {
	s, _ := newSubscriptionService()
	if _, err := s.Create("", "vehicle-1", "site-1", "Mensual", 50000, time.Now(), time.Now()); err == nil {
		t.Fatal("customerID vacío debe fallar")
	}
	if _, err := s.Create("customer-1", "", "site-1", "Mensual", 50000, time.Now(), time.Now()); err == nil {
		t.Fatal("vehicleID vacío debe fallar")
	}
	if _, err := s.Create("customer-1", "vehicle-1", "", "Mensual", 50000, time.Now(), time.Now()); err == nil {
		t.Fatal("siteID vacío debe fallar")
	}
	if _, err := s.Create("customer-1", "vehicle-1", "site-1", "", 50000, time.Now(), time.Now()); err == nil {
		t.Fatal("planName vacío debe fallar")
	}
}

func TestGetAndCancelSubscription(t *testing.T) {
	s, _ := newSubscriptionService()
	now := time.Now()
	sub, _ := s.Create("customer-1", "vehicle-1", "site-1", "Mensual", 50000, now, now.AddDate(0, 1, 0))
	got, err := s.Get(sub.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != sub.ID {
		t.Fatalf("ID inesperado: %s", got.ID)
	}
	if err := s.Cancel(sub.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	got, err = s.Get(sub.ID)
	if err != nil {
		t.Fatalf("Get after cancel: %v", err)
	}
	if got.Status != subscriptions.SubCancelled {
		t.Fatalf("estado debería ser cancelled, obtuvo %s", got.Status)
	}
}

func TestListBySite(t *testing.T) {
	s, _ := newSubscriptionService()
	now := time.Now()
	if _, err := s.Create("customer-1", "vehicle-1", "site-1", "Mensual", 50000, now, now.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("customer-2", "vehicle-2", "site-2", "Mensual", 50000, now, now.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListBySite("site-1", subscriptions.SubActive, 10, 0)
	if err != nil {
		t.Fatalf("ListBySite: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("se esperaba 1 suscripción para site-1, obtuvo %d", len(list))
	}
}

func TestValidateSubscription(t *testing.T) {
	s, _ := newSubscriptionService()
	now := time.Now()
	_, _ = s.Create("customer-1", "vehicle-1", "site-1", "Mensual", 50000, now.Add(-time.Hour), now.Add(time.Hour))
	result, err := s.Validate("vehicle-1", "site-1", now)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.VehicleID != "vehicle-1" || result.Status != subscriptions.SubActive {
		t.Fatalf("validación inesperada: %+v", result)
	}
}
