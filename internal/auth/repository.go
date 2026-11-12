package auth

import (
	"database/sql"
	"time"
)

// Repository define el contrato de acceso a datos de auth.
type Repository interface {
	Create(u *User) error
	ByUsername(username string) (*User, error)
	ByEmail(email string) (*User, error)
	ByID(id string) (*User, error)
	UpdateLastLogin(id string, at time.Time) error
	UpdatePasswordHash(id, passwordHash string) error
	UpdateLoginState(id string, failedAttempts int, lockedUntil *time.Time) error
	UpdateTOTP(id, secret string, enabled bool) error

	CreateRefreshToken(t *RefreshToken) error
	RefreshTokenByHash(hash string) (*RefreshToken, error)
	ListRefreshTokens(userID string) ([]*RefreshToken, error)
	RotateRefreshToken(oldID string, newToken *RefreshToken) error
	RevokeRefreshToken(id string) error
	RevokeRefreshTokenByID(userID, id string) error
	RevokeRefreshTokenFamily(familyID string) error
	RevokeUserRefreshTokens(userID string) error

	CreatePasswordResetToken(t *PasswordResetToken) error
	PasswordResetTokenByHash(hash string) (*PasswordResetToken, error)
	MarkPasswordResetTokenUsed(id string) error
}

// TxRepository extiende Repository con soporte de transacciones.
type TxRepository interface {
	Repository
	WithTx(tx *sql.Tx) Repository
}
