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

func (s *Service) accessSecret() string {
	if s.cfg.JWTAccessSecret != "" {
		return s.cfg.JWTAccessSecret
	}
	return s.cfg.JWTSecret
}

func (s *Service) refreshSecret() string {
	if s.cfg.JWTRefreshSecret != "" {
		return s.cfg.JWTRefreshSecret
	}
	return s.cfg.JWTSecret
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
	if err := security.ValidatePassword(password); err != nil {
		return nil, apperr.New(apperr.ErrInvalidInput, err.Error(), 400, nil)
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

// Login verifica credenciales sin TOTP. Se mantiene para clientes y pruebas
// que no usan MFA; el handler HTTP usa LoginTOTP.
func (s *Service) Login(username, password string) (*User, string, string, error) {
	return s.LoginTOTP(username, password, "")
}

// LoginTOTP verifica credenciales, bloqueo y, si aplica, el código TOTP.
func (s *Service) LoginTOTP(username, password, totpCode string) (*User, string, string, error) {
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
	if u.LockedUntil != nil && s.now().Before(*u.LockedUntil) {
		return nil, "", "", ErrAccountLocked
	}
	if !u.Active {
		_, _ = security.ComparePassword(password, u.PasswordHash)
		return nil, "", "", ErrInvalidCredentials
	}

	ok, err := security.ComparePassword(password, u.PasswordHash)
	if err != nil || !ok {
		if lockErr := s.recordLoginFailure(u); lockErr != nil {
			return nil, "", "", lockErr
		}
		return nil, "", "", ErrInvalidCredentials
	}

	if u.Role == RoleAdmin && u.TOTPEnabled {
		if u.TOTPSecret == nil || *u.TOTPSecret == "" {
			return nil, "", "", ErrTOTPInvalid
		}
		if totpCode == "" {
			return nil, "", "", ErrTOTPRequired
		}
		if !security.ValidateTOTP(*u.TOTPSecret, totpCode, s.now()) {
			if lockErr := s.recordLoginFailure(u); lockErr != nil {
				return nil, "", "", lockErr
			}
			return nil, "", "", ErrTOTPInvalid
		}
	}

	if err := s.repo.UpdateLoginState(u.ID, 0, nil); err != nil {
		log.Printf("auth: no se pudo resetear intentos fallidos de %s: %v", u.ID, err)
	}

	accessTok, _, err := security.AccessToken(s.accessSecret(), s.cfg.JWTIssuer, s.cfg.JWTAudience, u.ID, u.Username, string(u.Role), u.SiteID, s.cfg.JWTAccess)
	if err != nil {
		return nil, "", "", apperr.From(err)
	}

	refreshTok, refreshExp, err := security.RefreshToken(s.refreshSecret(), s.cfg.JWTIssuer, s.cfg.JWTAudience, u.ID, s.cfg.JWTRefresh)
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

func (s *Service) recordLoginFailure(u *User) error {
	attempts := u.FailedLoginAttempts + 1
	var lockedUntil *time.Time
	if attempts >= 5 {
		t := s.now().Add(15 * time.Minute)
		lockedUntil = &t
		attempts = 0
	}
	if err := s.repo.UpdateLoginState(u.ID, attempts, lockedUntil); err != nil {
		log.Printf("auth: no se pudo registrar intento fallido de %s: %v", u.ID, err)
	}
	u.FailedLoginAttempts = attempts
	u.LockedUntil = lockedUntil
	if lockedUntil != nil {
		return ErrAccountLocked
	}
	return nil
}

// Refresh valida y rota un refresh token, devolviendo un nuevo par de tokens.
func (s *Service) Refresh(refreshToken string) (string, string, error) {
	if refreshToken == "" {
		return "", "", ErrInvalidCredentials
	}

	userID, err := security.ParseRefreshToken(s.refreshSecret(), s.cfg.JWTIssuer, s.cfg.JWTAudience, refreshToken)
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
	newRefresh, refreshExp, err := security.RefreshToken(s.refreshSecret(), s.cfg.JWTIssuer, s.cfg.JWTAudience, u.ID, s.cfg.JWTRefresh)
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

	accessTok, _, err := security.AccessToken(s.accessSecret(), s.cfg.JWTIssuer, s.cfg.JWTAudience, u.ID, u.Username, string(u.Role), u.SiteID, s.cfg.JWTAccess)
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

// SetupTOTP genera un secreto TOTP pendiente de confirmación.
func (s *Service) SetupTOTP(userID string) (string, string, error) {
	u, err := s.repo.ByID(userID)
	if err != nil {
		return "", "", err
	}
	if u.TOTPEnabled {
		return "", "", apperr.New(apperr.ErrInvalidInput, "TOTP ya está habilitado", 400, nil)
	}
	secret, err := security.GenerateTOTPSecret()
	if err != nil {
		return "", "", apperr.From(err)
	}
	if err := s.repo.UpdateTOTP(u.ID, secret, false); err != nil {
		return "", "", err
	}
	return secret, security.TOTPURL("Parking", u.Username, secret), nil
}

// EnableTOTP confirma el secreto con un código válido.
func (s *Service) EnableTOTP(userID, code string) error {
	u, err := s.repo.ByID(userID)
	if err != nil {
		return err
	}
	if u.TOTPSecret == nil || *u.TOTPSecret == "" {
		return apperr.New(apperr.ErrInvalidInput, "primero configura TOTP", 400, nil)
	}
	if !security.ValidateTOTP(*u.TOTPSecret, code, s.now()) {
		return ErrTOTPInvalid
	}
	return s.repo.UpdateTOTP(u.ID, *u.TOTPSecret, true)
}

// DisableTOTP exige contraseña y código vigente.
func (s *Service) DisableTOTP(userID, password, code string) error {
	u, err := s.repo.ByID(userID)
	if err != nil {
		return err
	}
	ok, err := security.ComparePassword(password, u.PasswordHash)
	if err != nil || !ok {
		return ErrInvalidCredentials
	}
	if u.TOTPSecret == nil || !security.ValidateTOTP(*u.TOTPSecret, code, s.now()) {
		return ErrTOTPInvalid
	}
	return s.repo.UpdateTOTP(u.ID, "", false)
}

// ListSessions devuelve los refresh tokens del usuario.
func (s *Service) ListSessions(userID string) ([]*RefreshToken, error) {
	return s.repo.ListRefreshTokens(userID)
}

// RevokeSession revoca una sesión concreta del usuario.
func (s *Service) RevokeSession(userID, sessionID string) error {
	return s.repo.RevokeRefreshTokenByID(userID, sessionID)
}

// RequestPasswordReset genera un token de recuperación. Devuelve cadena vacía
// si el identificador no existe para no filtrar usuarios.
func (s *Service) RequestPasswordReset(identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	u, err := s.repo.ByUsername(identifier)
	if err != nil {
		u, err = s.repo.ByEmail(identifier)
	}
	if err != nil || u == nil {
		return "", nil
	}
	token, err := security.GenerateToken()
	if err != nil {
		return "", apperr.From(err)
	}
	if err := s.repo.CreatePasswordResetToken(&PasswordResetToken{
		UserID:    u.ID,
		TokenHash: security.HashToken(token),
		ExpiresAt: s.now().Add(15 * time.Minute),
	}); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPassword consume un token de recuperación y cambia la contraseña.
func (s *Service) ResetPassword(token, newPassword string) error {
	if token == "" {
		return ErrInvalidCredentials
	}
	if err := security.ValidatePassword(newPassword); err != nil {
		return apperr.New(apperr.ErrInvalidInput, err.Error(), 400, nil)
	}
	stored, err := s.repo.PasswordResetTokenByHash(security.HashToken(token))
	if err != nil || stored.UsedAt != nil || s.now().After(stored.ExpiresAt) {
		return ErrInvalidCredentials
	}
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return apperr.From(err)
	}
	if err := s.repo.UpdatePasswordHash(stored.UserID, hash); err != nil {
		return err
	}
	if err := s.repo.RevokeUserRefreshTokens(stored.UserID); err != nil {
		return err
	}
	return s.repo.MarkPasswordResetTokenUsed(stored.ID)
}

// IsDevelopment indica si el proceso corre en desarrollo (útil para mostrar
// tokens de recuperación solo en local).
func (s *Service) IsDevelopment() bool {
	return s.cfg.Env == "development"
}

// Me devuelve el usuario por su ID.
func (s *Service) Me(userID string) (*User, error) {
	return s.repo.ByID(userID)
}

// WSTicket emite un token efímero para abrir el WebSocket de ocupación.
func (s *Service) WSTicket(userID, username, role string, siteID *string) (string, time.Time, error) {
	return security.WSTicket(
		s.accessSecret(),
		s.cfg.JWTIssuer,
		s.cfg.JWTAudience,
		userID,
		username,
		role,
		siteID,
		30*time.Second,
	)
}
