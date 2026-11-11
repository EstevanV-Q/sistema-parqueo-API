package audit

import (
	"net/http"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
	ctxpkg "parking-backend/internal/platform/security/ctx"
	"parking-backend/internal/shared/pagination"
)

// Handler expone la consulta de auditoría.
type Handler struct {
	svc *Service
}

// NewHandler crea el handler de auditoría.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// List devuelve los registros de auditoría filtrados.
// GET /api/v1/audit?entity_type=&entity_id=&user_id=&action=&from=&to=&limit=&offset=
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	siteID := q.Get("site_id")
	if siteID != "" {
		if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
			httpserver.Error(w, err)
			return
		}
	} else if userSite, ok := ctxpkg.SiteIDFromCtx(r.Context()); ok && userSite != "" {
		siteID = userSite
	}
	if role, _ := ctxpkg.RoleFromCtx(r.Context()); role != "admin" && siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrForbidden, "el usuario no tiene una sede asignada", http.StatusForbidden, nil))
		return
	}

	filter := Filter{
		EntityType: q.Get("entity_type"),
		EntityID:   q.Get("entity_id"),
		UserID:     q.Get("user_id"),
		SiteID:     siteID,
		Action:     q.Get("action"),
		From:       q.Get("from"),
		To:         q.Get("to"),
	}
	page := pagination.FromRequest(r)

	entries, total, err := h.svc.List(filter, page.Limit, page.Offset)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"data": entries,
		"meta": page.Meta(total),
	})
}
