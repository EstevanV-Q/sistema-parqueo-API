package catalog

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/validate"
	"parking-backend/internal/shared/pagination"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ---- Sedes ----

type siteReq struct {
	Name          string `json:"name" validate:"required,max=200"`
	Code          string `json:"code" validate:"required,max=20"`
	Timezone      string `json:"timezone" validate:"required,max=60"`
	Currency      string `json:"currency" validate:"required,len=3"`
	CapacityTotal int    `json:"capacity_total" validate:"gte=0"`
	IsActive      *bool  `json:"is_active,omitempty"`
}

func (h *Handler) CreateSite(w http.ResponseWriter, r *http.Request) {
	var req siteReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	site, err := h.svc.CreateSite(req.Name, req.Code, req.Timezone, req.Currency, req.CapacityTotal)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, site)
}

func (h *Handler) ListSites(w http.ResponseWriter, r *http.Request) {
	sites, err := h.svc.ListSites()
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, sites)
}

func (h *Handler) GetSite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	site, err := h.svc.GetSite(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, site)
}

func (h *Handler) UpdateSite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req siteReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	site, err := h.svc.UpdateSite(id, req.Name, req.Code, req.Timezone, req.Currency, req.CapacityTotal, active)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, site)
}

func (h *Handler) DeleteSite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteSite(id); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.NoContent(w)
}

// ---- Zonas ----

type zoneReq struct {
	SiteID   string `json:"site_id" validate:"required"`
	Name     string `json:"name" validate:"required,max=100"`
	Type     string `json:"type" validate:"required,oneof=car moto bike mixed"`
	Capacity int    `json:"capacity" validate:"gte=0"`
	IsActive *bool  `json:"is_active,omitempty"`
}

func (h *Handler) CreateZone(w http.ResponseWriter, r *http.Request) {
	var req zoneReq
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
	zone, err := h.svc.CreateZone(req.SiteID, req.Name, req.Type, req.Capacity)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, zone)
}

func (h *Handler) ListZones(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "site_id requerido", http.StatusBadRequest, nil))
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	zones, err := h.svc.ListZones(siteID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, zones)
}

func (h *Handler) UpdateZone(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name     string `json:"name" validate:"required,max=100"`
		Type     string `json:"type" validate:"required,max=30"`
		Capacity int    `json:"capacity" validate:"gte=0"`
		IsActive *bool  `json:"is_active,omitempty"`
	}
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	zone, err := h.svc.UpdateZone(id, req.Name, req.Type, req.Capacity, active)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, zone)
}

func (h *Handler) DeleteZone(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteZone(id); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.NoContent(w)
}

// ---- Espacios ----

type spaceReq struct {
	ZoneID string `json:"zone_id" validate:"required"`
	Code   string `json:"code" validate:"required,max=20"`
	Type   string `json:"type" validate:"required,oneof=car moto bike mixed"`
}

func (h *Handler) CreateSpace(w http.ResponseWriter, r *http.Request) {
	var req spaceReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	space, err := h.svc.CreateSpace(req.ZoneID, req.Code, req.Type)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, space)
}

func (h *Handler) ListSpaces(w http.ResponseWriter, r *http.Request) {
	zoneID := r.URL.Query().Get("zone_id")
	if zoneID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "zone_id requerido", http.StatusBadRequest, nil))
		return
	}
	spaces, err := h.svc.ListSpaces(zoneID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	page := pagination.FromRequest(r)
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"data": spaces,
		"meta": page.Meta(len(spaces)),
	})
}

func (h *Handler) UpdateSpace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Code     string `json:"code" validate:"required,max=20"`
		Type     string `json:"type" validate:"required,oneof=car moto bike mixed"`
		Status   string `json:"status" validate:"omitempty,oneof=free occupied reserved disabled"`
		IsActive *bool  `json:"is_active,omitempty"`
	}
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	space, err := h.svc.UpdateSpace(id, req.Code, req.Type, req.Status, active)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, space)
}

func (h *Handler) UpdateSpaceStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status" validate:"required,oneof=free occupied reserved disabled"`
	}
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := h.svc.UpdateStatus(id, req.Status); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) DeleteSpace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteSpace(id); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.NoContent(w)
}

// ---- Tipos de vehículo ----

type vehicleTypeReq struct {
	Name     string `json:"name" validate:"required,max=50"`
	Code     string `json:"code" validate:"required,max=10"`
	IsActive *bool  `json:"is_active,omitempty"`
}

func (h *Handler) CreateVehicleType(w http.ResponseWriter, r *http.Request) {
	var req vehicleTypeReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	vt, err := h.svc.CreateVehicleType(req.Name, req.Code)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, vt)
}

func (h *Handler) ListVehicleTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.svc.ListVehicleTypes()
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, types)
}

func (h *Handler) UpdateVehicleType(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req vehicleTypeReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	vt, err := h.svc.UpdateVehicleType(id, req.Name, req.Code, active)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, vt)
}

func (h *Handler) DeleteVehicleType(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteVehicleType(id); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.NoContent(w)
}
