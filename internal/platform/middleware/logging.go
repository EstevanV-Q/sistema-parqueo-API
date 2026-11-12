package middleware

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// RequestLogger registra método, ruta, estado, duración, IP y nombres de los
// parámetros de query. Nunca registra valores de query, Authorization ni
// Cookie, para evitar filtrar tickets de WebSocket o tokens.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		queryKeys := make([]string, 0, len(r.URL.Query()))
		for key := range r.URL.Query() {
			queryKeys = append(queryKeys, key)
		}
		sort.Strings(queryKeys)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"query_keys", strings.Join(queryKeys, ","),
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", chimiddleware.GetReqID(r.Context()),
			"ip", clientIP(r),
		)
	})
}
