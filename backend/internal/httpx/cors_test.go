package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsOnlyConfiguredOrigin(t *testing.T) {
	h := CORS("http://localhost:5173")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dash/users", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("ACAO = %q, want the configured origin", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/dash/users", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO = %q, want empty for a foreign origin", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	h := CORS("http://localhost:5173")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/app/verify", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("preflight must allow Authorization, Content-Type")
	}
}
