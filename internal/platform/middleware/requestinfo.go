package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

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

// RequestIDHeader expone el request ID de chi para correlacionar logs y
// respuestas de error.
func RequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := chimiddleware.GetReqID(r.Context()); id != "" {
			w.Header().Set("X-Request-ID", id)
		}
		next.ServeHTTP(w, r)
	})
}

// trustProxyHeaders indica si se deben confiar en X-Real-IP / X-Forwarded-For.
// Solo debe activarse cuando la API está detrás de un proxy inverso controlado.
var trustProxyHeaders atomic.Bool

// ConfigureTrustProxyHeaders habilita o deshabilita la confianza en cabeceras
// de proxy. Debe llamarse al construir la aplicación, no en init.
func ConfigureTrustProxyHeaders(enabled bool) {
	trustProxyHeaders.Store(enabled)
}

func clientIP(r *http.Request) string {
	if trustProxyHeaders.Load() {
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			// Tomar el último componente añadido por el proxy inverso más cercano para evitar spoofing
			last := strings.TrimSpace(parts[len(parts)-1])
			if last != "" {
				return last
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
