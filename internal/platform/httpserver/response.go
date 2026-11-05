// Package httpserver centraliza las respuestas HTTP y la decodificación de
// cuerpos JSON para que todos los handlers del sistema se comporten igual.
package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"parking-backend/internal/platform/apperr"
)

// JSON escribe una respuesta JSON con el estado HTTP indicado.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil || status == http.StatusNoContent {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// Error escribe un error de dominio en formato JSON uniforme.
// Si err es nil, responde 500 con código INTERNAL para evitar panic.
func Error(w http.ResponseWriter, err error) {
	if err == nil {
		err = apperr.New(apperr.ErrInternal, "error interno", http.StatusInternalServerError, nil)
	}
	e := apperr.From(err)
	JSON(w, e.HTTPStatus, map[string]any{
		"code":    e.Code,
		"message": e.Message,
		"details": e.Details,
	})
}

// Decode decodifica el cuerpo JSON en dst y devuelve un error de dominio si el
// cuerpo es inválido o viene vacío.
func Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return apperr.New(apperr.ErrInvalidInput, "el cuerpo de la petición es requerido", http.StatusBadRequest, err)
		}
		return apperr.New(apperr.ErrInvalidInput, "cuerpo JSON inválido", http.StatusBadRequest, err)
	}
	return nil
}

// NoContent responde 204 sin cuerpo.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
