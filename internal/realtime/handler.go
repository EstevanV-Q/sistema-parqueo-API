package realtime

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"

	"parking-backend/internal/config"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/security"
)

// Handler maneja las conexiones WebSocket de tiempo real.
type Handler struct {
	hub        *Hub
	cfg        *config.Config
	originPats []string
}

func NewHandler(hub *Hub, cfg *config.Config) *Handler {
	return &Handler{
		hub:        hub,
		cfg:        cfg,
		originPats: originHostPatterns(cfg.CORSAllowedOrigins),
	}
}

func originHostPatterns(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			continue
		}
		out = append(out, u.Host)
	}
	return out
}

// OccupancyWS maneja el WebSocket de ocupación por sede.
// Ruta: GET /api/v1/ws/occupancy?site_id=...&ticket=<ws-ticket>
func (h *Handler) OccupancyWS(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "site_id requerido", http.StatusBadRequest, nil))
		return
	}
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	claims, err := security.ParseWSTicket(h.cfg.JWTSecret, h.cfg.JWTIssuer, h.cfg.JWTAudience, ticket)
	if err != nil {
		httpserver.Error(w, apperr.New(apperr.ErrUnauthorized, "ticket de websocket inválido o expirado", http.StatusUnauthorized, err))
		return
	}
	if claims.Role != "admin" {
		if claims.SiteID == nil || *claims.SiteID != siteID {
			httpserver.Error(w, apperr.New(apperr.ErrForbidden, "no tiene acceso a esta sede", http.StatusForbidden, nil))
			return
		}
	}

	opts := websocket.AcceptOptions{OriginPatterns: h.originPats}
	conn, err := websocket.Accept(w, r, &opts)
	if err != nil {
		httpserver.Error(w, apperr.New(apperr.ErrInternal, "no se pudo abrir websocket", http.StatusInternalServerError, err))
		return
	}

	c := h.hub.Register(conn, siteID)
	defer h.hub.Unregister(c)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Bucle de lectura para mantener la conexión viva y detectar desconexiones.
	for {
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
	}
}
