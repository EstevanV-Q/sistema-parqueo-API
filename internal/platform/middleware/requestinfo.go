package middleware

import (
	"net"
	"net/http"
	"strings"

	ctxpkg "parking-backend/internal/platform/security/ctx"
)

// RequestInfo inyecta la IP y el User-Agent del cliente en el contexto para
// que la auditoría pueda registrarlos.
func RequestInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		ctx := ctxpkg.WithIP(r.Context(), ip)
		ctx = ctxpkg.WithUserAgent(ctx, r.UserAgent())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		// Tomar el último componente añadido por el proxy inverso más cercano para evitar spoofing
		last := strings.TrimSpace(parts[len(parts)-1])
		if last != "" {
			return last
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
