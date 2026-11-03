package realtime

import (
	"encoding/json"
	"time"
)

// Publisher adapta el Hub a las interfaces de publicación de eventos que
// consumen los módulos de parking y devices.
type Publisher struct {
	hub *Hub
}

// NewPublisher crea un publicador sobre el hub.
func NewPublisher(hub *Hub) *Publisher {
	return &Publisher{hub: hub}
}

// Publish envía un evento JSON a todos los clientes de una sede.
func (p *Publisher) Publish(siteID, event string, payload map[string]any) {
	if p == nil || p.hub == nil || siteID == "" {
		return
	}

	envelope := map[string]any{
		"type":    event,
		"site_id": siteID,
		"data":    payload,
		"at":      time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	p.hub.Publish(siteID, data)
}
