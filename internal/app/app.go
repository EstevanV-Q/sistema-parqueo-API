// Package app es el composition root del backend: construye repositorios,
// servicios, handlers, middlewares y el router HTTP.
package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"parking-backend/api"
	"parking-backend/internal/audit"
	"parking-backend/internal/auth"
	"parking-backend/internal/billing"
	"parking-backend/internal/catalog"
	"parking-backend/internal/config"
	"parking-backend/internal/customers"
	"parking-backend/internal/devices"
	"parking-backend/internal/parking"
	"parking-backend/internal/platform/db"
	"parking-backend/internal/platform/httpserver"
	platmiddleware "parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/outbox"
	"parking-backend/internal/rateplans"
	"parking-backend/internal/realtime"
	"parking-backend/internal/reporting"
	"parking-backend/internal/subscriptions"
	syncer "parking-backend/internal/sync"
	"parking-backend/internal/vehicles"
)

// App agrupa las dependencias de larga vida del proceso.
type App struct {
	Config     *config.Config
	DB         *sql.DB
	Hub        *realtime.Hub
	Router     http.Handler
	syncCancel context.CancelFunc
}

// New construye la aplicación completa. El llamante es responsable de cerrar
// la base de datos con Close.
func New(cfg *config.Config) (*App, error) {
	database, err := db.Open(cfg)
	if err != nil {
		return nil, err
	}
	platmiddleware.SetAllowedOrigins(cfg.CORSAllowedOrigins)

	// --- Auditoría ---
	auditRepo := audit.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		auditRepo = audit.NewSQLiteRepository(database)
	}
	auditSvc := audit.NewService(auditRepo)
	auditHandler := audit.NewHandler(auditSvc)

	// --- Auth ---
	authRepo := auth.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		authRepo = auth.NewSQLiteRepository(database)
	}
	authSvc := auth.NewService(authRepo, cfg)
	authHandler := auth.NewHandler(authSvc)

	// --- Catálogo ---
	siteRepo := catalog.NewMySQLSiteRepo(database)
	zoneRepo := catalog.NewMySQLZoneRepo(database)
	spaceRepo := catalog.NewMySQLSpaceRepo(database)
	vehicleTypeRepo := catalog.NewMySQLVehicleTypeRepo(database)
	if cfg.IsSQLite() {
		siteRepo = catalog.NewSQLiteSiteRepo(database)
		zoneRepo = catalog.NewSQLiteZoneRepo(database)
		spaceRepo = catalog.NewSQLiteSpaceRepo(database)
		vehicleTypeRepo = catalog.NewSQLiteVehicleTypeRepo(database)
	}
	catalogSvc := catalog.NewService(siteRepo, zoneRepo, spaceRepo, vehicleTypeRepo)
	catalogHandler := catalog.NewHandler(catalogSvc)

	// --- Vehículos y clientes ---
	vehiclesRepo := vehicles.NewMySQLRepository(database)
	customersRepo := customers.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		vehiclesRepo = vehicles.NewSQLiteRepository(database)
		customersRepo = customers.NewSQLiteRepository(database)
	}
	vehiclesSvc := vehicles.NewService(vehiclesRepo)
	vehiclesHandler := vehicles.NewHandler(vehiclesSvc)

	customersSvc := customers.NewService(customersRepo)
	customersHandler := customers.NewHandler(customersSvc)

	// --- Tarifas ---
	rateplansRepo := rateplans.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		rateplansRepo = rateplans.NewSQLiteRepository(database)
	}
	rateplansSvc := rateplans.NewService(rateplansRepo)
	rateplansHandler := rateplans.NewHandler(rateplansSvc)

	// --- Pagos ---
	billingRepo := billing.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		billingRepo = billing.NewSQLiteRepository(database)
	}
	billingSvc := billing.NewService(billingRepo)

	// --- Abonados ---
	subscriptionsRepo := subscriptions.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		subscriptionsRepo = subscriptions.NewSQLiteRepository(database)
	}
	subscriptionsSvc := subscriptions.NewService(subscriptionsRepo)
	subsHandler := subscriptions.NewHandler(subscriptionsSvc, auditSvc)

	// --- Tiempo real ---
	rtHub := realtime.NewHub()
	go rtHub.Run()
	rtPublisher := realtime.NewPublisher(rtHub)
	rtHandler := realtime.NewHandler(rtHub, cfg)

	// --- Parking (depende de tarifas, catálogo, abonados, pagos y realtime) ---
	parkingRepo := parking.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		parkingRepo = parking.NewSQLiteRepository(database)
	}
	parkingSvc := parking.NewService(parkingRepo).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc).
		WithSubscriptionChecker(subscriptionsSvc).
		WithVehicleTypeResolver(catalogSvc).
		WithDefaultVehicleType(cfg.DefaultVehicleTypeCode).
		WithEventPublisher(rtPublisher)
	parkingHandler := parking.NewHandler(parkingSvc, auditSvc)

	// billingHandler se crea aquí porque necesita parkingSvc y subscriptionsSvc como SiteResolver.
	billingHandler := billing.NewHandler(billingSvc, auditSvc, &siteResolver{parking: parkingSvc, subs: subscriptionsSvc})

	// --- Dispositivos (consulta a parking y publica eventos) ---
	devicesRepo := devices.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		devicesRepo = devices.NewSQLiteRepository(database)
	}
	devicesSvc := devices.NewService(devicesRepo).
		WithGate(parkingSvc).
		WithTicketCreator(parkingSvc).
		WithPublisher(rtPublisher)
	devicesHandler := devices.NewHandler(devicesSvc)

	// --- Reportes ---
	reportingRepo := reporting.NewMySQLRepository(database)
	if cfg.IsSQLite() {
		reportingRepo = reporting.NewSQLiteRepository(database)
	}
	reportingSvc := reporting.NewService(reportingRepo)
	reportingHandler := reporting.NewHandler(reportingSvc)

	// --- Router ---
	authMW := platmiddleware.NewAuthMiddleware(cfg)
	var syncIngest *syncer.Handler
	if !cfg.IsSQLite() && cfg.SyncToken != "" {
		syncIngest = syncer.NewHandler(database, cfg.SyncToken)
	}
	r := buildRouter(database, authMW,
		authHandler, parkingHandler, billingHandler, subsHandler,
		vehiclesHandler, customersHandler, rateplansHandler,
		catalogHandler, devicesHandler, reportingHandler, auditHandler,
		rtHandler, syncIngest,
	)

	var syncCancel context.CancelFunc
	if cfg.IsSQLite() && cfg.SyncURL != "" {
		worker := syncer.NewWorker(
			outbox.NewStore(database),
			&syncer.HTTPSender{URL: cfg.SyncURL, Token: cfg.SyncToken, SiteID: cfg.EdgeSiteID},
			8,
			2*time.Second,
		)
		var syncCtx context.Context
		syncCtx, syncCancel = context.WithCancel(context.Background())
		go func() {
			if err := worker.Run(syncCtx, cfg.SyncInterval); err != nil {
				slog.Error("worker de sincronización detenido", "error", err)
			}
		}()
	}

	return &App{
		Config:     cfg,
		DB:         database,
		Hub:        rtHub,
		Router:     r,
		syncCancel: syncCancel,
	}, nil
}

