package httpserver_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
)

func TestErrorDoesNotLeakInternalDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	httpserver.Error(rec, errors.New("Error 1062: duplicate entry 'admin' for key 'users.username'"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, se esperaba 500", rec.Code)
	}
	body := strings.ToLower(rec.Body.String())
	if strings.Contains(body, "1062") || strings.Contains(body, "duplicate entry") || strings.Contains(body, "users.username") {
		t.Fatalf("la respuesta filtró detalles internos: %s", rec.Body.String())
	}
	if !strings.Contains(body, "error interno") {
		t.Fatalf("se esperaba mensaje genérico: %s", rec.Body.String())
	}
}

func TestAppErrorKeepsSafeMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	httpserver.Error(rec, apperr.New(apperr.ErrInvalidInput, "site_id requerido", http.StatusBadRequest, nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, se esperaba 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "site_id requerido") {
		t.Fatalf("se esperaba el mensaje de validación: %s", rec.Body.String())
	}
}

func TestDecodeRejectsNonJSONContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"x":1}`))
	req.Header.Set("Content-Type", "text/plain")
	var dst map[string]any
	err := httpserver.Decode(req, &dst)
	if err == nil {
		t.Fatal("se esperaba error para Content-Type no JSON")
	}
	if apperr.HTTPStatusFrom(err) != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, se esperaba 415", apperr.HTTPStatusFrom(err))
	}
}
