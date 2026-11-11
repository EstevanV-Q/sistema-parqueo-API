package auth_test

import (
	"testing"
	"time"

	. "parking-backend/internal/auth"
	"parking-backend/internal/config"
	"parking-backend/internal/platform/security"
)

func newTestService() (*Service, *fakeRepo) {
	repo := newFakeRepo()
	cfg := &config.Config{
		JWTSecret:   "test-secret",
		JWTIssuer:   "parking-backend",
		JWTAudience: "parking-api",
		JWTAccess:   time.Minute,
		JWTRefresh:  time.Hour,
	}
	return NewService(repo, cfg), repo
}

func TestRegisterAndLogin(t *testing.T) {
	svc, _ := newTestService()

	u, err := svc.Register("admin", "admin@test.com", "Admin123!", RoleAdmin, nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, access, refresh, err := svc.Login("admin", "Admin123!")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("Login debe devolver access y refresh")
	}

	claims, err := security.ParseAccessToken("test-secret", "parking-backend", "parking-api", access)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != u.ID {
		t.Fatalf("user_id = %s, quiere %s", claims.UserID, u.ID)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.Register("admin", "admin@test.com", "Admin123!", RoleAdmin, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, _, _, err := svc.Login("admin", "incorrecta"); err == nil {
		t.Fatal("Login con contraseña incorrecta debe fallar")
	}
}

func TestRefresh_RotatesAndRevokesOldToken(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.Register("admin", "admin@test.com", "Admin123!", RoleAdmin, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, _, refresh, err := svc.Login("admin", "Admin123!")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	newAccess, newRefresh, err := svc.Refresh(refresh)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if newAccess == "" || newRefresh == "" {
		t.Fatal("Refresh debe devolver un nuevo par")
	}
	if newRefresh == refresh {
		t.Fatal("el refresh token debe rotar")
	}

	// El nuevo token debe servir antes de detectar la reutilización.
	_, newestRefresh, err := svc.Refresh(newRefresh)
	if err != nil {
		t.Fatalf("Refresh con token nuevo: %v", err)
	}

	// Reutilizar el token viejo debe fallar y revocar toda la familia,
	// incluido el token más reciente.
	if _, _, err := svc.Refresh(refresh); err == nil {
		t.Fatal("la reutilización del token viejo debe fallar")
	}
	if _, _, err := svc.Refresh(newestRefresh); err == nil {
		t.Fatal("la reutilización debe revocar la familia completa")
	}
}

func TestLogout_RevokesToken(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.Register("admin", "admin@test.com", "Admin123!", RoleAdmin, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, _, refresh, err := svc.Login("admin", "Admin123!")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := svc.Logout(refresh); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, _, err := svc.Refresh(refresh); err == nil {
		t.Fatal("un token revocado no debe poder refrescarse")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.Register("admin", "admin@test.com", "Admin123!", RoleAdmin, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := svc.Register("admin", "otro@test.com", "Admin123!", RoleAdmin, nil); err == nil {
		t.Fatal("un usuario duplicado debe fallar")
	}
}
