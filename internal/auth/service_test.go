package auth

import (
	"testing"
	"time"

	"parking-backend/internal/config"
	"parking-backend/internal/platform/security"
)

type fakeRepo struct {
	users  map[string]*User
	byID   map[string]*User
	tokens map[string]*RefreshToken
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:  map[string]*User{},
		byID:   map[string]*User{},
		tokens: map[string]*RefreshToken{},
	}
}

func (f *fakeRepo) Create(u *User) error {
	if u.ID == "" {
		u.ID = "user-" + u.Username
	}
	f.users[u.Username] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeRepo) ByUsername(username string) (*User, error) {
	u, ok := f.users[username]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) ByID(id string) (*User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) UpdateLastLogin(id string, at time.Time) error { return nil }

func (f *fakeRepo) CreateRefreshToken(t *RefreshToken) error {
	if t.ID == "" {
		t.ID = "rt-" + t.TokenHash[:8]
	}
	f.tokens[t.TokenHash] = t
	return nil
}

func (f *fakeRepo) RefreshTokenByHash(hash string) (*RefreshToken, error) {
	t, ok := f.tokens[hash]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) RevokeRefreshToken(id string) error {
	now := time.Now()
	for _, t := range f.tokens {
		if t.ID == id {
			t.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeRepo) RevokeUserRefreshTokens(userID string) error {
	now := time.Now()
	for _, t := range f.tokens {
		if t.UserID == userID {
			t.RevokedAt = &now
		}
	}
	return nil
}

func newTestService() (*Service, *fakeRepo) {
	repo := newFakeRepo()
	cfg := &config.Config{
		JWTSecret:  "test-secret",
		JWTAccess:  time.Minute,
		JWTRefresh: time.Hour,
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

	claims, err := security.ParseAccessToken("test-secret", access)
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

	// El token viejo ya no debe servir.
	if _, _, err := svc.Refresh(refresh); err == nil {
		t.Fatal("el refresh token viejo debe quedar revocado")
	}

	// El nuevo sí debe servir.
	if _, _, err := svc.Refresh(newRefresh); err != nil {
		t.Fatalf("Refresh con token nuevo: %v", err)
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
