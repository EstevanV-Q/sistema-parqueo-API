// Package edge contiene el bootstrap y las piezas específicas del API local.
package edge

import (
	"database/sql"
	"fmt"
	"time"

	"parking-backend/internal/auth"
	"parking-backend/internal/catalog"
	"parking-backend/internal/config"
	"parking-backend/internal/customers"
	"parking-backend/internal/devices"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/rateplans"
	"parking-backend/internal/subscriptions"
	"parking-backend/internal/vehicles"
)

// SeedResult identifica los datos creados o reutilizados en el borde.
type SeedResult struct {
	SiteID     string
	SiteName   string
	AdminID    string
	OperatorID string
	AuditorID  string
	VehicleID  string
	DeviceID   string
	RatePlanID string
}

// Seed crea los datos mínimos para que una sede pueda operar sin conexión.
// Es idempotente por código de sede, usuario, documento, placa y nombre.
func Seed(database *sql.DB, cfg *config.Config) (*SeedResult, error) {
	if cfg.SeedAdminPassword == "" || cfg.SeedOperatorPassword == "" || cfg.SeedAuditorPassword == "" {
		return nil, fmt.Errorf("SEED_ADMIN_PASSWORD, SEED_OPERATOR_PASSWORD y SEED_AUDITOR_PASSWORD son requeridos")
	}
	if len(cfg.SeedDeviceAPIKey) < 16 {
		return nil, fmt.Errorf("SEED_DEVICE_API_KEY es requerido y debe tener al menos 16 caracteres")
	}

	siteRepo := catalog.NewSQLiteSiteRepo(database)
	zoneRepo := catalog.NewSQLiteZoneRepo(database)
	spaceRepo := catalog.NewSQLiteSpaceRepo(database)
	vtRepo := catalog.NewSQLiteVehicleTypeRepo(database)
	catalogSvc := catalog.NewService(siteRepo, zoneRepo, spaceRepo, vtRepo)
	authRepo := auth.NewSQLiteRepository(database)
	authSvc := auth.NewService(authRepo, cfg)
	rateplansSvc := rateplans.NewService(rateplans.NewSQLiteRepository(database))
	customersRepo := customers.NewSQLiteRepository(database)
	customersSvc := customers.NewService(customersRepo)
	vehiclesSvc := vehicles.NewService(vehicles.NewSQLiteRepository(database))
	subscriptionsRepo := subscriptions.NewSQLiteRepository(database)
	devicesSvc := devices.NewService(devices.NewSQLiteRepository(database))

	result := &SeedResult{SiteName: cfg.SeedSiteName}

	site, err := siteRepo.ByCode(cfg.SeedSiteCode)
	if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if site == nil {
		site, err = catalogSvc.CreateSite(cfg.SeedSiteName, cfg.SeedSiteCode, cfg.SiteTimezone, "CRC", 25)
		if err != nil {
			return nil, fmt.Errorf("crear sede: %w", err)
		}
	}
	result.SiteID = site.ID

	result.AdminID, err = ensureUser(authRepo, authSvc, "admin", "admin@parqueo.local", cfg.SeedAdminPassword, auth.RoleAdmin, nil)
	if err != nil {
		return nil, err
	}
	result.OperatorID, err = ensureUser(authRepo, authSvc, "operador", "operador@parqueo.local", cfg.SeedOperatorPassword, auth.RoleOperator, &site.ID)
	if err != nil {
		return nil, err
	}
	result.AuditorID, err = ensureUser(authRepo, authSvc, "auditor", "auditor@parqueo.local", cfg.SeedAuditorPassword, auth.RoleAuditor, nil)
	if err != nil {
		return nil, err
	}

	zone, err := ensureZone(zoneRepo, catalogSvc, site.ID, "General - Carros", "car", 20)
	if err != nil {
		return nil, err
	}
	if err := ensureSpaces(spaceRepo, catalogSvc, zone.ID, "A", "car", 20); err != nil {
		return nil, err
	}
	motoZone, err := ensureZone(zoneRepo, catalogSvc, site.ID, "Motos", "moto", 5)
	if err != nil {
		return nil, err
	}
	if err := ensureSpaces(spaceRepo, catalogSvc, motoZone.ID, "M", "moto", 5); err != nil {
		return nil, err
	}

	vehicleType, err := vtRepo.ByCode(cfg.DefaultVehicleTypeCode)
	if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if vehicleType == nil {
		vehicleType, err = catalogSvc.CreateVehicleType("Carro", cfg.DefaultVehicleTypeCode)
		if err != nil {
			return nil, err
		}
	}

	plan, err := ensureRatePlan(rateplansSvc, site.ID, vehicleType.ID, "Estándar Carro", 100000)
	if err != nil {
		return nil, err
	}
	result.RatePlanID = plan.ID

	motoType, err := vtRepo.ByCode("moto")
	if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if motoType == nil {
		motoType, err = catalogSvc.CreateVehicleType("Moto", "moto")
		if err != nil {
			return nil, err
		}
	}
	if _, err := ensureRatePlan(rateplansSvc, site.ID, motoType.ID, "Estándar Moto", 50000); err != nil {
		return nil, err
	}

	customer, err := ensureCustomer(customersRepo, customersSvc, "12345678", "Juan Pérez", "3001234567", "juan@email.com")
	if err != nil {
		return nil, err
	}
	vehicle, err := ensureVehicle(vehiclesSvc, cfg.SeedVehiclePlate, vehicleType.ID, customer.ID)
	if err != nil {
		return nil, err
	}
	result.VehicleID = vehicle.ID

	if existing, err := subscriptionsRepo.ActiveByVehicleSite(vehicle.ID, site.ID, time.Now().UTC()); err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return nil, err
	} else if existing == nil {
		now := time.Now().UTC()
		if err := subscriptionsRepo.Create(&subscriptions.Subscription{
			ID:         ids.UUIDv7String(),
			CustomerID: customer.ID,
			VehicleID:  vehicle.ID,
			SiteID:     site.ID,
			PlanName:   "Mensual Carro",
			PriceCents: 3000000,
			StartsAt:   now,
			ExpiresAt:  now.AddDate(0, 1, 0),
			Status:     subscriptions.SubActive,
		}); err != nil {
			return nil, fmt.Errorf("crear abono: %w", err)
		}
	}

	existingDevices, err := devicesSvc.ListBySite(site.ID)
	if err != nil {
		return nil, err
	}
	for _, d := range existingDevices {
		if d.Name == "Barrera Principal" {
			result.DeviceID = d.ID
			return result, nil
		}
	}
	device, err := devicesSvc.RegisterDevice(site.ID, "Barrera Principal", devices.DeviceBarrier, cfg.SeedDeviceAPIKey)
	if err != nil {
		return nil, err
	}
	result.DeviceID = device.ID
	return result, nil
}

