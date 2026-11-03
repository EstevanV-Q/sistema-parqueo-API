package rateplans

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/validate"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type createRatePlanReq struct {
	SiteID           string  `json:"site_id" validate:"required"`
	Name             string  `json:"name" validate:"required,max=100"`
	Currency         string  `json:"currency" validate:"omitempty,len=3"`
	GraceMinutes     int     `json:"grace_minutes" validate:"gte=0"`
	MinimumCents     int64   `json:"minimum_cents" validate:"gte=0"`
	FractionMinutes  int     `json:"fraction_minutes" validate:"required,gt=0"`
	CentsPerFraction int64   `json:"cents_per_fraction" validate:"gte=0"`
	DailyCapCents    int64   `json:"daily_cap_cents" validate:"gte=0"`
	LostTicketCents  int64   `json:"lost_ticket_cents" validate:"gte=0"`
	VehicleTypeID    *string `json:"vehicle_type_id,omitempty"`
}

func (h *Handler) CreateRatePlan(w http.ResponseWriter, r *http.Request) {
	var req createRatePlanReq
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

	rp, err := h.svc.Create(req.SiteID, req.Name, req.Currency, req.GraceMinutes, req.FractionMinutes, req.MinimumCents, req.CentsPerFraction, req.DailyCapCents, req.LostTicketCents, req.VehicleTypeID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, rp)
}

func (h *Handler) ListRatePlans(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "site_id requerido", http.StatusBadRequest, nil))
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	plans, err := h.svc.ListBySite(siteID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, plans)
}

func (h *Handler) GetRatePlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rp, err := h.svc.Get(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, rp)
}

func (h *Handler) DeleteRatePlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Deactivate(id); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.NoContent(w)
}
