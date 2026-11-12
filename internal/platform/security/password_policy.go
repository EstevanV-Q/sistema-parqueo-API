package security

import (
	"fmt"
	"strings"
)

// ValidatePassword aplica la política de contraseñas para altas y cambios.
func ValidatePassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("debe tener al menos 12 caracteres")
	}
	if len(password) > 128 {
		return fmt.Errorf("no debe superar 128 caracteres")
	}
	lower := strings.ToLower(password)
	for _, marker := range []string{
		"cambiar", "change", "password", "contraseña", "admin123",
		"oper123", "audit123", "device-api-key", "example", "<", ">",
	} {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("parece un placeholder o una contraseña de ejemplo")
		}
	}
	return nil
}

// ValidateSeedPassword mantiene el nombre histórico para el comando seed.
func ValidateSeedPassword(password string) error {
	return ValidatePassword(password)
}
