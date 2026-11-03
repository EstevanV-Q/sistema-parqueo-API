package vehicles

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/validate"
	"parking-backend/internal/shared/pagination"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type createVehicleReq struct {
	PlateNormalized string  `json:"plate_normalized" validate:"omitempty,max=20"`
	PlateRaw        string  `json:"plate_raw" validate:"max=50"`
	TypeID          string  `json:"type_id" validate:"required"`
	CustomerID      *string `json:"customer_id,omitempty"`
}

func (h *Handler) CreateVehicle(w http.ResponseWriter, r *http.Request) {
	var req createVehicleReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	v, err := h.svc.Create(req.PlateNormalized, req.PlateRaw, req.TypeID, req.CustomerID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, v)
}

func (h *Handler) ListVehicles(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	customerID := r.URL.Query().Get("customer_id")
	list, err := h.svc.List(customerID, page.Limit, page.Offset)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"data": list,
		"meta": page.Meta(0),
	})
}

func (h *Handler) GetVehicle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	v, err := h.svc.Get(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, v)
}

func (h *Handler) GetVehicleByPlate(w http.ResponseWriter, r *http.Request) {
	plate := chi.URLParam(r, "plate")
	v, err := h.svc.GetByPlate(plate)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, v)
}
