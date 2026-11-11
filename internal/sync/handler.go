package syncer

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"mime"
	"net/http"
	"strings"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/ids"
)

// Handler recibe eventos de las APIs de borde y los deja en sync_inbox.
// El procesamiento posterior es idempotente y puede reintentarse sin duplicar.
type Handler struct {
	db         *sql.DB
	token      string
	siteTokens map[string]string
}

func NewHandler(db *sql.DB, token string, siteTokens ...map[string]string) *Handler {
	var tokens map[string]string
	if len(siteTokens) > 0 {
		tokens = siteTokens[0]
	}
	return &Handler{db: db, token: token, siteTokens: tokens}
}

type ingestRequest struct {
	ID            string          `json:"id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"created_at"`
}

func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	if h.token == "" && len(h.siteTokens) == 0 {
		httpserver.Error(w, apperr.New(apperr.ErrNotFound, "sincronización no habilitada", http.StatusNotFound, nil))
		return
	}

	siteID := strings.TrimSpace(r.Header.Get("X-Site-ID"))
	siteBin := ids.StringToBin16(siteID)
	if len(siteBin) != 16 {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "X-Site-ID es requerido", http.StatusBadRequest, nil))
		return
	}

	expected := h.token
	if len(h.siteTokens) > 0 {
		expected = h.siteTokens[siteID]
	}
	if expected == "" {
		httpserver.Error(w, apperr.New(apperr.ErrUnauthorized, "sincronización no autorizada para esta sede", http.StatusUnauthorized, nil))
		return
	}
	authorization := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) ||
		subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authorization, prefix)), []byte(expected)) != 1 {
		httpserver.Error(w, apperr.New(apperr.ErrUnauthorized, "token de sincronización inválido", http.StatusUnauthorized, nil))
		return
	}

	var req ingestRequest
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		httpserver.Error(w, apperr.New(apperr.ErrUnsupportedMedia, "Content-Type debe ser application/json", http.StatusUnsupportedMediaType, nil))
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "cuerpo de sincronización inválido", http.StatusBadRequest, err))
		return
	}
	idBin := ids.StringToBin16(req.ID)
	aggregateBin := ids.StringToBin16(req.AggregateID)
	if len(idBin) != 16 || len(aggregateBin) != 16 ||
		strings.TrimSpace(req.AggregateType) == "" || len(req.AggregateType) > 50 ||
		strings.TrimSpace(req.EventType) == "" || len(req.EventType) > 100 ||
		len(req.Payload) == 0 {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "evento de sincronización incompleto", http.StatusBadRequest, nil))
		return
	}
	now := time.Now().UTC()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	if req.CreatedAt.Before(now.Add(-7*24*time.Hour)) || req.CreatedAt.After(now.Add(24*time.Hour)) {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "created_at fuera de la ventana permitida", http.StatusBadRequest, nil))
		return
	}

	var active bool
	err := h.db.QueryRow(`SELECT is_active FROM sites WHERE id = ? LIMIT 1`, siteBin).Scan(&active)
	if err == sql.ErrNoRows || (err == nil && !active) {
		httpserver.Error(w, apperr.New(apperr.ErrForbidden, "sede no autorizada", http.StatusForbidden, nil))
		return
	}
	if err != nil {
		httpserver.Error(w, apperr.From(err))
		return
	}

	_, err = h.db.Exec(`
		INSERT IGNORE INTO sync_inbox
			(id, site_id, aggregate_type, aggregate_id, event_type, payload, event_created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		idBin, siteBin, req.AggregateType, aggregateBin, req.EventType, string(req.Payload), req.CreatedAt)
	if err != nil {
		httpserver.Error(w, apperr.From(err))
		return
	}

	httpserver.JSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
