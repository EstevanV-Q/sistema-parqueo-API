package middleware

import (
	"net/http"
	"sync"
	"time"

	"parking-backend/internal/platform/apperr"
	"parking-backend/internal/platform/httpserver"
)

type window struct {
	count int
	reset time.Time
}

type limiter struct {
	mu      sync.Mutex
	windows map[string]*window
	limit   int
	period  time.Duration
}

func newLimiter(limit int, period time.Duration) *limiter {
	l := &limiter{
		windows: make(map[string]*window),
		limit:   limit,
		period:  period,
	}
	go l.cleanup()
	return l
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w, ok := l.windows[key]
	if !ok || now.After(w.reset) {
		l.windows[key] = &window{count: 1, reset: now.Add(l.period)}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

func (l *limiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for k, w := range l.windows {
			if now.After(w.reset) {
				delete(l.windows, k)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimit limita por IP las peticiones a un handler (ventana fija).
func RateLimit(limit int, period time.Duration) func(http.Handler) http.Handler {
	l := newLimiter(limit, period)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !l.allow(ip) {
				w.Header().Set("Retry-After", "60")
				httpserver.Error(w, apperr.New(apperr.ErrTooManyRequests, "demasiadas solicitudes, intente más tarde", http.StatusTooManyRequests, nil))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
