package realtime

import (
	"net/http"

	"github.com/coder/websocket"
)

// Handler maneja las conexiones WebSocket de tiempo real.
type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

var upgrader = websocket.AcceptOptions{
	OriginPatterns: []string{"*"},
}

// OccupancyWS maneja el WebSocket de ocupación por sede.
// Ruta: GET /api/v1/ws/occupancy?site_id=...
func (h *Handler) OccupancyWS(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		http.Error(w, "site_id requerido", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &upgrader)
	if err != nil {
		http.Error(w, "no se pudo abrir websocket", http.StatusInternalServerError)
		return
	}

	c := h.hub.Register(conn, siteID)
	defer h.hub.Unregister(c)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Bucle de lectura para mantener la conexión viva y detectar desconexiones.
	for {
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
	}
}
