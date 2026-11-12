package security_test

import (
	"testing"
	"time"

	"parking-backend/internal/platform/security"
)

func TestValidateTOTPRFC6238(t *testing.T) {
	// Secreto de prueba del RFC 6238 (ASCII "12345678901234567890").
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if !security.ValidateTOTP(secret, "287082", time.Unix(59, 0)) {
		t.Fatal("se esperaba válido el código TOTP del RFC 6238")
	}
	if security.ValidateTOTP(secret, "000000", time.Unix(59, 0)) {
		t.Fatal("un código incorrecto no debe validar")
	}
}
