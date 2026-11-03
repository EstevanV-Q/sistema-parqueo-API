package plate

import (
	"strings"
	"unicode"
)

// Normalize limpia una placa: mayúsculas, sin espacios ni guiones.
func Normalize(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ToUpper(p)
	var b strings.Builder
	for _, r := range p {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsValid verifica que la placa no esté vacía tras normalizar.
func IsValid(p string) bool {
	return Normalize(p) != ""
}
