package middleware_test

import (
	"context"
	"testing"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/middleware"
	ctxpkg "parking-backend/internal/platform/security/ctx"
)

func TestCheckSiteAccess(t *testing.T) {
	ctx := context.Background()
	ctx = ctxpkg.WithRole(ctx, "operator")
	ctx = ctxpkg.WithSiteID(ctx, "site-1")

	if err := middleware.CheckSiteAccess(ctx, "site-1"); err != nil {
		t.Fatalf("misma sede debe permitirse: %v", err)
	}
	if err := middleware.CheckSiteAccess(ctx, "site-2"); !apperr.Is(err, apperr.ErrForbidden) {
		t.Fatalf("otra sede debe denegarse, err=%v", err)
	}
}

func TestCheckSiteAccessAdminBypassesSite(t *testing.T) {
	ctx := ctxpkg.WithRole(context.Background(), "admin")
	if err := middleware.CheckSiteAccess(ctx, "site-2"); err != nil {
		t.Fatalf("admin debe poder operar en cualquier sede: %v", err)
	}
}

func TestCheckSiteAccessOperatorWithoutSite(t *testing.T) {
	ctx := ctxpkg.WithRole(context.Background(), "operator")
	if err := middleware.CheckSiteAccess(ctx, "site-1"); !apperr.Is(err, apperr.ErrForbidden) {
		t.Fatalf("operador sin sede debe denegarse, err=%v", err)
	}
}
