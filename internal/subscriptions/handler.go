package subscriptions

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"parking-backend/internal/audit"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/validate"
	"parking-backend/internal/shared/pagination"
)

type Handler struct {
	svc     *Service
	auditor *audit.Service
}

func NewHandler(svc *Service, auditor *audit.Service) *Handler {
	return &Handler{svc: svc, auditor: auditor}
}

type createSubscriptionReq struct {
	CustomerID string `json:"customer_id" validate:"required"`
	VehicleID  string `json:"vehicle_id" validate:"required"`
	SiteID     string `json:"site_id" validate:"required"`
	PlanName   string `json:"plan_name" validate:"required,max=100"`
	PriceCents int64  `json:"price_cents" validate:"gte=0"`
	StartsAt   string `json:"starts_at" validate:"required"`
	ExpiresAt  string `json:"expires_at" validate:"required"`
}

func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req createSubscriptionReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), req.SiteID); err != nil {
		httpserver.Error(w, err)
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "starts_at inválido (use RFC3339)", http.StatusBadRequest, err))
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "expires_at inválido (use RFC3339)", http.StatusBadRequest, err))
		return
	}
	if !expiresAt.After(startsAt) {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "expires_at debe ser posterior a starts_at", http.StatusBadRequest, nil))
		return
	}

	sub, err := h.svc.Create(req.CustomerID, req.VehicleID, req.SiteID, req.PlanName, req.PriceCents, startsAt, expiresAt)
	if err != nil {
		httpserver.Error(w, err)
		return
	}

	if h.auditor != nil {
		_ = h.auditor.Record(r.Context(), "subscription.create", "subscription", sub.ID, nil, map[string]any{
			"vehicle_id": sub.VehicleID,
			"site_id":    sub.SiteID,
			"plan_name":  sub.PlanName,
			"expires_at": sub.ExpiresAt,
		})
	}

	httpserver.JSON(w, http.StatusCreated, sub)
}

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "site_id requerido", http.StatusBadRequest, nil))
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}

	status := r.URL.Query().Get("status")
	page := pagination.FromRequest(r)

	subs, err := h.svc.ListBySite(siteID, status, page.Limit, page.Offset)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"data": subs,
		"meta": page.Meta(0),
	})
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.svc.Get(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, sub)
}

func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Cancel(id); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.NoContent(w)
}

func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.URL.Query().Get("vehicle_id")
	siteID := r.URL.Query().Get("site_id")
	if vehicleID == "" || siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "vehicle_id y site_id son requeridos", http.StatusBadRequest, nil))
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}

	sub, err := h.svc.Validate(vehicleID, siteID, time.Now().UTC())
	if err != nil {
		httpserver.Error(w, apperr.New(apperr.ErrNotFound, "no hay abono vigente", http.StatusNotFound, err))
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"valid":   true,
		"plan":    sub.PlanName,
		"expires": sub.ExpiresAt,
		"vehicle": sub.VehicleID,
	})
}
