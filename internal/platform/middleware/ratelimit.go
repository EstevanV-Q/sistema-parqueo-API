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

// KeyLimiter expone un limitador por clave para usarlo después de autenticar
// (por ejemplo, por username o por device_id). Es en memoria y por proceso.
type KeyLimiter struct {
	l *limiter
}

func NewKeyLimiter(limit int, period time.Duration) *KeyLimiter {
	return &KeyLimiter{l: newLimiter(limit, period)}
}

func (k *KeyLimiter) Allow(key string) bool {
	if k == nil || k.l == nil {
		return true
	}
	return k.l.allow(key)
}

// KeyExtractor extrae la clave de rate limiting de una petición HTTP.
// Por defecto se usa la IP del cliente, pero se puede inyectar una función
// que use el user_id del JWT o la X-Device-Key para un control más fino.
type KeyExtractor func(r *http.Request) string

// defaultKeyExtractor usa la IP del cliente como clave.
func defaultKeyExtractor(r *http.Request) string {
	return clientIP(r)
}

// RateLimit limita peticiones usando una clave extraída del request.
// Si keyExtractor es nil, se usa la IP del cliente.
func RateLimit(limit int, period time.Duration, keyExtractor ...KeyExtractor) func(http.Handler) http.Handler {
	l := newLimiter(limit, period)
	extract := defaultKeyExtractor
	if len(keyExtractor) > 0 && keyExtractor[0] != nil {
		extract = keyExtractor[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := extract(r)
			if !l.allow(key) {
				w.Header().Set("Retry-After", "60")
				httpserver.Error(w, apperr.New(apperr.ErrTooManyRequests, "demasiadas solicitudes, intente más tarde", http.StatusTooManyRequests, nil))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
