package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"parking-backend/internal/audit"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/middleware"
	"parking-backend/internal/platform/security/ctx"
	"parking-backend/internal/platform/validate"
)

type Handler struct {
	svc          *Service
	loginLimiter *middleware.KeyLimiter
	auditor      *audit.Service
}

func NewHandler(svc *Service, loginLimiters ...*middleware.KeyLimiter) *Handler {
	h := &Handler{svc: svc}
	if len(loginLimiters) > 0 {
		h.loginLimiter = loginLimiters[0]
	}
	return h
}

// WithAuditor inyecta el servicio de auditoría para registrar accesos.
func (h *Handler) WithAuditor(a *audit.Service) *Handler {
	h.auditor = a
	return h
}

type registerReq struct {
	Username string  `json:"username" validate:"required,min=3,max=100"`
	Email    string  `json:"email" validate:"required,email,max=255"`
	Password string  `json:"password" validate:"required,min=12,max=128"`
	Role     string  `json:"role" validate:"required,oneof=admin operator auditor"`
	SiteID   *string `json:"site_id,omitempty"`
}

type loginReq struct {
	Username string `json:"username" validate:"required,max=100"`
	Password string `json:"password" validate:"required,max=128"`
	TOTPCode string `json:"totp_code" validate:"omitempty,len=6,numeric"`
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

// userResp es el DTO público de usuario. Nunca debe incluir PasswordHash.
type userResp struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	Role        Role    `json:"role"`
	SiteID      *string `json:"site_id,omitempty"`
	Active      bool    `json:"active"`
	TOTPEnabled bool    `json:"totp_enabled"`
	CreatedAt   string  `json:"created_at,omitempty"`
	LastLogin   *string `json:"last_login,omitempty"`
}

func toUserResp(u *User) userResp {
	if u == nil {
		return userResp{}
	}
	return userResp{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Role:        u.Role,
		SiteID:      u.SiteID,
		Active:      u.Active,
		TOTPEnabled: u.TOTPEnabled,
		CreatedAt:   u.CreatedAt,
		LastLogin:   u.LastLogin,
	}
}

func userID(r *http.Request) string {
	if v, ok := ctx.UserIDFromCtx(r.Context()); ok {
		return v
	}
	return ""
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}

	var siteID *string
	if req.SiteID != nil && *req.SiteID != "" {
		siteID = req.SiteID
	}

	u, err := h.svc.Register(req.Username, req.Email, req.Password, Role(req.Role), siteID)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, map[string]any{"user": toUserResp(u)})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if h.loginLimiter != nil && !h.loginLimiter.Allow(strings.ToLower(req.Username)) {
		w.Header().Set("Retry-After", "60")
		httpserver.Error(w, apperr.New(apperr.ErrTooManyRequests, "demasiados intentos para este usuario", http.StatusTooManyRequests, nil))
		return
	}

	u, accessTok, refreshTok, err := h.svc.LoginTOTP(req.Username, req.Password, req.TOTPCode)
	if err != nil {
		slog.Warn("auth: login fallido", "username", req.Username, "error", err.Error())
		if h.auditor != nil {
			_ = h.auditor.Record(r.Context(), "auth.login_failed", "user", "", nil, map[string]any{
				"username": req.Username,
			})
		}
		httpserver.Error(w, err)
		return
	}
	if h.auditor != nil {
		_ = h.auditor.Record(r.Context(), "auth.login", "user", u.ID, nil, map[string]any{
			"username": u.Username,
			"role":     u.Role,
		})
	}
	_ = u
	httpserver.JSON(w, http.StatusOK, tokenResp{
		AccessToken:  accessTok,
		RefreshToken: refreshTok,
		TokenType:    "Bearer",
	})
}

// RefreshHandler rota el refresh token y devuelve un nuevo par de tokens.
func (h *Handler) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}

	accessTok, refreshTok, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, tokenResp{
		AccessToken:  accessTok,
		RefreshToken: refreshTok,
		TokenType:    "Bearer",
	})
}

// LogoutHandler revoca el refresh token entregado.
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	// El cuerpo es opcional: si viene vacío simplemente se responde OK.
	_ = httpserver.Decode(r, &req)
	if err := h.svc.Logout(req.RefreshToken); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// MeHandler devuelve el usuario autenticado.
func (h *Handler) MeHandler(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	if uid == "" {
		httpserver.Error(w, apperr.New(apperr.ErrUnauthorized, "no autenticado", http.StatusUnauthorized, nil))
		return
	}
	u, err := h.svc.Me(uid)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toUserResp(u))
}

// WSTicketHandler emite un token efímero para abrir el WebSocket sin exponer
// el access token de larga vida en la URL.
func (h *Handler) WSTicketHandler(w http.ResponseWriter, r *http.Request) {
	uid, _ := ctx.UserIDFromCtx(r.Context())
	username, _ := ctx.UsernameFromCtx(r.Context())
	role, _ := ctx.RoleFromCtx(r.Context())
	siteID, hasSite := ctx.SiteIDFromCtx(r.Context())
	if uid == "" || role == "" {
		httpserver.Error(w, apperr.New(apperr.ErrUnauthorized, "no autenticado", http.StatusUnauthorized, nil))
		return
	}
	var sitePtr *string
	if hasSite && siteID != "" {
		sitePtr = &siteID
	}
	token, expiresAt, err := h.svc.WSTicket(uid, username, role, sitePtr)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"ticket":     token,
		"expires_at": expiresAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

type totpCodeReq struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

func (h *Handler) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	secret, url, err := h.svc.SetupTOTP(uid)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"secret":       secret,
		"otpauth_url":  url,
		"totp_enabled": false,
	})
}

func (h *Handler) EnableTOTP(w http.ResponseWriter, r *http.Request) {
	var req totpCodeReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := h.svc.EnableTOTP(userID(r), req.Code); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{"totp_enabled": true})
}

func (h *Handler) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password" validate:"required,max=128"`
		Code     string `json:"code" validate:"required,len=6,numeric"`
	}
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := h.svc.DisableTOTP(userID(r), req.Password, req.Code); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{"totp_enabled": false})
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.svc.ListSessions(userID(r))
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{"data": sessions})
}

func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeSession(userID(r), chi.URLParam(r, "id")); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.NoContent(w)
}

func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.LogoutAll(userID(r)); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]string{"status": "logged_out_all"})
}

type passwordResetRequestReq struct {
	Identifier string `json:"identifier" validate:"required,max=255"`
}

func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequestReq
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	token, err := h.svc.RequestPasswordReset(req.Identifier)
	if err != nil {
		httpserver.Error(w, err)
		return
	}
	resp := map[string]any{"status": "accepted"}
	if h.svc.IsDevelopment() && token != "" {
		resp["reset_token"] = token
	}
	if token != "" && !h.svc.IsDevelopment() {
		slog.Warn("auth: recuperación de contraseña generada sin proveedor de correo configurado")
	}
	httpserver.JSON(w, http.StatusAccepted, resp)
}

func (h *Handler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token" validate:"required,max=512"`
		NewPassword string `json:"new_password" validate:"required,min=12,max=128"`
	}
	if err := httpserver.Decode(r, &req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpserver.Error(w, err)
		return
	}
	if err := h.svc.ResetPassword(req.Token, req.NewPassword); err != nil {
		httpserver.Error(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
}
