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

// Tipos de token. El middleware y los servicios deben validar siempre el tipo
// antes de aceptar un token: un refresh token no es un access token.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
	TokenTypeWS      = "ws"
)

// Claims contiene los claims de los JWT emitidos por el sistema.
type Claims struct {
	UserID    string  `json:"user_id"`
	Username  string  `json:"username"`
	Role      string  `json:"role"`
	SiteID    *string `json:"site_id,omitempty"`
	TokenType string  `json:"token_type"`
	jwt.RegisteredClaims
}

func nowFunc(now []clock.NowFunc) clock.NowFunc {
	if len(now) > 0 && now[0] != nil {
		return now[0]
	}
	return time.Now
}

func sign(secret string, claims Claims) (string, time.Time, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "v1"
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, claims.ExpiresAt.Time, nil
}

// AccessToken genera un JWT de acceso firmado con HS256.
func AccessToken(secret, issuer, audience, userID, username, role string, siteID *string, expiry time.Duration, now ...clock.NowFunc) (string, time.Time, error) {
	nowFn := nowFunc(now)
	expiresAt := nowFn().Add(expiry)
	issuedAt := nowFn()
	claims := Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		SiteID:    siteID,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			Subject:   userID,
		},
	}
	return sign(secret, claims)
}

// RefreshToken genera un JWT de refresh. No incluye rol ni sede porque solo
// sirve para solicitar un nuevo par de tokens en /auth/refresh.
func RefreshToken(secret, issuer, audience, userID string, expiry time.Duration, now ...clock.NowFunc) (string, time.Time, error) {
	nowFn := nowFunc(now)
	expiresAt := nowFn().Add(expiry)
	issuedAt := nowFn()

	// jti aleatorio: garantiza que dos refresh tokens del mismo usuario en el
	// mismo segundo sean distintos (necesario para la rotación).
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", time.Time{}, err
	}

	claims := Claims{
		UserID:    userID,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			Subject:   userID,
			ID:        hex.EncodeToString(jtiBytes),
		},
	}
	return sign(secret, claims)
}

// WSTicket genera un token efímero para abrir el WebSocket. Evita exponer el
// access token de larga vida en la query string.
func WSTicket(secret, issuer, audience, userID, username, role string, siteID *string, expiry time.Duration, now ...clock.NowFunc) (string, time.Time, error) {
	nowFn := nowFunc(now)
	expiresAt := nowFn().Add(expiry)
	issuedAt := nowFn()
	claims := Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		SiteID:    siteID,
		TokenType: TokenTypeWS,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			Subject:   userID,
		},
	}
	return sign(secret, claims)
}

func parseClaims(secret, issuer, audience, tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, errors.New("token requerido")
	}
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("método de firma inesperado: %v", token.Header["alg"])
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("token inválido")
	}
	return claims, nil
}

// ParseAccessToken valida y devuelve los claims de un token de acceso.
func ParseAccessToken(secret, issuer, audience, tokenStr string) (*Claims, error) {
	claims, err := parseClaims(secret, issuer, audience, tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, errors.New("tipo de token inválido")
	}
	if claims.UserID == "" || claims.Role == "" || claims.Subject != claims.UserID {
		return nil, errors.New("claims de acceso incompletos")
	}
	return claims, nil
}

// ParseRefreshToken valida un refresh token y devuelve el userID (sub).
func ParseRefreshToken(secret, issuer, audience, tokenStr string) (string, error) {
	claims, err := parseClaims(secret, issuer, audience, tokenStr)
	if err != nil {
		return "", err
	}
	if claims.TokenType != TokenTypeRefresh {
		return "", errors.New("tipo de token inválido")
	}
	if claims.UserID == "" || claims.Subject != claims.UserID {
		return "", errors.New("claims de refresh incompletos")
	}
	return claims.UserID, nil
}

// ParseWSTicket valida el token efímero usado para abrir el WebSocket.
func ParseWSTicket(secret, issuer, audience, tokenStr string) (*Claims, error) {
	claims, err := parseClaims(secret, issuer, audience, tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeWS {
		return nil, errors.New("tipo de token inválido")
	}
	if claims.UserID == "" || claims.Role == "" || claims.Subject != claims.UserID {
		return nil, errors.New("claims de websocket incompletos")
	}
	return claims, nil
}
