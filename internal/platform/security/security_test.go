package security

import (
	"testing"
	"time"
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
	siteID := "0190aabbccddeeff0011223344556677"
	token, expires, err := AccessToken("secret", "user-1", "admin", "admin", &siteID, time.Minute)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if expires.Before(time.Now()) {
		t.Fatal("la expiración debe estar en el futuro")
	}

	claims, err := ParseAccessToken("secret", token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != "user-1" || claims.Role != "admin" {
		t.Fatalf("claims inesperados: %+v", claims)
	}
	if claims.SiteID == nil || *claims.SiteID != siteID {
		t.Fatalf("site_id inesperado: %+v", claims.SiteID)
	}

	if _, err := ParseAccessToken("otro-secret", token); err == nil {
		t.Fatal("un secreto distinto debe invalidar el token")
	}
}

func TestJWTRefreshRoundTrip(t *testing.T) {
	token, _, err := RefreshToken("secret", "user-1", time.Hour)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	userID, err := ParseRefreshToken("secret", token)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("userID = %s, quiere user-1", userID)
	}
}
