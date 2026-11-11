package auth

import (
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"parking-backend/internal/config"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/clock"
	"parking-backend/internal/platform/security"
)

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

func getDummyHash() string {
	dummyHashOnce.Do(func() {
		dummyHash, _ = security.HashPassword("dummy-password-not-used")
	})
	return dummyHash
}

// Service contiene la lógica de negocio de auth.
type Service struct {
	repo Repository
	cfg  *config.Config
	now  clock.NowFunc
}

func NewService(repo Repository, cfg *config.Config) *Service {
	return &Service{
		repo: repo,
		cfg:  cfg,
		now:  clock.Real,
	}
}

// Register crea un nuevo usuario y hashea su contraseña.
func (s *Service) Register(username, email, password string, role Role, siteID *string) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))
	if username == "" || email == "" || password == "" {
		return nil, ErrInvalidInput
	}
	if len(username) > 100 || len(email) > 255 || len(password) > 128 {
		return nil, ErrInvalidInput
	}
	if role != RoleAdmin && role != RoleOperator && role != RoleAuditor {
		return nil, ErrInvalidInput
	}
	existing, err := s.repo.ByUsername(username)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAlreadyExists
	}
	existing, err = s.repo.ByEmail(email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAlreadyExists
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, apperr.From(err)
	}

	u := NewUser(username, email, hash, role, siteID)
	if err := s.repo.Create(u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login verifica credenciales, emite tokens y persiste el refresh token.
func (s *Service) Login(username, password string) (*User, string, string, error) {
	if username == "" || password == "" || len(username) > 100 || len(password) > 128 {
		return nil, "", "", ErrInvalidCredentials
	}
	u, err := s.repo.ByUsername(username)
	if err != nil {
		if hash := getDummyHash(); hash != "" {
			_, _ = security.ComparePassword(password, hash)
		}
		return nil, "", "", ErrInvalidCredentials
	}
	if !u.Active {
		_, _ = security.ComparePassword(password, u.PasswordHash)
		return nil, "", "", ErrInvalidCredentials
	}

	ok, err := security.ComparePassword(password, u.PasswordHash)
	if err != nil || !ok {
		return nil, "", "", ErrInvalidCredentials
	}

	accessTok, _, err := security.AccessToken(s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.JWTAudience, u.ID, u.Username, string(u.Role), u.SiteID, s.cfg.JWTAccess)
	if err != nil {
		return nil, "", "", apperr.From(err)
	}

	refreshTok, refreshExp, err := security.RefreshToken(s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.JWTAudience, u.ID, s.cfg.JWTRefresh)
	if err != nil {
		return nil, "", "", apperr.From(err)
	}

	if err := s.repo.CreateRefreshToken(&RefreshToken{
		UserID:    u.ID,
		TokenHash: security.HashToken(refreshTok),
		ExpiresAt: refreshExp,
	}); err != nil {
		return nil, "", "", apperr.From(err)
	}

	if err := s.repo.UpdateLastLogin(u.ID, s.now()); err != nil {
		log.Printf("auth: no se pudo actualizar last_login para usuario %s: %v", u.ID, err)
	}
	return u, accessTok, refreshTok, nil
}

// Refresh valida y rota un refresh token, devolviendo un nuevo par de tokens.
func (s *Service) Refresh(refreshToken string) (string, string, error) {
	if refreshToken == "" {
		return "", "", ErrInvalidCredentials
	}

	userID, err := security.ParseRefreshToken(s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.JWTAudience, refreshToken)
	if err != nil {
		return "", "", ErrInvalidCredentials
	}

	stored, err := s.repo.RefreshTokenByHash(security.HashToken(refreshToken))
	if err != nil {
		return "", "", ErrInvalidCredentials
	}
	if stored.UserID != userID {
		return "", "", ErrInvalidCredentials
	}
	familyID := stored.ID
	if stored.FamilyID != nil && *stored.FamilyID != "" {
		familyID = *stored.FamilyID
	}
	if stored.RevokedAt != nil {
		// Reutilización de un token ya rotado: revocar toda la familia.
		_ = s.repo.RevokeRefreshTokenFamily(familyID)
		return "", "", ErrInvalidCredentials
	}
	if !stored.IsValid(s.now()) {
		return "", "", ErrInvalidCredentials
	}

	u, err := s.repo.ByID(userID)
	if err != nil {
		return "", "", err
	}
	if !u.Active {
		return "", "", ErrInactive
	}

	// Se emite el nuevo refresh token y se rota el anterior en una sola
	// transacción. Si otro proceso ya lo rotó, se revoca la familia.
	newRefresh, refreshExp, err := security.RefreshToken(s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.JWTAudience, u.ID, s.cfg.JWTRefresh)
	if err != nil {
		return "", "", apperr.From(err)
	}
	newToken := &RefreshToken{
		UserID:    u.ID,
		TokenHash: security.HashToken(newRefresh),
		ExpiresAt: refreshExp,
	}
	if err := s.repo.RotateRefreshToken(stored.ID, newToken); err != nil {
		if errors.Is(err, ErrTokenReuse) {
			_ = s.repo.RevokeRefreshTokenFamily(familyID)
		}
		return "", "", ErrInvalidCredentials
	}

	accessTok, _, err := security.AccessToken(s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.JWTAudience, u.ID, u.Username, string(u.Role), u.SiteID, s.cfg.JWTAccess)
	if err != nil {
		return "", "", apperr.From(err)
	}

	return accessTok, newRefresh, nil
}

// Logout revoca el refresh token entregado. Si no existe, no falla.
func (s *Service) Logout(refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	stored, err := s.repo.RefreshTokenByHash(security.HashToken(refreshToken))
	if err != nil {
		return nil
	}
	return s.repo.RevokeRefreshToken(stored.ID)
}

// LogoutAll revoca todos los refresh tokens de un usuario.
func (s *Service) LogoutAll(userID string) error {
	return s.repo.RevokeUserRefreshTokens(userID)
}

// Me devuelve el usuario por su ID.
func (s *Service) Me(userID string) (*User, error) {
	return s.repo.ByID(userID)
}

// WSTicket emite un token efímero para abrir el WebSocket de ocupación.
func (s *Service) WSTicket(userID, username, role string, siteID *string) (string, time.Time, error) {
	return security.WSTicket(
		s.cfg.JWTSecret,
		s.cfg.JWTIssuer,
		s.cfg.JWTAudience,
		userID,
		username,
		role,
		siteID,
		30*time.Second,
	)
}
