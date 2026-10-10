// hubd — FlowHub server.
//
// Boot order: config → open pool → apply migrations → bootstrap the
// first-boot platform admin → router → listen. A route that is not yet
// implemented answers 501 with its SPEC.md section. The route list
// below IS the API surface — do not add routes outside SPEC.md
// §6/§7 (+ GET /dash/tenants, DECISIONS.md).
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/flowos/hub/internal/auth"
	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/dashboard"
	"github.com/flowos/hub/internal/db"
	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/migrate"
	"github.com/flowos/hub/internal/store"
	"github.com/flowos/hub/internal/user"
	"github.com/flowos/hub/internal/verify"
)

func notImplemented(section string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, http.StatusNotImplemented, "not implemented yet — see SPEC.md "+section)
	}
}

// handlers bundles the wired HTTP handlers so newRouter stays free of
// construction details (and tests can lock the surface with stubs).
type handlers struct {
	verify   *verify.Handler
	auth     *auth.Handler
	user     *user.Handler
	dash     *dashboard.Handler
	dashAuth func(http.Handler) http.Handler
}

func buildHandlers(cfg *config.Config, st *store.Store) *handlers {
	sender := auth.NewSender(cfg)
	// Echo the OTP only when explicitly enabled AND production SMS is
	// unconfigured (dev). Two independent guards so prod can never leak.
	devEchoOTP := cfg.DevEchoOTP && cfg.Msg91.AuthKey == "" && cfg.Msg91.DLTTemplateID == ""
	if devEchoOTP {
		slog.Info("dev OTP echo enabled — /app/otp/request returns dev_code and the 60s cooldown is bypassed")
	}
	ah := auth.NewHandler(st, sender, time.Now, devEchoOTP)
	vs := verify.NewService(st, time.Now)
	return &handlers{
		verify:   verify.NewHandler(vs),
		auth:     ah,
		user:     user.New(st, ah.OTP(), vs, time.Now),
		dash:     dashboard.New(st, time.Now),
		dashAuth: auth.RequireDashboard(st),
	}
}

// newRouter builds the full API surface. Handlers may be stubbed
// (nil stores) so tests lock the route list without a database.
func newRouter(cfg *config.Config, rl *httpx.RateLimiter, h *handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(httpx.RequestLog)
	r.Use(httpx.CORS(cfg.DashboardOrigin))
	r.Use(chimw.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Route("/api/v1", func(r chi.Router) {
		// App-facing — SPEC.md §6 (desktop client, §6.1 is law).
		r.Route("/app", func(r chi.Router) {
			// §9: 10/min per IP on login & verify.
			r.With(httpx.RateLimit(rl)).Post("/verify", h.verify.Verify)
			r.With(httpx.RateLimit(rl)).Post("/login", h.user.Login)
			r.Post("/register", h.user.Register)
			r.Post("/otp/request", h.auth.OTPRequest)
			r.Post("/otp/verify", h.auth.OTPVerify)
		})

		// Dashboard-facing — SPEC.md §7.
		r.Route("/dash", func(r chi.Router) {
			r.With(httpx.RateLimit(rl)).Post("/login", h.auth.DashLogin)
			r.With(h.dashAuth).Post("/logout", h.auth.DashLogout)

			r.With(h.dashAuth).Get("/users", h.dash.ListUsers)
			r.With(h.dashAuth).Post("/users", h.dash.CreateUser)
			r.With(h.dashAuth).Get("/users/{id}", h.dash.GetUser)
			r.With(h.dashAuth).Patch("/users/{id}/status", h.dash.UpdateUserStatus)
			r.With(h.dashAuth).Post("/users/{id}/licenses", h.dash.GrantLicense)

			r.With(h.dashAuth).Patch("/licenses/{id}/status", h.dash.UpdateLicenseStatus)
			r.With(h.dashAuth).Post("/licenses/{id}/regenerate-key", h.dash.RegenerateLicenseKey)
			r.With(h.dashAuth).Post("/licenses/{id}/validity", h.dash.SetLicenseValidity)

			// Payments land in a later change-set; stays 501 for now.
			r.With(h.dashAuth).Post("/payments", notImplemented("§7"))
			r.With(h.dashAuth).Post("/imports", h.dash.ImportCSV)
			r.With(h.dashAuth).Get("/audit", h.dash.ListAudit)
			r.With(h.dashAuth).Get("/tenant", h.dash.GetTenant)
			r.With(h.dashAuth).Patch("/tenant", h.dash.UpdateTenant)

			// Platform-admin (house-tenant SUPERADMIN) — SPEC.md §7;
			// GET list added by DECISIONS.md for the tenant page.
			r.With(h.dashAuth).Get("/tenants", h.dash.ListTenants)
			r.With(h.dashAuth).Post("/tenants", h.dash.CreateTenant)
			r.With(h.dashAuth).Patch("/tenants/{id}/status", h.dash.UpdateTenantStatus)
		})
	})
	return r
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	if err := migrate.Up(cfg.MigrationsDir, cfg.MySQLDSN); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	st := store.New(pool)
	created, err := user.Bootstrap(context.Background(), st, cfg.PlatformAdmin)
	if err != nil {
		log.Fatalf("bootstrap platform admin: %v", err)
	}
	if created {
		slog.Info("platform admin bootstrapped", "username", cfg.PlatformAdmin.Username)
	}

	rl := httpx.NewRateLimiter(10, time.Minute)
	slog.Info("hubd listening", "addr", cfg.Listen, "cors_origin", cfg.DashboardOrigin)
	if err := http.ListenAndServe(cfg.Listen, newRouter(cfg, rl, buildHandlers(cfg, st))); err != nil {
		log.Fatal(err)
	}
}
