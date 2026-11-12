package auth

import "time"

// Role define los roles del sistema.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleAuditor  Role = "auditor"
)

// User del sistema.
type User struct {
	ID                  string     `json:"id"`
	Username            string     `json:"username"`
	Email               string     `json:"email"`
	PasswordHash        string     `json:"-"`
	Role                Role       `json:"role"`
	SiteID              *string    `json:"site_id,omitempty"`
	Active              bool       `json:"active"`
	CreatedAt           string     `json:"created_at,omitempty"`
	LastLogin           *string    `json:"last_login,omitempty"`
	TOTPSecret          *string    `json:"-"`
	TOTPEnabled         bool       `json:"totp_enabled"`
	FailedLoginAttempts int        `json:"-"`
	LockedUntil         *time.Time `json:"-"`
}

// RefreshToken representa un refresh token persistido (solo se guarda su hash).
type RefreshToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	TokenHash  string     `json:"-"`
	FamilyID   *string    `json:"family_id,omitempty"`
	ReplacedBy *string    `json:"replaced_by,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// PasswordResetToken representa un token de recuperación de contraseña.
type PasswordResetToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// IsValid devuelve true si el token de refresh no está revocado ni expirado.
func (t *RefreshToken) IsValid(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// NewUser crea un User sin hashear la contraseña (el service se encarga).
func NewUser(username, email, passwordHash string, role Role, siteID *string) *User {
	return &User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		SiteID:       siteID,
		Active:       true,
	}
}

// CanAccessSite verifica si el usuario puede operar en una sede.
func (u *User) CanAccessSite(siteID string) bool {
	if u.Role == RoleAdmin && u.SiteID == nil {
		return true
	}
	if u.SiteID != nil && *u.SiteID == siteID {
		return true
	}
	return false
}
