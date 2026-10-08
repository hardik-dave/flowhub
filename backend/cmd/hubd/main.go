// hubd — FlowOS Hub server.
//
// Boot order: config → open pool → apply migrations → router → listen.
// A route that is not yet implemented answers 501 with its SPEC.md
// section. The route list below IS the API surface — do not add
// routes outside SPEC.md §6/§7 (+ GET /dash/tenants, DECISIONS.md).
package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/db"
	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/migrate"
)

func notImplemented(section string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, http.StatusNotImplemented, "not implemented yet — see SPEC.md "+section)
	}
}

// newRouter builds the full API surface; DB-free so tests can lock
// the route list (route surface IS the contract).
func newRouter(cfg *config.Config, rl *httpx.RateLimiter) *chi.Mux {
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
			r.With(httpx.RateLimit(rl)).Post("/verify", notImplemented("§6.1 (THE GOLDEN CONTRACT)"))
			r.With(httpx.RateLimit(rl)).Post("/login", notImplemented("§6.2"))
			r.Post("/register", notImplemented("§6.3"))
			r.Post("/otp/request", notImplemented("§6.4"))
			r.Post("/otp/verify", notImplemented("§6.4"))
		})

		// Dashboard-facing — SPEC.md §7.
		r.Route("/dash", func(r chi.Router) {
			r.With(httpx.RateLimit(rl)).Post("/login", notImplemented("§7"))
			r.Post("/logout", notImplemented("§7"))

			r.Get("/users", notImplemented("§7"))
			r.Post("/users", notImplemented("§7"))
			r.Get("/users/{id}", notImplemented("§7"))
			r.Patch("/users/{id}/status", notImplemented("§7"))
			r.Post("/users/{id}/licenses", notImplemented("§7"))

			r.Patch("/licenses/{id}/status", notImplemented("§7"))
			r.Post("/licenses/{id}/regenerate-key", notImplemented("§7"))

			r.Post("/payments", notImplemented("§7"))
			r.Post("/imports", notImplemented("§8"))
			r.Get("/audit", notImplemented("§7"))
			r.Get("/tenant", notImplemented("§7"))
			r.Patch("/tenant", notImplemented("§7"))

			// Platform-admin (house-tenant SUPERADMIN) — SPEC.md §7;
			// GET list added by DECISIONS.md for the tenant page.
			r.Get("/tenants", notImplemented("§7"))
			r.Post("/tenants", notImplemented("§7"))
			r.Patch("/tenants/{id}/status", notImplemented("§7"))
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

	rl := httpx.NewRateLimiter(10, time.Minute)
	slog.Info("hubd listening", "addr", cfg.Listen, "cors_origin", cfg.DashboardOrigin)
	if err := http.ListenAndServe(cfg.Listen, newRouter(cfg, rl)); err != nil {
		log.Fatal(err)
	}
}
