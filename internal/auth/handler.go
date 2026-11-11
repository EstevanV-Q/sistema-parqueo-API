package auth

import (
	"net/http"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/security/ctx"
	"parking-backend/internal/platform/validate"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type registerReq struct {
	Username string  `json:"username" validate:"required,min=3,max=100"`
	Email    string  `json:"email" validate:"required,email,max=255"`
	Password string  `json:"password" validate:"required,min=8,max=128"`
	Role     string  `json:"role" validate:"required,oneof=admin operator auditor"`
	SiteID   *string `json:"site_id,omitempty"`
}

type loginReq struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

// userResp es el DTO público de usuario. Nunca debe incluir PasswordHash.
type userResp struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	Role      Role    `json:"role"`
	SiteID    *string `json:"site_id,omitempty"`
	Active    bool    `json:"active"`
	CreatedAt string  `json:"created_at,omitempty"`
	LastLogin *string `json:"last_login,omitempty"`
}

func toUserResp(u *User) userResp {
	if u == nil {
		return userResp{}
	}
	return userResp{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		SiteID:    u.SiteID,
		Active:    u.Active,
		CreatedAt: u.CreatedAt,
		LastLogin: u.LastLogin,
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

	u, accessTok, refreshTok, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		httpserver.Error(w, err)
		return
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
