package auth

import (
	"errors"
	"time"

	"parking-backend/internal/config"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/clock"
	"parking-backend/internal/platform/security"
)

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
	if username == "" || email == "" || password == "" {
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
	u, err := s.repo.ByUsername(username)
	if err != nil {
		return nil, "", "", ErrInvalidCredentials
	}
	if !u.Active {
		return nil, "", "", ErrInactive
	}

	ok, err := security.ComparePassword(password, u.PasswordHash)
	if err != nil || !ok {
		return nil, "", "", ErrInvalidCredentials
	}

	accessTok, _, err := security.AccessToken(s.cfg.JWTSecret, u.ID, u.Username, string(u.Role), u.SiteID, s.cfg.JWTAccess)
	if err != nil {
		return nil, "", "", apperr.From(err)
	}

	refreshTok, refreshExp, err := security.RefreshToken(s.cfg.JWTSecret, u.ID, s.cfg.JWTRefresh)
	if err != nil {
		return nil, "", "", apperr.From(err)
	}

	if err := s.repo.CreateRefreshToken(&RefreshToken{
		UserID:    u.ID,
		TokenHash: security.HashToken(refreshTok),
		ExpiresAt: refreshExp,
	}); err != nil {
		return nil, "", "", err
	}

	_ = s.repo.UpdateLastLogin(u.ID, s.now())
	return u, accessTok, refreshTok, nil
}

// Refresh valida y rota un refresh token, devolviendo un nuevo par de tokens.
func (s *Service) Refresh(refreshToken string) (string, string, error) {
	if refreshToken == "" {
		return "", "", ErrInvalidCredentials
	}

	userID, err := security.ParseRefreshToken(s.cfg.JWTSecret, refreshToken)
	if err != nil {
		return "", "", ErrInvalidCredentials
	}

	stored, err := s.repo.RefreshTokenByHash(security.HashToken(refreshToken))
	if err != nil || !stored.IsValid(s.now()) {
		return "", "", ErrInvalidCredentials
	}
	if stored.UserID != userID {
		return "", "", ErrInvalidCredentials
	}

	u, err := s.repo.ByID(userID)
	if err != nil {
		return "", "", err
	}
	if !u.Active {
		return "", "", ErrInactive
	}

	// Rotación: se revoca el token usado y se emite uno nuevo.
	if err := s.repo.RevokeRefreshToken(stored.ID); err != nil {
		return "", "", err
	}

	accessTok, _, err := security.AccessToken(s.cfg.JWTSecret, u.ID, u.Username, string(u.Role), u.SiteID, s.cfg.JWTAccess)
	if err != nil {
		return "", "", apperr.From(err)
	}

	newRefresh, refreshExp, err := security.RefreshToken(s.cfg.JWTSecret, u.ID, s.cfg.JWTRefresh)
	if err != nil {
		return "", "", apperr.From(err)
	}
	if err := s.repo.CreateRefreshToken(&RefreshToken{
		UserID:    u.ID,
		TokenHash: security.HashToken(newRefresh),
		ExpiresAt: refreshExp,
	}); err != nil {
		return "", "", err
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

var _ = time.Time{}
