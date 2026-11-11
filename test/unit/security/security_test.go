package security_test

import (
	"testing"
	"time"

	. "parking-backend/internal/platform/security"
)

func TestPasswordHashAndCompare(t *testing.T) {
	hash, err := HashPassword("Admin123!")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "Admin123!" {
		t.Fatal("la contraseña no debe guardarse en claro")
	}

	ok, err := ComparePassword("Admin123!", hash)
	if err != nil {
		t.Fatalf("ComparePassword: %v", err)
	}
	if !ok {
		t.Fatal("la contraseña correcta debe validar")
	}

	ok, err = ComparePassword("incorrecta", hash)
	if err != nil {
		t.Fatalf("ComparePassword incorrecta: %v", err)
	}
	if ok {
		t.Fatal("una contraseña incorrecta no debe validar")
	}
}

func TestHashToken(t *testing.T) {
	token := "device-api-key-001"
	hash := HashToken(token)
	if hash == token {
		t.Fatal("el token no debe guardarse en claro")
	}
	if !CompareToken(token, hash) {
		t.Fatal("CompareToken debe aceptar el token correcto")
	}
	if CompareToken("otro", hash) {
		t.Fatal("CompareToken debe rechazar un token distinto")
	}
}

func TestJWTAccessRoundTrip(t *testing.T) {
	const issuer = "parking-backend"
	const audience = "parking-api"
	siteID := "0190aabbccddeeff0011223344556677"
	token, expires, err := AccessToken("secret", issuer, audience, "user-1", "user-1", "admin", &siteID, time.Minute)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if expires.Before(time.Now()) {
		t.Fatal("la expiración debe estar en el futuro")
	}

	claims, err := ParseAccessToken("secret", issuer, audience, token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != "user-1" || claims.Role != "admin" {
		t.Fatalf("claims inesperados: %+v", claims)
	}
	if claims.SiteID == nil || *claims.SiteID != siteID {
		t.Fatalf("site_id inesperado: %+v", claims.SiteID)
	}

	if _, err := ParseAccessToken("otro-secret", issuer, audience, token); err == nil {
		t.Fatal("un secreto distinto debe invalidar el token")
	}
}

func TestJWTRefreshRoundTrip(t *testing.T) {
	const issuer = "parking-backend"
	const audience = "parking-api"
	token, _, err := RefreshToken("secret", issuer, audience, "user-1", time.Hour)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	userID, err := ParseRefreshToken("secret", issuer, audience, token)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("userID = %s, quiere user-1", userID)
	}
}

func TestRefreshTokenRejectedAsAccessToken(t *testing.T) {
	const issuer = "parking-backend"
	const audience = "parking-api"
	refresh, _, err := RefreshToken("secret", issuer, audience, "user-1", time.Hour)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if _, err := ParseAccessToken("secret", issuer, audience, refresh); err == nil {
		t.Fatal("un refresh token no debe aceptarse como access token")
	}
}

func TestAccessTokenRejectedAsRefreshToken(t *testing.T) {
	const issuer = "parking-backend"
	const audience = "parking-api"
	access, _, err := AccessToken("secret", issuer, audience, "user-1", "user-1", "admin", nil, time.Minute)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if _, err := ParseRefreshToken("secret", issuer, audience, access); err == nil {
		t.Fatal("un access token no debe aceptarse como refresh token")
	}
}

func TestTokenRejectsWrongIssuerOrAudience(t *testing.T) {
	const issuer = "parking-backend"
	const audience = "parking-api"
	access, _, err := AccessToken("secret", issuer, audience, "user-1", "user-1", "admin", nil, time.Minute)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if _, err := ParseAccessToken("secret", "otro-issuer", audience, access); err == nil {
		t.Fatal("issuer distinto debe rechazarse")
	}
	if _, err := ParseAccessToken("secret", issuer, "otra-audience", access); err == nil {
		t.Fatal("audience distinta debe rechazarse")
	}
}

func TestAccessTokenRejectsEmptyClaims(t *testing.T) {
	const issuer = "parking-backend"
	const audience = "parking-api"
	access, _, err := AccessToken("secret", issuer, audience, "", "", "admin", nil, time.Minute)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if _, err := ParseAccessToken("secret", issuer, audience, access); err == nil {
		t.Fatal("un token sin user_id no debe aceptarse")
	}
}

func TestWSTicketRoundTrip(t *testing.T) {
	const issuer = "parking-backend"
	const audience = "parking-api"
	ticket, _, err := WSTicket("secret", issuer, audience, "user-1", "user-1", "admin", nil, 30*time.Second)
	if err != nil {
		t.Fatalf("WSTicket: %v", err)
	}
	claims, err := ParseWSTicket("secret", issuer, audience, ticket)
	if err != nil {
		t.Fatalf("ParseWSTicket: %v", err)
	}
	if claims.TokenType != TokenTypeWS || claims.UserID != "user-1" {
		t.Fatalf("claims inesperados: %+v", claims)
	}
	if _, err := ParseAccessToken("secret", issuer, audience, ticket); err == nil {
		t.Fatal("un ws ticket no debe aceptarse como access token")
	}
}
