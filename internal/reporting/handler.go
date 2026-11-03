package reporting

import (
	"net/http"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Ocupacion(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	o, err := h.svc.Ocupacion(siteID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, o)
}

func (h *Handler) Ingresos(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	i, err := h.svc.Ingresos(siteID, from, to)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, i)
}

func (h *Handler) CajaDiaria(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	date := r.URL.Query().Get("date")
	if date == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "date requerido (YYYY-MM-DD)", http.StatusBadRequest, nil))
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	c, err := h.svc.CajaDiaria(siteID, date)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, c)
}
