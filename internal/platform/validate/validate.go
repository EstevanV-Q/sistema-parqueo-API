// Package validate envuelve go-playground/validator para que los handlers
// validen DTOs con etiquetas `validate:"..."` y devuelvan errores de dominio
// con detalles por campo.
package validate

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"

	"parking-backend/internal/platform/apperr"
)

var (
	once sync.Once
	v    *validator.Validate
)

// V devuelve la instancia compartida del validador.
func V() *validator.Validate {
	once.Do(func() {
		v = validator.New()
	})
	return v
}

// Struct valida un DTO y convierte los errores en un apperr con detalles.
func Struct(s any) error {
	err := V().Struct(s)
	if err == nil {
		return nil
	}

	var ve validator.ValidationErrors
	if ok := asValidationErrors(err, &ve); ok {
		details := make(map[string]any, len(ve))
		for _, fe := range ve {
			details[fieldName(fe)] = message(fe)
		}
		return apperr.New(apperr.ErrInvalidInput, "datos inválidos", http.StatusBadRequest, err).WithDetails(details)
	}

	return apperr.New(apperr.ErrInvalidInput, "datos inválidos", http.StatusBadRequest, err)
}

func asValidationErrors(err error, target *validator.ValidationErrors) bool {
	if ve, ok := err.(validator.ValidationErrors); ok {
		*target = ve
		return true
	}
	return false
}

func fieldName(fe validator.FieldError) string {
	ns := fe.StructNamespace()
	if ns == "" {
		return strings.ToLower(fe.Field())
	}
	parts := strings.Split(ns, ".")
	return strings.ToLower(parts[len(parts)-1])
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "es obligatorio"
	case "email":
		return "debe ser un correo válido"
	case "min":
		return fmt.Sprintf("debe tener al menos %s caracteres", fe.Param())
	case "max":
		return fmt.Sprintf("debe tener como máximo %s caracteres", fe.Param())
	case "gt":
		return fmt.Sprintf("debe ser mayor que %s", fe.Param())
	case "gte":
		return fmt.Sprintf("debe ser mayor o igual que %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("debe ser uno de: %s", fe.Param())
	case "uuid":
		return "debe ser un identificador válido"
	case "datetime":
		return fmt.Sprintf("debe tener formato %s", fe.Param())
	default:
		return fmt.Sprintf("no cumple la regla %s", fe.Tag())
	}
}
