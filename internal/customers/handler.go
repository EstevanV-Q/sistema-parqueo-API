package customers

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

type createCustomerReq struct {
	Document string  `json:"document" validate:"required,max=50"`
	Name     string  `json:"name" validate:"required,max=200"`
	Phone    *string `json:"phone,omitempty" validate:"omitempty,max=30"`
	Email    *string `json:"email,omitempty" validate:"omitempty,email,max=255"`
}

func (h *Handler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req createCustomerReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	c, err := h.svc.Create(req.Document, req.Name, req.Phone, req.Email)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, c)
}

func (h *Handler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	list, err := h.svc.List(page.Limit, page.Offset)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"data": list,
		"meta": page.Meta(0),
	})
}

func (h *Handler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := h.svc.Get(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, c)
}
