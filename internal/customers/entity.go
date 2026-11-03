package customers

import "time"

// Customer representa un cliente del parqueo.
type Customer struct {
	ID        string
	Document  string
	Name      string
	Phone     *string
	Email     *string
	IsActive  bool
	CreatedAt time.Time
}
