package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimiterWindow(t *testing.T) {
	clk := time.Date(2026, time.September, 14, 10, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(3, time.Minute)
	rl.now = func() time.Time { return clk }

	for i := 1; i <= 3; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("request %d/3 must be allowed", i)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("4th request in the same window must be blocked")
	}
	if !rl.Allow("5.6.7.8") {
		t.Fatal("a different key must have its own window")
	}
	clk = clk.Add(61 * time.Second)
	if !rl.Allow("1.2.3.4") {
		t.Fatal("after the window expires the key must pass again")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	h := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app/verify", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app/verify", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: status = %d, want 429", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"error"`) {
		t.Errorf("body = %s, want the §6 error envelope", rec.Body.String())
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := ClientIP(r); got != "203.0.113.9" {
		t.Errorf("ClientIP with XFF = %q, want the first hop", got)
	}

	r = httptest.NewRequest(http.MethodPost, "/login", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	if got := ClientIP(r); got != "10.0.0.5" {
		t.Errorf("ClientIP fallback = %q, want socket host", got)
	}
}
