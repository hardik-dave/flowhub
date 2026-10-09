package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/flowos/hub/internal/auth"
	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/dashboard"
	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/user"
	"github.com/flowos/hub/internal/verify"
)

// testHandlers wires handlers over a nil store — enough to register
// routes; the surface tests never invoke them with a nil store.
func testHandlers() *handlers {
	ah := auth.NewHandler(nil, auth.NewSender(nil), time.Now)
	vs := verify.NewService(nil, time.Now)
	return &handlers{
		verify:   verify.NewHandler(vs),
		auth:     ah,
		user:     user.New(nil, ah.OTP(), vs, time.Now),
		dash:     dashboard.New(nil, time.Now),
		dashAuth: func(next http.Handler) http.Handler { return next },
	}
}

// TestRouteSurface locks the API surface: every SPEC §6/§7 route
// (22 business endpoints + healthz), nothing more (AGENTS.md rule 3).
func TestRouteSurface(t *testing.T) {
	cfg := &config.Config{DashboardOrigin: "http://localhost:5173"}
	r := newRouter(cfg, httpx.NewRateLimiter(10, time.Minute), testHandlers())

	want := []string{
		"GET /healthz",
		// §6 app-facing (5)
		"POST /api/v1/app/verify",
		"POST /api/v1/app/login",
		"POST /api/v1/app/register",
		"POST /api/v1/app/otp/request",
		"POST /api/v1/app/otp/verify",
		// §7 dashboard-facing (16)
		"POST /api/v1/dash/login",
		"POST /api/v1/dash/logout",
		"GET /api/v1/dash/users",
		"POST /api/v1/dash/users",
		"GET /api/v1/dash/users/{id}",
		"PATCH /api/v1/dash/users/{id}/status",
		"POST /api/v1/dash/users/{id}/licenses",
		"PATCH /api/v1/dash/licenses/{id}/status",
		"POST /api/v1/dash/licenses/{id}/regenerate-key",
		"POST /api/v1/dash/payments",
		"POST /api/v1/dash/imports",
		"GET /api/v1/dash/audit",
		"GET /api/v1/dash/tenant",
		"PATCH /api/v1/dash/tenant",
		"POST /api/v1/dash/tenants",
		"PATCH /api/v1/dash/tenants/{id}/status",
		// DECISIONS.md: platform-admin tenant list (1)
		"GET /api/v1/dash/tenants",
	}

	wantSet := make(map[string]bool, len(want))
	for _, w := range want {
		wantSet[w] = true
	}
	got := map[string]bool{}
	collectRoutes(r, "", got)
	for _, w := range want {
		if !got[w] {
			t.Errorf("route missing: %s", w)
		}
	}
	for g := range got {
		if !wantSet[g] {
			t.Errorf("unexpected route (not in SPEC §6/§7): %s", g)
		}
	}
}

// collectRoutes flattens chi's mount tree: subrouter patterns are
// relative ("/app/*" mounts "/verify" as "/api/v1/app/verify").
func collectRoutes(routes chi.Routes, prefix string, out map[string]bool) {
	for _, rt := range routes.Routes() {
		if rt.SubRoutes != nil {
			collectRoutes(rt.SubRoutes, prefix+strings.TrimSuffix(rt.Pattern, "/*"), out)
			continue
		}
		for method := range rt.Handlers {
			out[method+" "+prefix+rt.Pattern] = true
		}
	}
}

func TestHealthz(t *testing.T) {
	cfg := &config.Config{DashboardOrigin: "http://localhost:5173"}
	r := newRouter(cfg, httpx.NewRateLimiter(10, time.Minute), testHandlers())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("GET /healthz = %d %q, want 200 \"ok\"", rec.Code, rec.Body.String())
	}
}
