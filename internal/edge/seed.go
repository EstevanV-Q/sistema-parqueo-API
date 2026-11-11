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
	"parking-backend/internal/platform/security"
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
	if cfg.Env == "production" && !cfg.AllowSeed {
		return nil, fmt.Errorf("el seed está bloqueado en producción; define ALLOW_SEED=true solo si es imprescindible")
	}
	if cfg.SeedAdminPassword == "" || cfg.SeedOperatorPassword == "" || cfg.SeedAuditorPassword == "" {
		return nil, fmt.Errorf("SEED_ADMIN_PASSWORD, SEED_OPERATOR_PASSWORD y SEED_AUDITOR_PASSWORD son requeridos")
	}
	for name, password := range map[string]string{
		"SEED_ADMIN_PASSWORD":    cfg.SeedAdminPassword,
		"SEED_OPERATOR_PASSWORD": cfg.SeedOperatorPassword,
		"SEED_AUDITOR_PASSWORD":  cfg.SeedAuditorPassword,
	} {
		if err := security.ValidateSeedPassword(password); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	if len(cfg.SeedDeviceAPIKey) < 16 {
		return nil, fmt.Errorf("SEED_DEVICE_API_KEY es requerido y debe tener al menos 16 caracteres")
	}
	if err := security.ValidateSeedPassword(cfg.SeedDeviceAPIKey); err != nil {
		return nil, fmt.Errorf("SEED_DEVICE_API_KEY: %w", err)
	}
	security.ConfigureTokenPepper(cfg.TokenPepper)

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
		site, err = catalogSvc.CreateSite(cfg.SeedSiteName, cfg.SeedSiteCode, cfg.SiteTimezone, "COP", 50)
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

	zone, err := ensureZone(zoneRepo, catalogSvc, site.ID)
	if err != nil {
		return nil, err
	}
	for i := 1; i <= 10; i++ {
		code := fmt.Sprintf("A-%02d", i)
		if existing, err := spaceRepo.ByCode(zone.ID, code); err == nil && existing != nil {
			continue
		} else if err != nil && !apperr.Is(err, apperr.ErrNotFound) {
			return nil, err
		}
		if _, err := catalogSvc.CreateSpace(zone.ID, code, "car"); err != nil && !apperr.Is(err, apperr.ErrAlreadyExists) {
			return nil, err
		}
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

	plan, err := ensureRatePlan(rateplansSvc, site.ID, vehicleType.ID)
	if err != nil {
		return nil, err
	}
	result.RatePlanID = plan.ID

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
			PriceCents: 50000,
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
		hash, hashErr := security.HashPassword(password)
		if hashErr != nil {
			return "", hashErr
		}
		if err := repo.UpdatePasswordHash(u.ID, hash); err != nil {
			return "", fmt.Errorf("actualizar contraseña de %s: %w", username, err)
		}
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

func ensureZone(repo catalog.ZoneRepository, svc *catalog.Service, siteID string) (*catalog.Zone, error) {
	zones, err := repo.ListBySite(siteID)
	if err != nil {
		return nil, err
	}
	for _, z := range zones {
		if z.Name == "General - Carros" {
			return z, nil
		}
	}
	return svc.CreateZone(siteID, "General - Carros", "car", 30)
}

func ensureRatePlan(svc *rateplans.Service, siteID, typeID string) (*rateplans.RatePlan, error) {
	plans, err := svc.ListBySite(siteID)
	if err != nil {
		return nil, err
	}
	for _, p := range plans {
		if p.Name == "Estándar Carro" {
			return p, nil
		}
	}
	return svc.Create(siteID, "Estándar Carro", "COP", 10, 60, 1000, 1500, 20000, 25000, &typeID)
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
