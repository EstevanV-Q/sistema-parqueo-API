package middleware

import (
	"net/http"
	"strings"

	"parking-backend/internal/config"
	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
	"parking-backend/internal/platform/security"
	ctxpkg "parking-backend/internal/platform/security/ctx"
)

// AuthMiddleware valida el JWT de acceso y inyecta user_id, username, role y
// site_id en el contexto.
type AuthMiddleware struct {
	cfg *config.Config
}

func NewAuthMiddleware(cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{cfg: cfg}
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httpserver.Error(w, apperr.New(apperr.ErrUnauthorized, "token de acceso requerido", http.StatusUnauthorized, nil))
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			httpserver.Error(w, apperr.New(apperr.ErrUnauthorized, "formato de token inválido", http.StatusUnauthorized, nil))
			return
		}

		claims, err := security.ParseAccessToken(m.cfg.JWTSecret, parts[1])
		if err != nil {
			httpserver.Error(w, apperr.New(apperr.ErrUnauthorized, "token inválido o expirado", http.StatusUnauthorized, err))
			return
		}

		reqCtx := r.Context()
		reqCtx = ctxpkg.WithUserID(reqCtx, claims.UserID)
		reqCtx = ctxpkg.WithUsername(reqCtx, claims.Username)
		reqCtx = ctxpkg.WithRole(reqCtx, claims.Role)
		if claims.SiteID != nil {
			reqCtx = ctxpkg.WithSiteID(reqCtx, *claims.SiteID)
		}

		next.ServeHTTP(w, r.WithContext(reqCtx))
	})
}

// RequireRole permite el paso solo si el usuario tiene uno de los roles dados.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := ctxpkg.RoleFromCtx(r.Context())
			if !ok {
				httpserver.Error(w, apperr.New(apperr.ErrForbidden, "rol no encontrado en contexto", http.StatusForbidden, nil))
				return
			}
			for _, a := range allowed {
				if role == a {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpserver.Error(w, apperr.New(apperr.ErrForbidden, "rol insuficiente", http.StatusForbidden, nil))
		})
	}
}

// allowedOrigins contiene los orígenes permitidos. En producción debe configurarse
// con los dominios reales del frontend. Por defecto (desarrollo) se permite localhost.
var allowedOrigins = []string{
	"http://localhost:3000",
	"http://localhost:8080",
	"https://localhost:3000",
}

// SetAllowedOrigins reemplaza la lista de orígenes permitidos. Útil para inyectar
// dominios de producción desde la configuración de la aplicación.
func SetAllowedOrigins(origins []string) {
	allowedOrigins = origins
}

// originPermitido verifica si el origen de la petición está en la lista permitida.
func originPermitido(origin string) bool {
	for _, o := range allowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

// CORS agrega encabezados de acceso cruzado restringidos.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Encabezados de seguridad para todas las respuestas
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if origin != "" && originPermitido(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key, X-Device-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