// Close libera recursos.
func (a *App) Close() error {
	if a.syncCancel != nil {
		a.syncCancel()
	}
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

func buildRouter(
	database *sql.DB,
	authMW *platmiddleware.AuthMiddleware,
	authHandler *auth.Handler,
	parkingHandler *parking.Handler,
	billingHandler *billing.Handler,
	subsHandler *subscriptions.Handler,
	vehiclesHandler *vehicles.Handler,
	customersHandler *customers.Handler,
	rateplansHandler *rateplans.Handler,
	catalogHandler *catalog.Handler,
	devicesHandler *devices.Handler,
	reportingHandler *reporting.Handler,
	auditHandler *audit.Handler,
	rtHandler *realtime.Handler,
	syncIngest *syncer.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(platmiddleware.RequestIDHeader)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Logger)
	r.Use(platmiddleware.RequestInfo)
	r.Use(platmiddleware.CORS)

	// Salud
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpserver.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if err := database.PingContext(req.Context()); err != nil {
			httpserver.JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "error", "message": "database unavailable",
			})
			return
		}
		httpserver.JSON(w, http.StatusOK, map[string]string{
			"status": "ready", "database": "connected",
		})
	})

	// Documentación
	r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(api.SwaggerHTML))
	})
	r.Get("/swagger/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(api.OpenAPISpec)
	})

	r.Route("/api/v1", func(r chi.Router) {
		// ---------- Público ----------
		r.Route("/auth", func(r chi.Router) {
			r.With(platmiddleware.RateLimit(10, time.Minute)).Post("/login", authHandler.Login)
			r.With(platmiddleware.RateLimit(30, time.Minute)).Post("/refresh", authHandler.RefreshHandler)
			r.Post("/logout", authHandler.LogoutHandler)

			r.With(authMW.Handler).Get("/me", authHandler.MeHandler)
			r.With(authMW.Handler).Post("/ws-ticket", authHandler.WSTicketHandler)
			r.With(authMW.Handler, platmiddleware.RequireRole("admin")).Post("/register", authHandler.Register)
		})

		// Los dispositivos se autentican con X-Device-Key, no con JWT.
		r.With(platmiddleware.RateLimit(240, time.Minute)).Post("/devices/events", devicesHandler.ProcessEvent)

		// WebSocket de ocupación (solo lectura, filtrado por sede).
		r.Get("/ws/occupancy", rtHandler.OccupancyWS)

		// Ingesta idempotente de eventos de las APIs de borde.
		if syncIngest != nil {
			r.With(platmiddleware.RateLimit(600, time.Minute)).Post("/sync/events", syncIngest.Ingest)
		}

		// ---------- Protegido por JWT ----------
		r.Group(func(r chi.Router) {
			r.Use(authMW.Handler)

			// Tickets
			r.Route("/tickets", func(r chi.Router) {
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/", parkingHandler.Enter)
				r.Get("/", parkingHandler.List)
				r.Get("/{id}", parkingHandler.Get)
				r.Get("/{id}/events", parkingHandler.Events)
				r.Post("/{id}/quote", parkingHandler.Quote)
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/{id}/settle", parkingHandler.Settle)
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/{id}/cancel", parkingHandler.Cancel)
			})

			// Pagos
			r.Route("/payments", func(r chi.Router) {
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/", billingHandler.CreatePayment)
				r.Get("/", billingHandler.ListPayments)
				r.Get("/{id}", billingHandler.GetPayment)
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/{id}/refund", billingHandler.RefundPayment)
			})

			// Abonados
			r.Route("/subscriptions", func(r chi.Router) {
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/", subsHandler.CreateSubscription)
				r.Get("/", subsHandler.ListSubscriptions)
				r.Get("/validate", subsHandler.Validate)
				r.Get("/{id}", subsHandler.GetSubscription)
				r.With(platmiddleware.RequireRole("admin", "operator")).Delete("/{id}", subsHandler.CancelSubscription)
			})

			// Vehículos
			r.Route("/vehicles", func(r chi.Router) {
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/", vehiclesHandler.CreateVehicle)
				r.Get("/", vehiclesHandler.ListVehicles)
				r.Get("/plate/{plate}", vehiclesHandler.GetVehicleByPlate)
				r.Get("/{id}", vehiclesHandler.GetVehicle)
			})

			// Clientes
			r.Route("/customers", func(r chi.Router) {
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/", customersHandler.CreateCustomer)
				r.With(platmiddleware.RequireRole("admin", "operator")).Get("/", customersHandler.ListCustomers)
				r.With(platmiddleware.RequireRole("admin", "operator")).Get("/{id}", customersHandler.GetCustomer)
			})

			// Tarifas
			r.Route("/rate-plans", func(r chi.Router) {
				r.With(platmiddleware.RequireRole("admin")).Post("/", rateplansHandler.CreateRatePlan)
				r.Get("/", rateplansHandler.ListRatePlans)
				r.Get("/{id}", rateplansHandler.GetRatePlan)
				r.With(platmiddleware.RequireRole("admin")).Delete("/{id}", rateplansHandler.DeleteRatePlan)
			})

			// Dispositivos (administración)
			r.Route("/devices", func(r chi.Router) {
				r.With(platmiddleware.RequireRole("admin")).Post("/", devicesHandler.RegisterDevice)
				r.Get("/", devicesHandler.ListDevices)
			})

			// Reportes
			r.Route("/reports", func(r chi.Router) {
				r.Get("/occupancy", reportingHandler.Ocupacion)
				r.With(platmiddleware.RequireRole("admin", "auditor")).Get("/revenue", reportingHandler.Ingresos)
				r.With(platmiddleware.RequireRole("admin", "auditor")).Get("/cash-daily", reportingHandler.CajaDiaria)
			})

			// Auditoría
			r.With(platmiddleware.RequireRole("admin", "auditor")).Get("/audit", auditHandler.List)

			// Catálogo
			r.Route("/catalog", func(r chi.Router) {
				r.Get("/sites", catalogHandler.ListSites)
				r.Get("/sites/{id}", catalogHandler.GetSite)
				r.Get("/zones", catalogHandler.ListZones)
				r.Get("/spaces", catalogHandler.ListSpaces)
				r.Get("/vehicle-types", catalogHandler.ListVehicleTypes)

				r.Group(func(r chi.Router) {
					r.Use(platmiddleware.RequireRole("admin"))
					r.Post("/sites", catalogHandler.CreateSite)
					r.Put("/sites/{id}", catalogHandler.UpdateSite)
					r.Delete("/sites/{id}", catalogHandler.DeleteSite)

					r.Post("/zones", catalogHandler.CreateZone)
					r.Put("/zones/{id}", catalogHandler.UpdateZone)
					r.Delete("/zones/{id}", catalogHandler.DeleteZone)

					r.Post("/spaces", catalogHandler.CreateSpace)
					r.Put("/spaces/{id}", catalogHandler.UpdateSpace)
					r.Put("/spaces/{id}/status", catalogHandler.UpdateSpaceStatus)
					r.Delete("/spaces/{id}", catalogHandler.DeleteSpace)

					r.Post("/vehicle-types", catalogHandler.CreateVehicleType)
					r.Put("/vehicle-types/{id}", catalogHandler.UpdateVehicleType)
					r.Delete("/vehicle-types/{id}", catalogHandler.DeleteVehicleType)
				})
			})
		})
	})

	return r
}

// siteResolver implementa billing.SiteResolver usando los servicios de parking y suscripciones.
type siteResolver struct {
	parking interface {
		TicketSite(ticketID string) (string, error)
	}
	subs interface {
		SubscriptionSite(subscriptionID string) (string, error)
	}
}

func (s *siteResolver) TicketSite(ticketID string) (string, error) {
	return s.parking.TicketSite(ticketID)
}

func (s *siteResolver) SubscriptionSite(subscriptionID string) (string, error) {
	return s.subs.SubscriptionSite(subscriptionID)
}
