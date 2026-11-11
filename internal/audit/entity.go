package audit

import "time"

// Entry representa un registro append-only de auditoría.
type Entry struct {
	ID         string         `json:"id"`
	UserID     *string        `json:"user_id,omitempty"`
	SiteID     *string        `json:"site_id,omitempty"`
	Action     string         `json:"action"`
	EntityType string         `json:"entity_type"`
	EntityID   *string        `json:"entity_id,omitempty"`
	OldValues  map[string]any `json:"old_values,omitempty"`
	NewValues  map[string]any `json:"new_values,omitempty"`
	IPAddress  *string        `json:"ip_address,omitempty"`
	UserAgent  *string        `json:"user_agent,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Filter permite acotar la consulta de auditoría.
type Filter struct {
	EntityType string
	EntityID   string
	UserID     string
	SiteID     string
	Action     string
	From       string
	To         string
}
