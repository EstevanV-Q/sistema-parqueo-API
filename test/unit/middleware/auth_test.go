package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"parking-backend/internal/config"
	"parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/security"
)

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:   "test-secret-with-at-least-32-characters",
		JWTIssuer:   "parking-backend",
		JWTAudience: "parking-api",
		JWTAccess:   time.Minute,
		JWTRefresh:  time.Hour,
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestAuthMiddlewareRejectsRefreshToken(t *testing.T) {
	cfg := testConfig()
	refresh, _, err := security.RefreshToken(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, "user-1", time.Hour)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", nil)
	req.Header.Set("Authorization", "Bearer "+refresh)
	rec := httptest.NewRecorder()

	middleware.NewAuthMiddleware(cfg).Handler(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, se esperaba 401; body = %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddlewareAcceptsAccessToken(t *testing.T) {
	cfg := testConfig()
	access, _, err := security.AccessToken(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, "user-1", "admin", "admin", nil, time.Minute)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rec := httptest.NewRecorder()

	middleware.NewAuthMiddleware(cfg).Handler(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, se esperaba 204; body = %s", rec.Code, rec.Body.String())
	}
}
