// Package pagination estandariza los parámetros de paginación de los listados.
package pagination

import (
	"net/http"
	"strconv"
)

const (
	// DefaultLimit es el tamaño de página por defecto.
	DefaultLimit = 50
	// MaxLimit es el máximo permitido para evitar consultas gigantes.
	MaxLimit = 200
)

// Params contiene los parámetros normalizados de paginación.
type Params struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// FromRequest lee `limit` y `offset` de la query string aplicando límites.
func FromRequest(r *http.Request) Params {
	limit := DefaultLimit
	offset := 0

	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	if raw := r.URL.Query().Get("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			offset = n
		}
	}

	return Params{Limit: limit, Offset: offset}
}

// Meta describe la página devuelta.
type Meta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total,omitempty"`
}

// Meta construye los metadatos de la página.
func (p Params) Meta(total int) Meta {
	return Meta{Limit: p.Limit, Offset: p.Offset, Total: total}
}
