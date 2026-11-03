// Package app es el composition root del backend: construye repositorios,
// servicios, handlers, middlewares y el router HTTP.
package app

import (
	"database/sql"
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
	"parking-backend/internal/rateplans"
	"parking-backend/internal/realtime"
	"parking-backend/internal/reporting"
	"parking-backend/internal/subscriptions"
	"parking-backend/internal/vehicles"
)

// App agrupa las dependencias de larga vida del proceso.
type App struct {
	Config *config.Config
	DB     *sql.DB
	Hub    *realtime.Hub
	Router http.Handler
}

// New construye la aplicación completa. El llamante es responsable de cerrar
// la base de datos con Close.
func New(cfg *config.Config) (*App, error) {
	database, err := db.New(cfg)
	if err != nil {
		return nil, err
	}

	// --- Auditoría ---
	auditSvc := audit.NewService(audit.NewMySQLRepository(database))
	auditHandler := audit.NewHandler(auditSvc)

	// --- Auth ---
	authRepo := auth.NewMySQLRepository(database)
	authSvc := auth.NewService(authRepo, cfg)
	authHandler := auth.NewHandler(authSvc)

	// --- Catálogo ---
	catalogSvc := catalog.NewService(
		catalog.NewMySQLSiteRepo(database),
		catalog.NewMySQLZoneRepo(database),
		catalog.NewMySQLSpaceRepo(database),
		catalog.NewMySQLVehicleTypeRepo(database),
	)
	catalogHandler := catalog.NewHandler(catalogSvc)

	// --- Vehículos y clientes ---
	vehiclesSvc := vehicles.NewService(vehicles.NewMySQLRepository(database))
	vehiclesHandler := vehicles.NewHandler(vehiclesSvc)

	customersSvc := customers.NewService(customers.NewMySQLRepository(database))
	customersHandler := customers.NewHandler(customersSvc)

	// --- Tarifas ---
	rateplansSvc := rateplans.NewService(rateplans.NewMySQLRepository(database))
	rateplansHandler := rateplans.NewHandler(rateplansSvc)

	// --- Pagos ---
	billingSvc := billing.NewService(billing.NewMySQLRepository(database))
	billingHandler := billing.NewHandler(billingSvc, auditSvc)

	// --- Abonados ---
	subscriptionsSvc := subscriptions.NewService(subscriptions.NewMySQLRepository(database))
	subsHandler := subscriptions.NewHandler(subscriptionsSvc, auditSvc)

	// --- Tiempo real ---
	rtHub := realtime.NewHub()
	go rtHub.Run()
	rtPublisher := realtime.NewPublisher(rtHub)
	rtHandler := realtime.NewHandler(rtHub)

	// --- Parking (depende de tarifas, catálogo, abonados, pagos y realtime) ---
	parkingSvc := parking.NewService(parking.NewMySQLRepository(database)).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc).
		WithSubscriptionChecker(subscriptionsSvc).
		WithPaymentCreator(billingSvc).
		WithEventPublisher(rtPublisher)
	parkingHandler := parking.NewHandler(parkingSvc, auditSvc)

	// --- Dispositivos (consulta a parking y publica eventos) ---
	devicesSvc := devices.NewService(devices.NewMySQLRepository(database)).
		WithGate(parkingSvc).
		WithTicketCreator(parkingSvc).
		WithPublisher(rtPublisher)
	devicesHandler := devices.NewHandler(devicesSvc)

	// --- Reportes ---
	reportingSvc := reporting.NewService(reporting.NewMySQLRepository(database))
	reportingHandler := reporting.NewHandler(reportingSvc)

	// --- Router ---
	authMW := platmiddleware.NewAuthMiddleware(cfg)
	r := buildRouter(database, authMW,
		authHandler, parkingHandler, billingHandler, subsHandler,
		vehiclesHandler, customersHandler, rateplansHandler,
		catalogHandler, devicesHandler, reportingHandler, auditHandler,
		rtHandler,
	)

	return &App{
		Config: cfg,
		DB:     database,
		Hub:    rtHub,
		Router: r,
	}, nil
}

// Close libera recursos.
func (a *App) Close() error {
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
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
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
			r.With(authMW.Handler, platmiddleware.RequireRole("admin")).Post("/register", authHandler.Register)
		})

		// Los dispositivos se autentican con X-Device-Key, no con JWT.
		r.With(platmiddleware.RateLimit(240, time.Minute)).Post("/devices/events", devicesHandler.ProcessEvent)

		// WebSocket de ocupación (solo lectura, filtrado por sede).
		r.Get("/ws/occupancy", rtHandler.OccupancyWS)

		// ---------- Protegido por JWT ----------
		r.Group(func(r chi.Router) {
			r.Use(authMW.Handler)

			// Tickets
			r.Route("/tickets", func(r chi.Router) {
				r.Post("/", parkingHandler.Enter)
				r.Get("/", parkingHandler.List)
				r.Get("/{id}", parkingHandler.Get)
				r.Get("/{id}/events", parkingHandler.Events)
				r.Post("/{id}/quote", parkingHandler.Quote)
				r.Post("/{id}/settle", parkingHandler.Settle)
				r.Post("/{id}/cancel", parkingHandler.Cancel)
			})

			// Pagos
			r.Route("/payments", func(r chi.Router) {
				r.Post("/", billingHandler.CreatePayment)
				r.Get("/", billingHandler.ListPayments)
				r.Get("/{id}", billingHandler.GetPayment)
				r.With(platmiddleware.RequireRole("admin", "operator")).Post("/{id}/refund", billingHandler.RefundPayment)
			})

			// Abonados
			r.Route("/subscriptions", func(r chi.Router) {
				r.Post("/", subsHandler.CreateSubscription)
				r.Get("/", subsHandler.ListSubscriptions)
				r.Get("/validate", subsHandler.Validate)
				r.Get("/{id}", subsHandler.GetSubscription)
				r.With(platmiddleware.RequireRole("admin", "operator")).Delete("/{id}", subsHandler.CancelSubscription)
			})

			// Vehículos
			r.Route("/vehicles", func(r chi.Router) {
				r.Post("/", vehiclesHandler.CreateVehicle)
				r.Get("/", vehiclesHandler.ListVehicles)
				r.Get("/plate/{plate}", vehiclesHandler.GetVehicleByPlate)
				r.Get("/{id}", vehiclesHandler.GetVehicle)
			})

			// Clientes
			r.Route("/customers", func(r chi.Router) {
				r.Post("/", customersHandler.CreateCustomer)
				r.Get("/", customersHandler.ListCustomers)
				r.Get("/{id}", customersHandler.GetCustomer)
			})

			// Tarifas
			r.Route("/rate-plans", func(r chi.Router) {
				r.Post("/", rateplansHandler.CreateRatePlan)
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
