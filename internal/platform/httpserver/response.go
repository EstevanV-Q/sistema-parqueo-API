// Package httpserver centraliza las respuestas HTTP y la decodificación de
// cuerpos JSON para que todos los handlers del sistema se comporten igual.
package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"parking-backend/internal/platform/apperr"
)

// MaxBytes limita el tamaño del cuerpo de las peticiones HTTP.
func MaxBytes(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, max)
			}
			next.ServeHTTP(w, r)
		})
	}
}

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
	message := e.Message
	var details map[string]any
	if e.Code == apperr.ErrInternal {
		// Nunca exponer detalles de SQL, rutas o errores internos al cliente.
		message = "error interno"
		slog.Error("error interno en petición", "message", e.Message, "error", e.Err)
	} else {
		details = e.Details
	}
	JSON(w, e.HTTPStatus, map[string]any{
		"code":    e.Code,
		"message": message,
		"details": details,
	})
}

// Decode decodifica el cuerpo JSON en dst y devuelve un error de dominio si el
// cuerpo es inválido o viene vacío.
func Decode(r *http.Request, dst any) error {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return apperr.New(apperr.ErrUnsupportedMedia, "Content-Type debe ser application/json", http.StatusUnsupportedMediaType, err)
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return apperr.New(apperr.ErrInvalidInput, "el cuerpo de la petición es requerido", http.StatusBadRequest, err)
		}
		return apperr.New(apperr.ErrInvalidInput, "cuerpo JSON inválido", http.StatusBadRequest, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return apperr.New(apperr.ErrInvalidInput, "el cuerpo contiene datos JSON adicionales", http.StatusBadRequest, err)
	}
	return nil
}

// NoContent responde 204 sin cuerpo.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
