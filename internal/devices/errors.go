package devices

import "parking-backend/internal/platform/apperr"

var (
	ErrNotFound        = apperr.New(apperr.ErrNotFound, "recurso no encontrado", 404, nil)
	ErrDeviceUnauthorized = apperr.New(apperr.ErrDeviceUnauthorized, "dispositivo no autorizado", 401, nil)
	ErrDeviceInactive     = apperr.New(apperr.ErrForbidden, "dispositivo inactivo", 403, nil)
	ErrInvalidEventType   = apperr.New(apperr.ErrInvalidInput, "tipo de evento no soportado", 400, nil)
)
