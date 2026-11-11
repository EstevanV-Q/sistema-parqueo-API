package devices

import (
	"net/http"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/validate"
)

type Handler struct {
	svc           *Service
	deviceLimiter *middleware.KeyLimiter
}

func NewHandler(svc *Service, deviceLimiters ...*middleware.KeyLimiter) *Handler {
	h := &Handler{svc: svc}
	if len(deviceLimiters) > 0 {
		h.deviceLimiter = deviceLimiters[0]
	}
	return h
}

// deviceResp es el DTO público de un dispositivo. Nunca incluye el hash de la
// API key ni el hash de autenticación.
type deviceResp struct {
	ID            string  `json:"id"`
	SiteID        string  `json:"site_id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	IsActive      bool    `json:"is_active"`
	LastHeartbeat *string `json:"last_heartbeat,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
}

func toDeviceResp(d *Device) deviceResp {
	if d == nil {
		return deviceResp{}
	}
	return deviceResp{
		ID:            d.ID,
		SiteID:        d.SiteID,
		Name:          d.Name,
		Type:          d.Type,
		IsActive:      d.IsActive,
		LastHeartbeat: d.LastHeartbeat,
		CreatedAt:     d.CreatedAt,
	}
}

type registerDeviceReq struct {
	SiteID string `json:"site_id" validate:"required"`
	Name   string `json:"name" validate:"required,max=100"`
	Type   string `json:"type" validate:"required,oneof=barrier lpr rfid printer"`
	APIKey string `json:"api_key" validate:"required,min=16"`
}

func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req registerDeviceReq
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

	d, err := h.svc.RegisterDevice(req.SiteID, req.Name, req.Type, req.APIKey)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, toDeviceResp(d))
}

func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "site_id requerido", http.StatusBadRequest, nil))
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	list, err := h.svc.ListBySite(siteID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	out := make([]deviceResp, 0, len(list))
	for _, d := range list {
		out = append(out, toDeviceResp(d))
	}
	httpserver.JSON(w, http.StatusOK, out)
}

type processEventReq struct {
	EventID         string `json:"event_id" validate:"required,max=128"`
	EventType       string `json:"event_type" validate:"required,max=50"`
	PlateNormalized string `json:"plate_normalized" validate:"omitempty,max=20"`
}

// ProcessEvent requiere el header X-Device-Key con la API key del dispositivo.
func (h *Handler) ProcessEvent(w http.ResponseWriter, r *http.Request) {
	device, err := h.svc.Authenticate(r.Header.Get("X-Device-Key"))
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if h.deviceLimiter != nil && !h.deviceLimiter.Allow(device.ID) {
		w.Header().Set("Retry-After", "60")
		httpserver.Error(w, apperr.New(apperr.ErrTooManyRequests, "demasiados eventos para este dispositivo", http.StatusTooManyRequests, nil))
		return
	}

	var req processEventReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}

	event, err := h.svc.ProcessEvent(device, req.EventID, req.EventType, req.PlateNormalized)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"event":   event,
		"command": event.Decision,
	})
}
