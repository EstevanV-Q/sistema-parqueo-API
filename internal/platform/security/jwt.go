package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"parking-backend/internal/platform/clock"
)

// Claims contiene los claims del JWT de acceso.
type Claims struct {
	UserID   string  `json:"user_id"`
	Username string  `json:"username"`
	Role     string  `json:"role"`
	SiteID   *string `json:"site_id,omitempty"`
	jwt.RegisteredClaims
}

// AccessToken genera un JWT de acceso firmado con HS256.
// Si se proporciona now, se usa para calcular expiración y issued_at (útil en tests).
func AccessToken(secret string, userID, username, role string, siteID *string, expiry time.Duration, now ...clock.NowFunc) (string, time.Time, error) {
	nowFn := time.Now
	if len(now) > 0 && now[0] != nil {
		nowFn = now[0]
	}
	expiresAt := nowFn().Add(expiry)
	issuedAt := nowFn()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		SiteID:   siteID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// RefreshToken genera un JWT de refresh (sin claims de negocio, solo sub y exp).
// Si se proporciona now, se usa para calcular expiración y issued_at (útil en tests).
func RefreshToken(secret string, userID string, expiry time.Duration, now ...clock.NowFunc) (string, time.Time, error) {
	nowFn := time.Now
	if len(now) > 0 && now[0] != nil {
		nowFn = now[0]
	}
	expiresAt := nowFn().Add(expiry)
	issuedAt := nowFn()

	// jti aleatorio: garantiza que dos refresh tokens del mismo usuario en el
	// mismo segundo sean distintos (necesario para la rotación).
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", time.Time{}, err
	}

	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		Subject:   userID,
		ID:        hex.EncodeToString(jtiBytes),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// ParseAccessToken valida y devuelve los claims de un token de acceso.
func ParseAccessToken(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("token inválido")
}

// ParseRefreshToken valida un token de refresh y devuelve el userID (sub).
func ParseRefreshToken(secret, tokenStr string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims.Subject, nil
	}
	return "", errors.New("token inválido")
}
