package integration

import (
	"database/sql"
	"path/filepath"
	"testing"

	"parking-backend/internal/auth"
	"parking-backend/internal/billing"
	"parking-backend/internal/catalog"
	"parking-backend/internal/config"
	"parking-backend/internal/parking"
	platformdb "parking-backend/internal/platform/db"
	"parking-backend/internal/rateplans"
	"parking-backend/internal/seed"
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

func TestSQLiteTicketLifecycle(t *testing.T) {
	database := openSQLite(t)

	siteRepo := catalog.NewSiteRepo(database)
	zoneRepo := catalog.NewZoneRepo(database)
	spaceRepo := catalog.NewSpaceRepo(database)
	vtRepo := catalog.NewVehicleTypeRepo(database)
	catalogSvc := catalog.NewService(siteRepo, zoneRepo, spaceRepo, vtRepo)

	site, err := catalogSvc.CreateSite("Sede Edge", "EDGE-01", "America/Costa_Rica", "CRC", 10)
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

	rateplansSvc := rateplans.NewService(rateplans.NewRepository(database))
	plan, err := rateplansSvc.Create(site.ID, "Tarifa Edge", "CRC", 0, 60, 1000, 1500, 20000, 25000, &vt.ID)
	if err != nil {
		t.Fatalf("CreateRatePlan: %v", err)
	}

	authSvc := auth.NewService(auth.NewRepository(database), testConfig())
	cashier, err := authSvc.Register("edge-cashier", "edge@example.com", "edge-password-123", auth.RoleOperator, &site.ID)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	parkingSvc := parking.NewService(parking.NewRepository(database, false)).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc)

	ticket, err := parkingSvc.CreateTicket(site.ID, zone.ID, space.ID, "", "EDGE123", "EDGE123", vt.ID, plan.ID)
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if ticket.Status != parking.TicketOpen {
		t.Fatalf("status = %s", ticket.Status)
	}

	stored, err := parking.NewRepository(database, false).ByID(ticket.ID)
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

	events, err := parking.NewRepository(database, false).Events(ticket.ID)
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

}

func TestSQLiteCancelRecordsEvent(t *testing.T) {
	database := openSQLite(t)

	siteRepo := catalog.NewSiteRepo(database)
	zoneRepo := catalog.NewZoneRepo(database)
	spaceRepo := catalog.NewSpaceRepo(database)
	vtRepo := catalog.NewVehicleTypeRepo(database)
	catalogSvc := catalog.NewService(siteRepo, zoneRepo, spaceRepo, vtRepo)

	site, _ := catalogSvc.CreateSite("Sede Edge", "EDGE-02", "America/Costa_Rica", "CRC", 10)
	zone, _ := catalogSvc.CreateZone(site.ID, "Zona", "car", 10)
	space, _ := catalogSvc.CreateSpace(zone.ID, "E-01", "car")
	vt, _ := catalogSvc.CreateVehicleType("Carro", "car")
	rateplansSvc := rateplans.NewService(rateplans.NewRepository(database))
	plan, _ := rateplansSvc.Create(site.ID, "Tarifa", "CRC", 0, 60, 1000, 1500, 20000, 25000, &vt.ID)

	authSvc := auth.NewService(auth.NewRepository(database), testConfig())
	operator, err := authSvc.Register("edge-operator", "operator@example.com", "edge-password-123", auth.RoleOperator, &site.ID)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	parkingSvc := parking.NewService(parking.NewRepository(database, false)).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc)

	ticket, err := parkingSvc.CreateTicket(site.ID, zone.ID, space.ID, "", "CANCEL1", "CANCEL1", vt.ID, plan.ID)
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if _, err := parkingSvc.Cancel(ticket.ID, operator.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	events, err := parking.NewRepository(database, false).Events(ticket.ID)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 2 || events[1].EventType != "cancel" {
		t.Fatalf("eventos = %+v", events)
	}
}

func TestSQLitePaymentIdempotencyKey(t *testing.T) {
	database := openSQLite(t)
	svc := billing.NewService(billing.NewRepository(database))

	key := "idem-sqlite-001"
	first, err := svc.RegisterPayment(nil, nil, 100, "CRC", "cash", "", nil, &key)
	if err != nil {
		t.Fatalf("primer pago: %v", err)
	}
	second, err := svc.RegisterPayment(nil, nil, 100, "CRC", "cash", "", nil, &key)
	if err != nil {
		t.Fatalf("pago idempotente repetido: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("idempotencia devolvió pagos distintos: %s != %s", second.ID, first.ID)
	}
}

func TestSQLiteSeedAndAutoAssignSpace(t *testing.T) {
	database := openSQLite(t)
	cfg := testConfig()
	cfg.SiteTimezone = "America/Costa_Rica"
	cfg.SeedSiteCode = "TEST-01"
	cfg.SeedSiteName = "Sede Test"
	cfg.SeedAdminPassword = "AdminTest123!"
	cfg.SeedOperatorPassword = "OperTest123!"
	cfg.SeedAuditorPassword = "AuditTest123!"
	cfg.SeedVehiclePlate = "XYZ987"

	res, err := seed.Run(database, cfg)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	// El seed es idempotente.
	if _, err := seed.Run(database, cfg); err != nil {
		t.Fatalf("seed repetido: %v", err)
	}

	vtRepo := catalog.NewVehicleTypeRepo(database)
	catalogSvc := catalog.NewService(catalog.NewSiteRepo(database), catalog.NewZoneRepo(database), catalog.NewSpaceRepo(database), vtRepo)
	moto, err := vtRepo.ByCode("moto")
	if err != nil {
		t.Fatalf("tipo moto: %v", err)
	}
	free, err := catalogSvc.FreeSpaces(res.SiteID, moto.ID)
	if err != nil {
		t.Fatalf("FreeSpaces: %v", err)
	}
	if len(free) != 5 {
		t.Fatalf("espacios de moto libres = %d, quiere 5", len(free))
	}
	space, err := catalogSvc.GetSpace(free[0].SpaceID)
	if err != nil || space.Code != "M-01" {
		t.Fatalf("primer espacio de moto = %+v (%v), quiere M-01", space, err)
	}
}
