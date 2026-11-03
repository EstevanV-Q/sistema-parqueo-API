package apperr

import (
	"errors"
	"net/http"
)

// Error representa un error de dominio con código estable y estado HTTP.
type Error struct {
	Code       string
	Message    string
	HTTPStatus int
	Err        error
	Details    map[string]any
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Is compara por código de error.
func Is(err error, code string) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

// From convierte un error genérico en Error si ya lo es.
func From(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{
		Code:       "INTERNAL",
		Message:    err.Error(),
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
	}
}

// FromErr es como From pero devuelve nil cuando err es nil. Se usa en rutas
// donde el resultado de una operación exitosa se retorna directamente como
// error (por ejemplo, el retorno de un Exec).
func FromErr(err error) error {
	if err == nil {
		return nil
	}
	return From(err)
}

// Códigos de error estándar del dominio.
const (
	ErrInternal           = "INTERNAL"
	ErrInvalidInput       = "INVALID_INPUT"
	ErrNotFound           = "NOT_FOUND"
	ErrAlreadyExists      = "ALREADY_EXISTS"
	ErrUnauthorized       = "UNAUTHORIZED"
	ErrForbidden          = "FORBIDDEN"
	ErrSpaceOccupied      = "SPACE_OCCUPIED"
	ErrVehicleAlreadyIn   = "VEHICLE_ALREADY_INSIDE"
	ErrTicketNotOpen      = "TICKET_NOT_OPEN"
	ErrTicketAlreadyPaid  = "TICKET_ALREADY_PAID"
	ErrSiteClosed         = "SITE_CLOSED"
	ErrRatePlanNotFound   = "RATE_PLAN_NOT_FOUND"
	ErrDeviceNotFound     = "DEVICE_NOT_FOUND"
	ErrDeviceUnauthorized = "DEVICE_UNAUTHORIZED"
	ErrIdempotencyKey     = "IDEMPOTENCY_KEY_CONFLICT"
	ErrTooManyRequests    = "TOO_MANY_REQUESTS"
	ErrConflict           = "CONFLICT"
)

// New crea un Error con los campos dados.
func New(code, message string, httpStatus int, err error) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Err:        err,
	}
}

// WithDetails añade detalles al error.
func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

// HTTPStatusFrom extrae el estado HTTP de un error.
func HTTPStatusFrom(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.HTTPStatus
	}
	return http.StatusInternalServerError
}
