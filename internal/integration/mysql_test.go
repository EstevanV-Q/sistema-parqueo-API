package integration

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	"parking-backend/internal/billing"
	"parking-backend/internal/catalog"
	"parking-backend/internal/parking"
	"parking-backend/internal/rateplans"
)

// TestTicketLifecycle_MySQL recorre el ciclo completo contra un MySQL real.
//
// Se ejecuta solo si TEST_MYSQL_DSN está definido, por ejemplo:
//
//	TEST_MYSQL_DSN="parking_user:parking_pass@tcp(localhost:3306)/parking_test?parseTime=true&charset=utf8mb4" \
//	  go test ./internal/integration -v
func TestTicketLifecycle_MySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN no configurado; se omite la prueba de integración")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	if err := goose.SetDialect("mysql"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	if err := goose.Up(db, "../../migrations"); err != nil {
		t.Fatalf("goose.Up: %v", err)
	}

	cleanDatabase(t, db)

	// --- Catálogo base ---
	siteRepo := catalog.NewMySQLSiteRepo(db)
	zoneRepo := catalog.NewMySQLZoneRepo(db)
	spaceRepo := catalog.NewMySQLSpaceRepo(db)
	vtRepo := catalog.NewMySQLVehicleTypeRepo(db)
	catalogSvc := catalog.NewService(siteRepo, zoneRepo, spaceRepo, vtRepo)

	site, err := catalogSvc.CreateSite("Sede Test", "TEST-01", "America/Bogota", "COP", 10)
	if err != nil {
		t.Fatalf("CreateSite: %v", err)
	}
	zone, err := catalogSvc.CreateZone(site.ID, "Zona Test", "car", 10)
	if err != nil {
		t.Fatalf("CreateZone: %v", err)
	}
	space, err := catalogSvc.CreateSpace(zone.ID, "T-01", "car")
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}
	vt, err := catalogSvc.CreateVehicleType("Carro Test", "car")
	if err != nil {
		t.Fatalf("CreateVehicleType: %v", err)
	}

	// --- Tarifa ---
	rateplansSvc := rateplans.NewService(rateplans.NewMySQLRepository(db))
	if _, err := rateplansSvc.Create(site.ID, "Tarifa Test", "COP", 10, 60, 1000, 1500, 20000, 25000, &vt.ID); err != nil {
		t.Fatalf("CreateRatePlan: %v", err)
	}

	// --- Servicios de parking y billing ---
	billingSvc := billing.NewService(billing.NewMySQLRepository(db))
	parkingSvc := parking.NewService(parking.NewMySQLRepository(db)).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc).
		WithPaymentCreator(billingSvc)

	// --- Entrada ---
	ticket, err := parkingSvc.CreateTicket(site.ID, zone.ID, space.ID, "", "TST123", "TST-123", vt.ID, "")
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if ticket.Status != parking.TicketOpen {
		t.Fatalf("status = %s, quiere open", ticket.Status)
	}

	occupied, err := spaceRepo.ByID(space.ID)
	if err != nil {
		t.Fatalf("spaceRepo.ByID: %v", err)
	}
	if occupied.Status != "occupied" {
		t.Fatalf("estado del espacio = %s, quiere occupied", occupied.Status)
	}

	// --- Cotización y cierre ---
	_, quote, err := parkingSvc.Quote(ticket.ID)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}

	settled, err := parkingSvc.Settle(ticket.ID, int64(quote.Amount), "cash", "")
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if settled.Status != parking.TicketPaid {
		t.Fatalf("status = %s, quiere paid", settled.Status)
	}

	free, err := spaceRepo.ByID(space.ID)
	if err != nil {
		t.Fatalf("spaceRepo.ByID tras salida: %v", err)
	}
	if free.Status != "free" {
		t.Fatalf("estado del espacio = %s, quiere free", free.Status)
	}

	payment, err := billingSvc.PaymentByTicket(ticket.ID)
	if err != nil {
		t.Fatalf("PaymentByTicket: %v", err)
	}
	if payment.AmountCents != int64(quote.Amount) {
		t.Fatalf("pago = %d, quiere %d", payment.AmountCents, quote.Amount)
	}
}

func cleanDatabase(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		t.Fatalf("desactivar FK: %v", err)
	}
	defer func() {
		if _, err := db.Exec("SET FOREIGN_KEY_CHECKS = 1"); err != nil {
			t.Fatalf("reactivar FK: %v", err)
		}
	}()

	tables := []string{
		"device_events", "devices", "audit_log", "payments", "ticket_events",
		"tickets", "rate_plan_tiers", "rate_plans", "subscriptions",
		"vehicles", "customers", "spaces", "zones", "vehicle_types", "sites",
	}
	for _, table := range tables {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("limpiar %s: %v", table, err)
		}
	}
}
