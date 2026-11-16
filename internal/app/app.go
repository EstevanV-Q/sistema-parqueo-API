// Package app arma la aplicación: repositorios, servicios, handlers y router.
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
	"parking-backend/internal/parking"
	"parking-backend/internal/platform/db"
	"parking-backend/internal/platform/httpserver"
	mw "parking-backend/internal/platform/middleware"
	"parking-backend/internal/rateplans"
	"parking-backend/internal/reporting"
	"parking-backend/internal/subscriptions"
	"parking-backend/internal/vehicles"
	"parking-backend/web"
)

// App agrupa las dependencias de larga vida del proceso.
type App struct {
	Config *config.Config
	DB     *sql.DB
	Router http.Handler
}

// handlers agrupa los handlers HTTP para pasarlos al router.
type handlers struct {
	auth          *auth.Handler
	parking       *parking.Handler
	billing       *billing.Handler
	subscriptions *subscriptions.Handler
	vehicles      *vehicles.Handler
	customers     *customers.Handler
	rateplans     *rateplans.Handler
	catalog       *catalog.Handler
	reporting     *reporting.Handler
	audit         *audit.Handler
}

// New abre la base de datos y construye la aplicación. Cerrar con Close.
func New(cfg *config.Config) (*App, error) {
	database, err := db.Open(cfg)
	if err != nil {
		return nil, err
	}
	return NewWithDB(cfg, database), nil
}

// NewWithDB construye la aplicación sobre una conexión ya abierta.
func NewWithDB(cfg *config.Config, database *sql.DB) *App {
	mw.SetAllowedOrigins(cfg.CORSAllowedOrigins)

	auditSvc := audit.NewService(audit.NewRepository(database))
	authSvc := auth.NewService(auth.NewRepository(database), cfg)
	catalogSvc := catalog.NewService(
		catalog.NewSiteRepo(database),
		catalog.NewZoneRepo(database),
		catalog.NewSpaceRepo(database),
		catalog.NewVehicleTypeRepo(database),
	)
	rateplansSvc := rateplans.NewService(rateplans.NewRepository(database))
	subscriptionsSvc := subscriptions.NewService(subscriptions.NewRepository(database))
	billingSvc := billing.NewService(billing.NewRepository(database))

	parkingSvc := parking.NewService(parking.NewRepository(database, !cfg.IsSQLite())).
		WithRatePlanReader(rateplansSvc).
		WithSpaceUpdater(catalogSvc).
		WithSpaceAllocator(spaceAllocator{catalog: catalogSvc}).
		WithSubscriptionChecker(subscriptionsSvc)

	h := handlers{
		auth:          auth.NewHandler(authSvc),
		parking:       parking.NewHandler(parkingSvc, auditSvc),
		billing:       billing.NewHandler(billingSvc, auditSvc, &siteResolver{parking: parkingSvc, subs: subscriptionsSvc}),
		subscriptions: subscriptions.NewHandler(subscriptionsSvc, auditSvc),
		vehicles:      vehicles.NewHandler(vehicles.NewService(vehicles.NewRepository(database))),
		customers:     customers.NewHandler(customers.NewService(customers.NewRepository(database))),
		rateplans:     rateplans.NewHandler(rateplansSvc),
		catalog:       catalog.NewHandler(catalogSvc),
		reporting:     reporting.NewHandler(reporting.NewService(reporting.NewRepository(database))),
		audit:         audit.NewHandler(auditSvc),
	}

	return &App{
		Config: cfg,
		DB:     database,
		Router: buildRouter(database, mw.NewAuthMiddleware(cfg), h),
	}
}

// Close libera la conexión a la base de datos.
func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

