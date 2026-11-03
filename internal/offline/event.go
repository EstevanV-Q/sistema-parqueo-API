// Package offline implementa una cola local persistente para operar sin red y
// sincronizar eventos con la API de forma idempotente.
package offline

import "time"

// Event es una operación capturada en el borde (entrada/salida/pago).
type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
	Attempts  int            `json:"attempts"`
}
