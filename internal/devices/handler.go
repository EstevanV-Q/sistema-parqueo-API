package devices

import (
	"net/http"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/validate"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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

	d, err := h.svc.RegisterDevice(req.SiteID, req.Name, req.Type, req.APIKey)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, d)
}

func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "site_id requerido", http.StatusBadRequest, nil))
		return
	}
	list, err := h.svc.ListBySite(siteID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, list)
}

type processEventReq struct {
	EventID         string `json:"event_id" validate:"required"`
	EventType       string `json:"event_type" validate:"required"`
	PlateNormalized string `json:"plate_normalized"`
}

// ProcessEvent requiere el header X-Device-Key con la API key del dispositivo.
func (h *Handler) ProcessEvent(w http.ResponseWriter, r *http.Request) {
	device, err := h.svc.Authenticate(r.Header.Get("X-Device-Key"))
	if err != nil {
		httpserver.Error(w, err)
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