func buildRouter(database *sql.DB, authMW *mw.AuthMiddleware, h handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(mw.RequestIDHeader)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Logger)
	r.Use(mw.RequestInfo)
	r.Use(mw.CORS)

	// Salud
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpserver.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if err := database.PingContext(req.Context()); err != nil {
			httpserver.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "database unavailable"})
			return
		}
		httpserver.JSON(w, http.StatusOK, map[string]string{"status": "ready", "database": "connected"})
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
		// Público
		r.Route("/auth", func(r chi.Router) {
			r.With(mw.RateLimit(10, time.Minute)).Post("/login", h.auth.Login)
			r.With(mw.RateLimit(30, time.Minute)).Post("/refresh", h.auth.RefreshHandler)
			r.Post("/logout", h.auth.LogoutHandler)
			r.With(authMW.Handler).Get("/me", h.auth.MeHandler)
			r.With(authMW.Handler, mw.RequireRole("admin")).Post("/register", h.auth.Register)
		})

		// Protegido por JWT
		r.Group(func(r chi.Router) {
			r.Use(authMW.Handler)
			staff := mw.RequireRole("admin", "operator")
			admin := mw.RequireRole("admin")
			auditors := mw.RequireRole("admin", "auditor")

			r.Route("/tickets", func(r chi.Router) {
				r.With(staff).Post("/", h.parking.Enter)
				r.Get("/", h.parking.List)
				r.Get("/{id}", h.parking.Get)
				r.Get("/{id}/events", h.parking.Events)
				r.Post("/{id}/quote", h.parking.Quote)
				r.With(staff).Post("/{id}/settle", h.parking.Settle)
				r.With(staff).Post("/{id}/cancel", h.parking.Cancel)
			})

			r.Route("/payments", func(r chi.Router) {
				r.With(staff).Post("/", h.billing.CreatePayment)
				r.Get("/", h.billing.ListPayments)
				r.Get("/{id}", h.billing.GetPayment)
				r.With(staff).Post("/{id}/refund", h.billing.RefundPayment)
			})

			r.Route("/subscriptions", func(r chi.Router) {
				r.With(staff).Post("/", h.subscriptions.CreateSubscription)
				r.Get("/", h.subscriptions.ListSubscriptions)
				r.Get("/validate", h.subscriptions.Validate)
				r.Get("/{id}", h.subscriptions.GetSubscription)
				r.With(staff).Delete("/{id}", h.subscriptions.CancelSubscription)
			})

			r.Route("/vehicles", func(r chi.Router) {
				r.With(staff).Post("/", h.vehicles.CreateVehicle)
				r.Get("/", h.vehicles.ListVehicles)
				r.Get("/plate/{plate}", h.vehicles.GetVehicleByPlate)
				r.Get("/{id}", h.vehicles.GetVehicle)
			})

			r.Route("/customers", func(r chi.Router) {
				r.Use(staff)
				r.Post("/", h.customers.CreateCustomer)
				r.Get("/", h.customers.ListCustomers)
				r.Get("/{id}", h.customers.GetCustomer)
			})

			r.Route("/rate-plans", func(r chi.Router) {
				r.With(admin).Post("/", h.rateplans.CreateRatePlan)
				r.Get("/", h.rateplans.ListRatePlans)
				r.Get("/{id}", h.rateplans.GetRatePlan)
				r.With(admin).Delete("/{id}", h.rateplans.DeleteRatePlan)
			})

			r.Route("/reports", func(r chi.Router) {
				r.Get("/occupancy", h.reporting.Ocupacion)
				r.With(auditors).Get("/revenue", h.reporting.Ingresos)
				r.With(auditors).Get("/cash-daily", h.reporting.CajaDiaria)
			})

			r.With(auditors).Get("/audit", h.audit.List)

			r.Route("/catalog", func(r chi.Router) {
				r.Get("/sites", h.catalog.ListSites)
				r.Get("/sites/{id}", h.catalog.GetSite)
				r.Get("/zones", h.catalog.ListZones)
				r.Get("/spaces", h.catalog.ListSpaces)
				r.Get("/vehicle-types", h.catalog.ListVehicleTypes)

				r.Group(func(r chi.Router) {
					r.Use(admin)
					r.Post("/sites", h.catalog.CreateSite)
					r.Put("/sites/{id}", h.catalog.UpdateSite)
					r.Delete("/sites/{id}", h.catalog.DeleteSite)
					r.Post("/zones", h.catalog.CreateZone)
					r.Put("/zones/{id}", h.catalog.UpdateZone)
					r.Delete("/zones/{id}", h.catalog.DeleteZone)
					r.Post("/spaces", h.catalog.CreateSpace)
					r.Put("/spaces/{id}", h.catalog.UpdateSpace)
					r.Put("/spaces/{id}/status", h.catalog.UpdateSpaceStatus)
					r.Delete("/spaces/{id}", h.catalog.DeleteSpace)
					r.Post("/vehicle-types", h.catalog.CreateVehicleType)
					r.Put("/vehicle-types/{id}", h.catalog.UpdateVehicleType)
					r.Delete("/vehicle-types/{id}", h.catalog.DeleteVehicleType)
				})
			})
		})
	})

	// Interfaz web (archivos estáticos embebidos en el binario).
	r.Handle("/*", web.Handler())

	return r
}

// siteResolver permite a billing saber a qué sede pertenece un ticket o abono.
type siteResolver struct {
	parking interface{ TicketSite(string) (string, error) }
	subs    interface{ SubscriptionSite(string) (string, error) }
}

func (s *siteResolver) TicketSite(id string) (string, error) { return s.parking.TicketSite(id) }
func (s *siteResolver) SubscriptionSite(id string) (string, error) {
	return s.subs.SubscriptionSite(id)
}

// spaceAllocator adapta catalog.Service a parking.SpaceAllocator.
type spaceAllocator struct {
	catalog *catalog.Service
}

func (a spaceAllocator) FreeSpaces(siteID, vehicleTypeID string) ([]parking.SpaceCandidate, error) {
	refs, err := a.catalog.FreeSpaces(siteID, vehicleTypeID)
	if err != nil {
		return nil, err
	}
	out := make([]parking.SpaceCandidate, len(refs))
	for i, r := range refs {
		out[i] = parking.SpaceCandidate{SpaceID: r.SpaceID, ZoneID: r.ZoneID}
	}
	return out, nil
}