func ensureUser(repo auth.Repository, svc *auth.Service, username, email, password string, role auth.Role, siteID *string) (string, error) {
	if u, err := repo.ByUsername(username); err == nil && u != nil {
		return u.ID, nil
	} else if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return "", err
	}
	created, err := svc.Register(username, email, password, role, siteID)
	if err != nil {
		if apperr.Is(err, apperr.ErrAlreadyExists) {
			u, lookupErr := repo.ByUsername(username)
			if lookupErr == nil && u != nil {
				return u.ID, nil
			}
		}
		return "", fmt.Errorf("crear usuario %s: %w", username, err)
	}
	return created.ID, nil
}

func ensureZone(repo catalog.ZoneRepository, svc *catalog.Service, siteID, name, typ string, capacity int) (*catalog.Zone, error) {
	zones, err := repo.ListBySite(siteID)
	if err != nil {
		return nil, err
	}
	for _, z := range zones {
		if z.Name == name {
			return z, nil
		}
	}
	return svc.CreateZone(siteID, name, typ, capacity)
}

// ensureSpaces crea los espacios PREFIJO-01..PREFIJO-n de una zona si no existen.
func ensureSpaces(repo catalog.SpaceRepository, svc *catalog.Service, zoneID, prefix, typ string, n int) error {
	for i := 1; i <= n; i++ {
		code := fmt.Sprintf("%s-%02d", prefix, i)
		if existing, err := repo.ByCode(zoneID, code); err == nil && existing != nil {
			continue
		} else if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
			return err
		}
		if _, err := svc.CreateSpace(zoneID, code, typ); err != nil && !apperr.Is(err, apperr.ErrAlreadyExists) {
			return err
		}
	}
	return nil
}

// ensureRatePlan crea una tarifa por hora en colones (montos en céntimos):
// 10 min de gracia, fracción de 60 min, mínimo una hora, tope diario de 12 horas
// y ticket perdido equivalente a 15 horas.
func ensureRatePlan(svc *rateplans.Service, siteID, typeID, name string, perHourCents int64) (*rateplans.RatePlan, error) {
	plans, err := svc.ListBySite(siteID)
	if err != nil {
		return nil, err
	}
	for _, p := range plans {
		if p.Name == name {
			return p, nil
		}
	}
	return svc.Create(siteID, name, "CRC", 10, 60, perHourCents, perHourCents, perHourCents*12, perHourCents*15, &typeID)
}

func ensureCustomer(repo customers.Repository, svc *customers.Service, document, name, phone, email string) (*customers.Customer, error) {
	if c, err := repo.ByDocument(document); err == nil && c != nil {
		return c, nil
	} else if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	c, err := svc.Create(document, name, &phone, &email)
	if err != nil {
		if apperr.Is(err, apperr.ErrAlreadyExists) {
			return repo.ByDocument(document)
		}
		return nil, err
	}
	return c, nil
}

func ensureVehicle(svc *vehicles.Service, plate, typeID, customerID string) (*vehicles.Vehicle, error) {
	if v, err := svc.GetByPlate(plate); err == nil && v != nil {
		return v, nil
	}
	v, err := svc.Create("", plate, typeID, &customerID)
	if err != nil {
		if apperr.Is(err, apperr.ErrAlreadyExists) {
			return svc.GetByPlate(plate)
		}
		return nil, err
	}
	return v, nil
}
