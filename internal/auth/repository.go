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

	CreateRefreshToken(t *RefreshToken) error
	RefreshTokenByHash(hash string) (*RefreshToken, error)
	RotateRefreshToken(oldID string, newToken *RefreshToken) error
	RevokeRefreshToken(id string) error
	RevokeRefreshTokenFamily(familyID string) error
	RevokeUserRefreshTokens(userID string) error
}

// TxRepository extiende Repository con soporte de transacciones.
type TxRepository interface {
	Repository
	WithTx(tx *sql.Tx) Repository
}
