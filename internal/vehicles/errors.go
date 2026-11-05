package vehicles

import "parking-backend/internal/platform/apperr"

var (
	ErrNotFound      = apperr.New(apperr.ErrNotFound, "recurso no encontrado", 404, nil)
	ErrAlreadyExists = apperr.New(apperr.ErrAlreadyExists, "ya existe", 409, nil)
	ErrInvalidInput  = apperr.New(apperr.ErrInvalidInput, "datos inválidos", 400, nil)
)
