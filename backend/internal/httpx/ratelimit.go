package httpx

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a fixed-window per-key counter — SPEC.md §9's
// "10/min per IP on login & verify". now is injectable for tests.
type RateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	now       func() time.Time
	hits      map[string]*rlWindow
	nextSweep time.Time
}

type rlWindow struct {
	count int
	reset time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		now:    time.Now,
		hits:   map[string]*rlWindow{},
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	if now.After(rl.nextSweep) {
		for k, w := range rl.hits {
			if !now.Before(w.reset) {
				delete(rl.hits, k)
			}
		}
		rl.nextSweep = now.Add(rl.window)
	}
	w := rl.hits[key]
	if w == nil || !now.Before(w.reset) {
		rl.hits[key] = &rlWindow{count: 1, reset: now.Add(rl.window)}
		return true
	}
	w.count++
	return w.count <= rl.limit
}

// RateLimit 429s once a key passes n events per window.
func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(ClientIP(r)) {
				Error(w, http.StatusTooManyRequests, "Too many requests. Try again in a minute.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP prefers X-Forwarded-For (hubd sits behind Caddy — SPEC §9
// TLS), then X-Real-IP, then the socket address.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, _ := strings.Cut(xff, ","); strings.TrimSpace(first) != "" {
			return strings.TrimSpace(first)
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return xr
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
