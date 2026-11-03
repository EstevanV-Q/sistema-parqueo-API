package middleware

import (
	"context"
	"net/http"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	ctxpkg "parking-backend/internal/platform/security/ctx"
)

// CheckSiteAccess valida que el usuario del contexto pueda operar en la sede.
// Un administrador sin sede asignada puede operar en cualquier sede.
func CheckSiteAccess(c context.Context, siteID string) error {
	if siteID == "" {
		return nil
	}

	role, _ := ctxpkg.RoleFromCtx(c)
	if role == "admin" {
		return nil
	}

	userSite, ok := ctxpkg.SiteIDFromCtx(c)
	if !ok || userSite == "" {
		return apperr.New(apperr.ErrForbidden, "el usuario no tiene una sede asignada", http.StatusForbidden, nil)
	}
	if userSite != siteID {
		return apperr.New(apperr.ErrForbidden, "no tiene acceso a esta sede", http.StatusForbidden, nil)
	}
	return nil
}

// RequireSiteAccess aplica CheckSiteAccess usando el parámetro `site_id` de la
// query string. Para cuerpos JSON la validación se hace en el handler.
func RequireSiteAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := CheckSiteAccess(r.Context(), r.URL.Query().Get("site_id")); err != nil {
			httpserver.Error(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}
