package devices_test

import (
	"errors"
	"testing"
	"time"

	"parking-backend/internal/devices"
	"parking-backend/internal/platform/security"
)

/* ---------- fakes ---------- */

type fakeRepo struct {
	devices  map[string]*devices.Device
	byAPIKey map[string]*devices.Device
	events   map[string]*devices.DeviceEvent
}

func (f *fakeRepo) Create(d *devices.Device) error {
	if d.ID == "" {
		d.ID = "device-" + d.Name
	}
	f.devices[d.ID] = d
	f.byAPIKey[d.APIKeyHash] = d
	return nil
}
func (f *fakeRepo) ByID(id string) (*devices.Device, error) {
	d, ok := f.devices[id]
	if !ok {
		return nil, devices.ErrNotFound
	}
	return d, nil
}
func (f *fakeRepo) ByAPIKey(apiKeyHash string) (*devices.Device, error) {
	d, ok := f.byAPIKey[apiKeyHash]
	if !ok {
		return nil, devices.ErrNotFound
	}
	return d, nil
}
func (f *fakeRepo) UpdateHeartbeat(id string, at time.Time) error {
	if _, ok := f.devices[id]; !ok {
		return devices.ErrNotFound
	}
	return nil
}
func (f *fakeRepo) CreateEvent(e *devices.DeviceEvent) error {
	if e.ID == "" {
		e.ID = "event-" + e.EventID
	}
	f.events[e.ID] = e
	return nil
}
func (f *fakeRepo) EventByID(eventID string) (*devices.DeviceEvent, error) {
	for _, e := range f.events {
		if e.EventID == eventID {
			return e, nil
		}
	}
	return nil, devices.ErrNotFound
}
func (f *fakeRepo) ListBySite(siteID string) ([]*devices.Device, error) {
	out := []*devices.Device{}
	for _, d := range f.devices {
		if d.SiteID == siteID {
			out = append(out, d)
		}
	}
	return out, nil
}

type fakeGate struct {
	allow  bool
	reason string
	err    error
}

func (f *fakeGate) AuthorizeEntry(siteID, plateNormalized string) (bool, string, error) {
	return f.allow, f.reason, f.err
}
func (f *fakeGate) AuthorizeExit(siteID, plateNormalized string) (bool, string, error) {
	return f.allow, f.reason, f.err
}

type fakeTicketCreator struct {
	err error
}

func (f *fakeTicketCreator) AutoEnter(siteID, plateNormalized string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "ticket-123", nil
}

type fakePublisher struct{}

func (f *fakePublisher) Publish(siteID, event string, payload map[string]any) {}

func newDevicesService() *devices.Service {
	return devices.NewService(&fakeRepo{
		devices:  map[string]*devices.Device{},
		byAPIKey: map[string]*devices.Device{},
		events:   map[string]*devices.DeviceEvent{},
	}).
		WithGate(&fakeGate{allow: true}).
		WithTicketCreator(&fakeTicketCreator{}).
		WithPublisher(&fakePublisher{})
}

/* ---------- tests ---------- */

func TestRegisterDevice_OK(t *testing.T) {
	s := newDevicesService()
	d, err := s.RegisterDevice("site-1", "Barrera Norte", "barrier", "device-api-key-001")
	if err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}
	if d.Name != "Barrera Norte" || d.Type != "barrier" || !d.IsActive {
		t.Fatalf("datos inesperados: %+v", d)
	}
	if d.APIKeyHash == "device-api-key-001" {
		t.Fatal("la API key no debería guardarse en texto plano")
	}
}

func TestRegisterDevice_InvalidType(t *testing.T) {
	s := newDevicesService()
	if _, err := s.RegisterDevice("site-1", "Dispositivo X", "tipo-invalido", "key"); err == nil {
		t.Fatal("tipo inválido debe fallar")
	}
}

func TestRegisterDevice_MissingFields(t *testing.T) {
	s := newDevicesService()
	if _, err := s.RegisterDevice("", "Nombre", "barrier", "key"); err == nil {
		t.Fatal("siteID vacío debe fallar")
	}
	if _, err := s.RegisterDevice("site-1", "", "barrier", "key"); err == nil {
		t.Fatal("name vacío debe fallar")
	}
	if _, err := s.RegisterDevice("site-1", "Nombre", "", "key"); err == nil {
		t.Fatal("type vacío debe fallar")
	}
	if _, err := s.RegisterDevice("site-1", "Nombre", "barrier", ""); err == nil {
		t.Fatal("apiKey vacío debe fallar")
	}
}

func TestAuthenticate_OK(t *testing.T) {
	s := newDevicesService()
	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "device-api-key-001")
	auth, err := s.Authenticate("device-api-key-001")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if auth.ID != d.ID {
		t.Fatalf("ID inesperado: %s", auth.ID)
	}
}

func TestAuthenticate_WrongKey(t *testing.T) {
	s := newDevicesService()
	if _, err := s.Authenticate("clave-incorrecta"); err == nil {
		t.Fatal("clave incorrecta debe fallar")
	} else if err != devices.ErrDeviceUnauthorized {
		t.Fatalf("se esperaba ErrDeviceUnauthorized, obtuvo %v", err)
	}
}

