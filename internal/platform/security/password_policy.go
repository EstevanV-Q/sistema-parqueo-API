package security

import (
	"fmt"
	"strings"
)

// ValidateSeedPassword aplica una política mínima a las contraseñas usadas por
// el comando seed. No se aplica al registro normal para no romper clientes
// existentes, pero el seed solo debe crear credenciales robustas.
func ValidateSeedPassword(password string) error {
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
