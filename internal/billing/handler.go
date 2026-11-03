package billing

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"parking-backend/internal/audit"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/security/ctx"
	"parking-backend/internal/platform/validate"
	"parking-backend/internal/shared/pagination"
)

type Handler struct {
	svc     *Service
	auditor *audit.Service
}

func NewHandler(svc *Service, auditor *audit.Service) *Handler {
	return &Handler{svc: svc, auditor: auditor}
}

type createPaymentReq struct {
	TicketID       *string `json:"ticket_id"`
	SubscriptionID *string `json:"subscription_id"`
	AmountCents    int64   `json:"amount_cents" validate:"required,gt=0"`
	Currency       string  `json:"currency" validate:"omitempty,len=3"`
	Method         string  `json:"method" validate:"omitempty,oneof=cash card transfer abonnement waived"`
	Reference      string  `json:"reference" validate:"max=100"`
	CashierUserID  *string `json:"cashier_user_id"`
	IdempotencyKey *string `json:"idempotency_key" validate:"omitempty,max=128"`
}

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req createPaymentReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}

	if req.Currency == "" {
		req.Currency = "COP"
	}
	// El cajero por defecto es el usuario autenticado.
	if req.CashierUserID == nil {
		if uid, ok := ctx.UserIDFromCtx(r.Context()); ok && uid != "" {
			req.CashierUserID = &uid
		}
	}
	// La clave de idempotencia también puede venir por header.
	if req.IdempotencyKey == nil {
		if key := r.Header.Get("Idempotency-Key"); key != "" {
			req.IdempotencyKey = &key
		}
	}

	p, err := h.svc.RegisterPayment(req.TicketID, req.SubscriptionID, req.AmountCents, req.Currency, req.Method, req.Reference, req.CashierUserID, req.IdempotencyKey)
	if err != nil {
		httpserver.Error(w, err)
		return
	}

	if h.auditor != nil {
		_ = h.auditor.Record(r.Context(), "payment.create", "payment", p.ID, nil, map[string]any{
			"ticket_id":       p.TicketID,
			"subscription_id": p.SubscriptionID,
			"amount_cents":    p.AmountCents,
			"method":          p.Method,
			"status":          p.Status,
		})
	}

	httpserver.JSON(w, http.StatusCreated, p)
}

type refundReq struct {
	Reason string `json:"reason" validate:"max=200"`
}

// RefundPayment anula o reembolsa un pago completado.
func (h *Handler) RefundPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req refundReq
	_ = httpserver.Decode(r, &req)

	p, err := h.svc.RefundPayment(id, req.Reason)
	if err != nil {
		httpserver.Error(w, err)
		return
	}

	if h.auditor != nil {
		_ = h.auditor.Record(r.Context(), "payment.refund", "payment", p.ID,
			map[string]any{"status": PaymentCompleted},
			map[string]any{"status": PaymentRefunded, "reason": req.Reason},
		)
	}

	httpserver.JSON(w, http.StatusOK, p)
}

func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.svc.repo.ByID(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, p)
}

func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "site_id requerido", http.StatusBadRequest, nil))
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	page := pagination.FromRequest(r)

	payments, err := h.svc.repo.ListBySite(siteID, from, to, page.Limit, page.Offset)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"data": payments,
		"meta": page.Meta(0),
	})
}

// CajaDiaria devuelve el arqueo de caja de una sede en una fecha.
func (h *Handler) CajaDiaria(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	date := r.URL.Query().Get("date")
	if siteID == "" || date == "" {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "site_id y date son requeridos", http.StatusBadRequest, nil))
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}
	caja, err := h.svc.repo.CajaDiaria(siteID, date)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, caja)
}
