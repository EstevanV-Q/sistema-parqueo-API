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

// SiteResolver resuelve la sede de un ticket o abono para aplicar autorización
// horizontal en pagos.
type SiteResolver interface {
	TicketSite(ticketID string) (string, error)
	SubscriptionSite(subscriptionID string) (string, error)
}

type Handler struct {
	svc      *Service
	auditor  *audit.Service
	resolver SiteResolver
}

func NewHandler(svc *Service, auditor *audit.Service, resolver SiteResolver) *Handler {
	return &Handler{svc: svc, auditor: auditor, resolver: resolver}
}

func nonEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (h *Handler) paymentSite(p *Payment) (string, error) {
	if h.resolver == nil {
		return "", apperr.New(apperr.ErrInternal, "error interno", http.StatusInternalServerError, nil)
	}
	if p.TicketID != nil && *p.TicketID != "" {
		return h.resolver.TicketSite(*p.TicketID)
	}
	if p.SubscriptionID != nil && *p.SubscriptionID != "" {
		return h.resolver.SubscriptionSite(*p.SubscriptionID)
	}
	return "", apperr.New(apperr.ErrInvalidInput, "el pago no tiene ticket ni abono asociado", http.StatusBadRequest, nil)
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

	ticketID := nonEmpty(req.TicketID)
	subscriptionID := nonEmpty(req.SubscriptionID)
	if (ticketID == "") == (subscriptionID == "") {
		httpserver.Error(w, apperr.New(apperr.ErrInvalidInput, "debe indicar exactamente un ticket_id o subscription_id", http.StatusBadRequest, nil))
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

	if h.resolver == nil {
		httpserver.Error(w, apperr.New(apperr.ErrInternal, "error interno", http.StatusInternalServerError, nil))
		return
	}
	var siteID string
	var err error
	if ticketID != "" {
		siteID, err = h.resolver.TicketSite(ticketID)
	} else {
		siteID, err = h.resolver.SubscriptionSite(subscriptionID)
	}
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}

	p, err := h.svc.RegisterPayment(req.TicketID, req.SubscriptionID, req.AmountCents, req.Currency, req.Method, req.Reference, req.CashierUserID, req.IdempotencyKey)
	if err != nil {
		httpserver.Error(w, err)
		return
	}

	if h.auditor != nil {
		_ = h.auditor.Record(r.Context(), "payment.create", "payment", p.ID, nil, map[string]any{
			"site_id":         siteID,
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

	current, err := h.svc.repo.ByID(id)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	siteID, err := h.paymentSite(current)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
		httpserver.Error(w, err)
		return
	}

	p, err := h.svc.RefundPayment(id, req.Reason)
	if err != nil {
		httpserver.Error(w, err)
		return
	}

	if h.auditor != nil {
		_ = h.auditor.Record(r.Context(), "payment.refund", "payment", p.ID,
			map[string]any{"status": PaymentCompleted},
			map[string]any{"site_id": siteID, "status": PaymentRefunded, "reason": req.Reason},
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
	siteID, err := h.paymentSite(p)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := middleware.CheckSiteAccess(r.Context(), siteID); err != nil {
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
