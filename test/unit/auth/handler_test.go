package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"parking-backend/internal/auth"
	"parking-backend/internal/config"
)

type fakeRepo struct {
	users       map[string]*auth.User
	byID        map[string]*auth.User
	tokens      map[string]*auth.RefreshToken
	resetTokens map[string]*auth.PasswordResetToken
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:       map[string]*auth.User{},
		byID:        map[string]*auth.User{},
		tokens:      map[string]*auth.RefreshToken{},
		resetTokens: map[string]*auth.PasswordResetToken{},
	}
}

func (f *fakeRepo) Create(u *auth.User) error {
	if u.ID == "" {
		u.ID = "user-" + u.Username
	}
	f.users[u.Username] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeRepo) ByUsername(username string) (*auth.User, error) {
	u, ok := f.users[username]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) ByEmail(email string) (*auth.User, error) {
	for _, u := range f.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return nil, auth.ErrNotFound
}

func (f *fakeRepo) ByID(id string) (*auth.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) UpdateLastLogin(string, time.Time) error { return nil }
func (f *fakeRepo) UpdatePasswordHash(id, hash string) error {
	if u, ok := f.byID[id]; ok {
		u.PasswordHash = hash
	}
	return nil
}

func (f *fakeRepo) UpdateLoginState(id string, attempts int, lockedUntil *time.Time) error {
	if u, ok := f.byID[id]; ok {
		u.FailedLoginAttempts = attempts
		u.LockedUntil = lockedUntil
	}
	return nil
}

func (f *fakeRepo) UpdateTOTP(id, secret string, enabled bool) error {
	if u, ok := f.byID[id]; ok {
		u.TOTPSecret = &secret
		u.TOTPEnabled = enabled
	}
	return nil
}

func (f *fakeRepo) CreateRefreshToken(t *auth.RefreshToken) error {
	if t.ID == "" {
		t.ID = "rt-" + t.TokenHash[:8]
	}
	f.tokens[t.TokenHash] = t
	return nil
}

func (f *fakeRepo) RefreshTokenByHash(hash string) (*auth.RefreshToken, error) {
	t, ok := f.tokens[hash]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) RotateRefreshToken(oldID string, newToken *auth.RefreshToken) error {
	var old *auth.RefreshToken
	for _, t := range f.tokens {
		if t.ID == oldID {
			old = t
			break
		}
	}
	if old == nil {
		return auth.ErrNotFound
	}
	if old.RevokedAt != nil {
		return auth.ErrTokenReuse
	}
	now := time.Now()
	old.RevokedAt = &now
	if newToken.ID == "" {
		newToken.ID = "rt-" + newToken.TokenHash[:8]
	}
	family := old.ID
	if old.FamilyID != nil {
		family = *old.FamilyID
	}
	newToken.FamilyID = &family
	old.ReplacedBy = &newToken.ID
	f.tokens[newToken.TokenHash] = newToken
	return nil
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

func (f *fakeRepo) RevokeRefreshTokenFamily(familyID string) error {
	now := time.Now()
	for _, t := range f.tokens {
		if (t.FamilyID != nil && *t.FamilyID == familyID) || t.ID == familyID {
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

func (f *fakeRepo) ListRefreshTokens(userID string) ([]*auth.RefreshToken, error) {
	var out []*auth.RefreshToken
	for _, t := range f.tokens {
		if t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeRepo) RevokeRefreshTokenByID(userID, id string) error {
	now := time.Now()
	for _, t := range f.tokens {
		if t.UserID == userID && t.ID == id {
			t.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeRepo) CreatePasswordResetToken(t *auth.PasswordResetToken) error {
	if t.ID == "" {
		t.ID = "reset-" + t.TokenHash[:8]
	}
	f.resetTokens[t.TokenHash] = t
	return nil
}

func (f *fakeRepo) PasswordResetTokenByHash(hash string) (*auth.PasswordResetToken, error) {
	t, ok := f.resetTokens[hash]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) MarkPasswordResetTokenUsed(id string) error {
	for _, t := range f.resetTokens {
		if t.ID == id {
			now := time.Now()
			t.UsedAt = &now
		}
	}
	return nil
}

func newHandler() *auth.Handler {
	cfg := &config.Config{
		JWTSecret:   "test-secret-with-at-least-32-characters",
		JWTIssuer:   "parking-backend",
		JWTAudience: "parking-api",
		JWTAccess:   time.Minute,
		JWTRefresh:  time.Hour,
	}
	return auth.NewHandler(auth.NewService(newFakeRepo(), cfg))
}

func TestRegisterResponseDoesNotExposePasswordHash(t *testing.T) {
	h := newHandler()
	body := `{"username":"admin","email":"admin@test.com","password":"Admin123!Strong","role":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "passwordhash") ||
		strings.Contains(strings.ToLower(rec.Body.String()), "password_hash") {
		t.Fatalf("la respuesta no debe incluir el hash de contraseña: %s", rec.Body.String())
	}

	var resp struct {
		User struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Email    string `json:"email"`
			Role     string `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("respuesta JSON inválida: %v", err)
	}
	if resp.User.ID == "" || resp.User.Username != "admin" || resp.User.Email != "admin@test.com" {
		t.Fatalf("usuario inesperado: %+v", resp.User)
	}
}

func TestRegisterDuplicateEmailReturnsConflict(t *testing.T) {
	h := newHandler()
	first := `{"username":"admin","email":"admin@test.com","password":"Admin123!Strong","role":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(first))
	req.Header.Set("Content-Type", "application/json")
	h.Register(httptest.NewRecorder(), req)

	second := `{"username":"otro","email":"ADMIN@test.com","password":"Admin123!Strong","role":"admin"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(second))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, se esperaba 409; body = %s", rec.Code, rec.Body.String())
	}
}
