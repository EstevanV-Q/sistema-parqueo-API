package auth

import (
	"parking-backend/internal/platform/apperr"
)

var (
	ErrInvalidInput = apperr.New(apperr.ErrInvalidInput, "datos inválidos", 400, nil)
	ErrNotFound     = apperr.New(apperr.ErrNotFound, "recurso no encontrado", 404, nil)
	ErrAlreadyExists = apperr.New(apperr.ErrAlreadyExists, "ya existe", 409, nil)
	ErrUnauthorized = apperr.New(apperr.ErrUnauthorized, "no autorizado", 401, nil)
	ErrForbidden    = apperr.New(apperr.ErrForbidden, "prohibido", 403, nil)
	ErrInactive     = apperr.New(apperr.ErrForbidden, "usuario inactivo", 403, nil)
	ErrInvalidCredentials = apperr.New(apperr.ErrUnauthorized, "credenciales inválidas", 401, nil)
)