func TestProcessEvent_EntryDetected(t *testing.T) {
	s := newDevicesService()
	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "key")
	event, err := s.ProcessEvent(d, "evt-001", devices.EventEntryDetected, "XYZ987")
	if err != nil {
		t.Fatalf("ProcessEvent: %v", err)
	}
	if event.Decision != devices.DecisionAllow {
		t.Fatalf("decisión inesperada: %s", event.Decision)
	}
	if event.PlateNormalized == nil || *event.PlateNormalized != "XYZ987" {
		t.Fatalf("placa inesperada: %v", event.PlateNormalized)
	}
}

func TestProcessEvent_EntryDetectedFailsClosedWhenAutoEnterFails(t *testing.T) {
	repo := &fakeRepo{
		devices:  map[string]*devices.Device{},
		byAPIKey: map[string]*devices.Device{},
		events:   map[string]*devices.DeviceEvent{},
	}
	s := devices.NewService(repo).
		WithGate(&fakeGate{allow: true}).
		WithTicketCreator(&fakeTicketCreator{err: errors.New("tipo de vehículo no resuelto")})

	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "key")
	if _, err := s.ProcessEvent(d, "evt-auto-fail", devices.EventEntryDetected, "XYZ987"); err == nil {
		t.Fatal("la entrada debe fallar si AutoEnter falla")
	}
	if len(repo.events) != 0 {
		t.Fatalf("no debe registrar un evento allow; hay %d", len(repo.events))
	}
}

func TestProcessEvent_ExitDetected(t *testing.T) {
	s := newDevicesService()
	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "key")
	event, err := s.ProcessEvent(d, "evt-002", devices.EventExitDetected, "XYZ987")
	if err != nil {
		t.Fatalf("ProcessEvent: %v", err)
	}
	if event.Decision != devices.DecisionAllow {
		t.Fatalf("decisión inesperada: %s", event.Decision)
	}
}

func TestProcessEvent_Heartbeat(t *testing.T) {
	s := newDevicesService()
	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "key")
	event, err := s.ProcessEvent(d, "evt-003", devices.EventHeartbeat, "")
	if err != nil {
		t.Fatalf("ProcessEvent: %v", err)
	}
	if event.Decision != devices.DecisionAllow {
		t.Fatalf("decisión inesperada: %s", event.Decision)
	}
}

func TestProcessEvent_CommandAck(t *testing.T) {
	s := newDevicesService()
	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "key")
	event, err := s.ProcessEvent(d, "evt-004", devices.EventCommandAck, "")
	if err != nil {
		t.Fatalf("ProcessEvent: %v", err)
	}
	if event.Decision != devices.DecisionAllow {
		t.Fatalf("decisión inesperada: %s", event.Decision)
	}
}

func TestProcessEvent_InvalidEventType(t *testing.T) {
	s := newDevicesService()
	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "key")
	if _, err := s.ProcessEvent(d, "evt-005", "tipo-invalido", ""); err == nil {
		t.Fatal("tipo de evento inválido debe fallar")
	} else if err != devices.ErrInvalidEventType {
		t.Fatalf("se esperaba ErrInvalidEventType, obtuvo %v", err)
	}
}

func TestProcessEvent_MissingEventID(t *testing.T) {
	s := newDevicesService()
	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "key")
	if _, err := s.ProcessEvent(d, "", devices.EventHeartbeat, ""); err == nil {
		t.Fatal("eventID vacío debe fallar")
	}
}

func TestProcessEvent_IdempotentByEventID(t *testing.T) {
	s := newDevicesService()
	d, _ := s.RegisterDevice("site-1", "Barrera", "barrier", "key")
	e1, err := s.ProcessEvent(d, "evt-dup", devices.EventEntryDetected, "XYZ987")
	if err != nil {
		t.Fatalf("ProcessEvent: %v", err)
	}
	e2, err := s.ProcessEvent(d, "evt-dup", devices.EventEntryDetected, "XYZ987")
	if err != nil {
		t.Fatalf("ProcessEvent: %v", err)
	}
	if e1.ID != e2.ID {
		t.Fatalf("se esperaba el mismo evento por event_id duplicado")
	}
}

func TestProcessEvent_NilDevice(t *testing.T) {
	s := newDevicesService()
	if _, err := s.ProcessEvent(nil, "evt-006", devices.EventHeartbeat, ""); err == nil {
		t.Fatal("device nil debe fallar")
	} else if err != devices.ErrDeviceUnauthorized {
		t.Fatalf("se esperaba ErrDeviceUnauthorized, obtuvo %v", err)
	}
}

func TestHashTokenProducesHash(t *testing.T) {
	hash := security.HashToken("clave-secreta")
	if hash == "" {
		t.Fatal("HashToken no debería devolver vacío")
	}
	if hash == "clave-secreta" {
		t.Fatal("HashToken no debería devolver el texto plano")
	}
}
