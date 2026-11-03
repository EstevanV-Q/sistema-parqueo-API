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
	ID           string
	Username     string
	Email        string
	PasswordHash string
	Role         Role
	SiteID       *string // nil = acceso a todas las sedes (admin)
	Active       bool
	CreatedAt    string
	LastLogin    *string
}

// RefreshToken representa un refresh token persistido (solo se guarda su hash).
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// IsValid devuelve true si el token no está revocado ni expirado.
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
