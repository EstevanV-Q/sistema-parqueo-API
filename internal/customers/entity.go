package customers

import "time"

// Customer representa un cliente del parqueo.
type Customer struct {
	ID        string    `json:"id"`
	Document  string    `json:"document"`
	Name      string    `json:"name"`
	Phone     *string   `json:"phone,omitempty"`
	Email     *string   `json:"email,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}
