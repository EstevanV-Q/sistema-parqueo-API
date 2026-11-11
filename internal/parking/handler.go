package parking

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"parking-backend/internal/audit"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/security/ctx"
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

func userID(r *http.Request) string {
	if v, ok := ctx.UserIDFromCtx(r.Context()); ok {
		return v
	}
	return ""
}

func (h *Handler) record(r *http.Request, action, entityID string, oldValues, newValues map[string]any) {
	if h.auditor == nil {
		return
	}
	_ = h.auditor.Record(r.Context(), action, "ticket", entityID, oldValues, newValues)
}

type enterReq struct {
	SiteID          string `json:"site_id" validate:"required"`
	ZoneID          string `json:"zone_id"`
	SpaceID         string `json:"space_id"`
	VehicleID       string `json:"vehicle_id"`
	PlateNormalized string `json:"plate_normalized" validate:"required,max=20"`
	PlateRaw        string `json:"plate_raw" validate:"max=50"`
	TypeID          string `json:"type_id" validate:"required"`
	RatePlanID      string `json:"rate_plan_id"`
}

// Enter registra la entrada de un vehículo.
func (h *Handler) Enter(w http.ResponseWriter, r *http.Request) {
	var req enterReq
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

	ticket, err := h.svc.CreateTicket(req.SiteID, req.ZoneID, req.SpaceID, req.VehicleID, req.PlateNormalized, req.PlateRaw, req.TypeID, req.RatePlanID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}

	h.record(r, "ticket.entry", ticket.ID, nil, map[string]any{
		"site_id":  ticket.SiteID,
		"space_id": ticket.SpaceID,
		"plate":    ticket.PlateNormalized,
		"status":   ticket.Status,
		"entry_at": ticket.EntryAt,
	})

	httpserver.JSON(w, http.StatusCreated, ticket)
}

// Quote calcula la salida sin modificar el ticket.
func (h *Handler) Quote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, quote, err := h.svc.Quote(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), t.SiteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, quote)
}

type settleReq struct {
	PaidCents int64  `json:"paid_cents" validate:"gte=0"`
	Method    string `json:"method" validate:"omitempty,oneof=cash card transfer abonnement waived"`
}

// Settle paga y cierra el ticket.
func (h *Handler) Settle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req settleReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	current, err := h.svc.repo.ByID(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), current.SiteID); err != nil {
		httpserver.Error(w, err)
		return
	}

	ticket, err := h.svc.Settle(id, req.PaidCents, req.Method, userID(r))
	if err != nil {
		httpserver.Error(w, err)
		return
	}

	h.record(r, "ticket.settle", ticket.ID,
		map[string]any{"status": TicketOpen},
		map[string]any{
			"site_id":     ticket.SiteID,
			"status":      ticket.Status,
			"total_cents": ticket.TotalCents,
			"paid_cents":  ticket.PaidCents,
			"method":      req.Method,
		})

	httpserver.JSON(w, http.StatusOK, ticket)
}

// Cancel anula un ticket abierto.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	current, err := h.svc.repo.ByID(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), current.SiteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	ticket, err := h.svc.Cancel(id, userID(r))
	if err != nil {
		httpserver.Error(w, err)
		return
	}

	h.record(r, "ticket.cancel", ticket.ID,
		map[string]any{"status": TicketOpen},
		map[string]any{"site_id": ticket.SiteID, "status": TicketCancelled})

	httpserver.JSON(w, http.StatusOK, ticket)
}

// List lista tickets de una sede con paginación.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
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

	tickets, err := h.svc.repo.ListBySite(siteID, status, page.Limit, page.Offset)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	total, err := h.svc.repo.CountBySite(siteID, status)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"data": tickets,
		"meta": page.Meta(total),
	})
}

// Get obtiene un ticket por ID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ticket, err := h.svc.repo.ByID(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), ticket.SiteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, ticket)
}

// Events devuelve el historial de eventos de un ticket.
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ticket, err := h.svc.repo.ByID(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), ticket.SiteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	events, err := h.svc.repo.Events(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, events)
}
