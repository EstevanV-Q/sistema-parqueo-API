package realtime

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Hub gestiona las conexiones WebSocket agrupadas por sala (site_id).
type Hub struct {
	mu        sync.RWMutex
	rooms     map[string]map[*connection]struct{}
	broadcast chan message
}

// connection envuelve un WebSocket con su sala y serializa las escrituras.
type connection struct {
	conn *websocket.Conn
	room string
	mu   sync.Mutex
}

func (c *connection) write(ctx context.Context, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Write(ctx, websocket.MessageText, data)
}

// message a broadcastar a una sala.
type message struct {
	room string
	data []byte
}

// NewHub crea un hub vacío.
func NewHub() *Hub {
	return &Hub{
		rooms:     make(map[string]map[*connection]struct{}),
		broadcast: make(chan message, 512),
	}
}

// Run inicia el bucle de broadcast (debe ejecutarse en una goroutine).
func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.mu.RLock()
		conns := make([]*connection, 0, len(h.rooms[msg.room]))
		for c := range h.rooms[msg.room] {
			conns = append(conns, c)
		}
		h.mu.RUnlock()

		for _, c := range conns {
			go func(conn *connection, payload []byte) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := conn.write(ctx, payload); err != nil {
					h.Unregister(conn)
					_ = conn.conn.Close(websocket.StatusInternalError, "write error")
				}
			}(c, msg.data)
		}
	}
}

// Register añade una conexión a una sala y devuelve el manejador de conexión.
func (h *Hub) Register(conn *websocket.Conn, room string) *connection {
	c := &connection{conn: conn, room: room}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*connection]struct{})
	}
	h.rooms[room][c] = struct{}{}
	return c
}

// Unregister remueve una conexión de su sala usando el mismo puntero.
func (h *Hub) Unregister(c *connection) {
	if c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.rooms[c.room]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.rooms, c.room)
		}
	}
}

// Publish envía un mensaje a todas las conexiones de una sala. Si el buffer
// está lleno, descarta el mensaje para no bloquear la petición de negocio.
func (h *Hub) Publish(room string, data []byte) {
	select {
	case h.broadcast <- message{room: room, data: data}:
	default:
	}
}

// RoomSize devuelve cuántas conexiones hay en una sala (útil para métricas).
func (h *Hub) RoomSize(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room])
}
