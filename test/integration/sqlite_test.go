package integration

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"parking-backend/internal/auth"
	"parking-backend/internal/billing"
	"parking-backend/internal/catalog"
	"parking-backend/internal/config"
	"parking-backend/internal/devices"
	"parking-backend/internal/edge"
	"parking-backend/internal/parking"
	platformdb "parking-backend/internal/platform/db"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/outbox"
	"parking-backend/internal/rateplans"
	"parking-backend/internal/subscriptions"
)

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:   "test-secret-at-least-32-characters-long",
		JWTIssuer:   "parking-test",
		JWTAudience: "parking-test",
	}
}

func openSQLite(t *testing.T) *sql.DB {
	t.Helper()
	database, err := platformdb.OpenSQLite(filepath.Join(t.TempDir(), "edge.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestSQLiteTicketLifecycleAndOutbox(t *testing.T) {
	database := openSQLite(t)

	siteRepo := catalog.NewSQLiteSiteRepo(database)
	zoneRepo := catalog.NewSQLiteZoneRepo(database)
	spaceRepo := catalog.NewSQLiteSpaceRepo(database)
	vtRepo := catalog.NewSQLiteVehicleTypeRepo(database)
	catalogSvc := catalog.NewService(siteRepo, zoneRepo, spaceRepo, vtRepo)

	site, err := catalogSvc.CreateSite("Sede Edge", "EDGE-01", "America/Bogota", "COP", 10)
	if err != nil {
		t.Fatalf("CreateSite: %v", err)
	}
	zone, err := catalogSvc.CreateZone(site.ID, "Zona Edge", "car", 10)
	if err != nil {
		t.Fatalf("CreateZone: %v", err)
	}
	space, err := catalogSvc.CreateSpace(zone.ID, "E-01", "car")
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}
	vt, err := catalogSvc.CreateVehicleType("Carro Edge", "car")
	if err != nil {
		t.Fatalf("CreateVehicleType: %v", err)
	}

	rateplansSvc := rateplans.NewService(rateplans.NewSQLiteRepository(database))
	plan, err := rateplansSvc.Create(site.ID, "Tarifa Edge", "COP", 0, 60, 1000, 1500, 20000, 25000, &vt.ID)
	if err != nil {
		t.Fatalf("CreateRatePlan: %v", err)
	}

	authSvc := auth.NewService(auth.NewSQLiteRepository(database), testConfig())
	cashier, err := authSvc.Register("edge-cashier", "edge@example.com", "edge-password-123", auth.RoleOperator, &site.ID)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	parkingSvc := parking.NewService(parking.NewSQLiteRepository(database)).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc).
		WithVehicleTypeResolver(catalogSvc)

	ticket, err := parkingSvc.CreateTicket(site.ID, zone.ID, space.ID, "", "EDGE123", "EDGE123", vt.ID, plan.ID)
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if ticket.Status != parking.TicketOpen {
		t.Fatalf("status = %s", ticket.Status)
	}

	stored, err := parking.NewSQLiteRepository(database).ByID(ticket.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if stored.PlateNormalized != "EDGE123" {
		t.Fatalf("plate = %s", stored.PlateNormalized)
	}

	paid, err := parkingSvc.Settle(ticket.ID, 1000, "cash", cashier.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if paid.Status != parking.TicketPaid {
		t.Fatalf("settled status = %s", paid.Status)
	}

	events, err := parking.NewSQLiteRepository(database).Events(ticket.ID)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 2 || events[0].EventType != "entry" || events[1].EventType != "exit" {
		t.Fatalf("eventos = %+v", events)
	}
	if events[1].ActorUserID == nil || *events[1].ActorUserID != cashier.ID {
		t.Fatalf("actor de salida = %+v", events[1].ActorUserID)
	}

	spaceAfter, err := spaceRepo.ByID(space.ID)
	if err != nil {
		t.Fatalf("space ByID: %v", err)
	}
	if spaceAfter.Status != "free" {
		t.Fatalf("espacio = %s, quiere free", spaceAfter.Status)
	}

	pending, err := outbox.NewStore(database).Pending(50)
	if err != nil {
		t.Fatalf("outbox Pending: %v", err)
	}
	ticketEvents := 0
	for _, event := range pending {
		if event.AggregateType == "ticket" {
			ticketEvents++
		}
	}
	if ticketEvents != 2 {
		t.Fatalf("eventos de ticket en outbox = %d, quiere 2 (entrada y liquidación)", ticketEvents)
	}
}

func TestSQLiteAutoEnterFromDeviceEvent(t *testing.T) {
	database := openSQLite(t)
	cfg := testConfig()
	cfg.DefaultVehicleTypeCode = "car"
	cfg.SiteTimezone = "America/Bogota"
	cfg.SeedSiteCode = "EDGE-LPR"
	cfg.SeedSiteName = "Sede LPR"
	cfg.SeedAdminPassword = "AdminEdge123!"
	cfg.SeedOperatorPassword = "OperEdge123!"
	cfg.SeedAuditorPassword = "AuditEdge123!"
	cfg.SeedDeviceAPIKey = "edge-device-key-123456"
	cfg.SeedVehiclePlate = "XYZ987"

	if _, err := edge.Seed(database, cfg); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	siteRepo := catalog.NewSQLiteSiteRepo(database)
	site, err := siteRepo.ByCode(cfg.SeedSiteCode)
	if err != nil {
		t.Fatalf("site: %v", err)
	}
	catalogSvc := catalog.NewService(
		siteRepo,
		catalog.NewSQLiteZoneRepo(database),
		catalog.NewSQLiteSpaceRepo(database),
		catalog.NewSQLiteVehicleTypeRepo(database),
	)
	rateplansSvc := rateplans.NewService(rateplans.NewSQLiteRepository(database))
	parkingSvc := parking.NewService(parking.NewSQLiteRepository(database)).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc).
		WithSubscriptionChecker(subscriptions.NewService(subscriptions.NewSQLiteRepository(database))).
		WithVehicleTypeResolver(catalogSvc).
		WithDefaultVehicleType(cfg.DefaultVehicleTypeCode)
	devicesSvc := devices.NewService(devices.NewSQLiteRepository(database)).
		WithGate(parkingSvc).
		WithTicketCreator(parkingSvc)

	device, err := devicesSvc.Authenticate(cfg.SeedDeviceAPIKey)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	event, err := devicesSvc.ProcessEvent(device, ids.UUIDv7String(), "entry_detected", "AUTOEDGE1")
	if err != nil {
		t.Fatalf("ProcessEvent: %T %v", err, err)
	}
	if event.Decision != devices.DecisionAllow {
		t.Fatalf("decision = %s", event.Decision)
	}
	if device.SiteID != site.ID {
		t.Fatalf("device site = %s, want %s", device.SiteID, site.ID)
	}
	ticket, err := parking.NewSQLiteRepository(database).OpenByPlateSite("AUTOEDGE1", site.ID)
	if err != nil {
		t.Fatalf("OpenByPlateSite: %v", err)
	}
	if ticket.Status != parking.TicketOpen {
		t.Fatalf("ticket = %+v", ticket)
	}
}

func TestSQLiteCancelWritesOutboxAndEvent(t *testing.T) {
	database := openSQLite(t)

	siteRepo := catalog.NewSQLiteSiteRepo(database)
	zoneRepo := catalog.NewSQLiteZoneRepo(database)
	spaceRepo := catalog.NewSQLiteSpaceRepo(database)
	vtRepo := catalog.NewSQLiteVehicleTypeRepo(database)
	catalogSvc := catalog.NewService(siteRepo, zoneRepo, spaceRepo, vtRepo)

	site, _ := catalogSvc.CreateSite("Sede Edge", "EDGE-02", "America/Bogota", "COP", 10)
	zone, _ := catalogSvc.CreateZone(site.ID, "Zona", "car", 10)
	space, _ := catalogSvc.CreateSpace(zone.ID, "E-01", "car")
	vt, _ := catalogSvc.CreateVehicleType("Carro", "car")
	rateplansSvc := rateplans.NewService(rateplans.NewSQLiteRepository(database))
	plan, _ := rateplansSvc.Create(site.ID, "Tarifa", "COP", 0, 60, 1000, 1500, 20000, 25000, &vt.ID)

	authSvc := auth.NewService(auth.NewSQLiteRepository(database), testConfig())
	operator, err := authSvc.Register("edge-operator", "operator@example.com", "edge-password-123", auth.RoleOperator, &site.ID)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	parkingSvc := parking.NewService(parking.NewSQLiteRepository(database)).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc).
		WithVehicleTypeResolver(catalogSvc)

	ticket, err := parkingSvc.CreateTicket(site.ID, zone.ID, space.ID, "", "CANCEL1", "CANCEL1", vt.ID, plan.ID)
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if _, err := parkingSvc.Cancel(ticket.ID, operator.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	events, err := parking.NewSQLiteRepository(database).Events(ticket.ID)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 2 || events[1].EventType != "cancel" {
		t.Fatalf("eventos = %+v", events)
	}
	pending, err := outbox.NewStore(database).Pending(50)
	if err != nil {
		t.Fatalf("outbox Pending: %v", err)
	}
	ticketEvents := 0
	for _, event := range pending {
		if event.AggregateType == "ticket" {
			ticketEvents++
		}
	}
	if ticketEvents != 2 {
		t.Fatalf("eventos de ticket en outbox = %d, quiere 2", ticketEvents)
	}
}

func TestSQLitePaymentIdempotencyKey(t *testing.T) {
	database := openSQLite(t)
	svc := billing.NewService(billing.NewSQLiteRepository(database))

	key := "idem-sqlite-001"
	first, err := svc.RegisterPayment(nil, nil, 100, "COP", "cash", "", nil, &key)
	if err != nil {
		t.Fatalf("primer pago: %v", err)
	}
	second, err := svc.RegisterPayment(nil, nil, 100, "COP", "cash", "", nil, &key)
	if err != nil {
		t.Fatalf("pago idempotente repetido: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("idempotencia devolvió pagos distintos: %s != %s", second.ID, first.ID)
	}
}

func TestSQLiteRefreshTokenReuseRevokesFamily(t *testing.T) {
	database := openSQLite(t)
	cfg := testConfig()
	cfg.JWTAccess = time.Minute
	cfg.JWTRefresh = time.Hour
	svc := auth.NewService(auth.NewSQLiteRepository(database), cfg)
	if _, err := svc.Register("admin", "admin@example.com", "edge-password-123", auth.RoleAdmin, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, _, refresh, err := svc.Login("admin", "edge-password-123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	_, rotated, err := svc.Refresh(refresh)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	_, newest, err := svc.Refresh(rotated)
	if err != nil {
		t.Fatalf("Refresh rotado: %v", err)
	}
	if _, _, err := svc.Refresh(refresh); err == nil {
		t.Fatal("reutilizar el token viejo debe fallar")
	}
	if _, _, err := svc.Refresh(newest); err == nil {
		t.Fatal("la reutilización debe revocar toda la familia")
	}
}
