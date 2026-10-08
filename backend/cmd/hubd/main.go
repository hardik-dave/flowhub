// hubd — FlowOS Hub server skeleton.
//
// STATUS: scaffold. Every route below returns 501 with a pointer to
// its SPEC.md section. The builder (contractor or agent) replaces
// this stdlib mux with chi and implements per milestone M-A → M-D.
// The route list IS the API surface — do not add routes not in SPEC.md.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func notImplemented(section string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "not implemented yet — see SPEC.md " + section,
		})
	}
}

func main() {
	mux := http.NewServeMux()

	// App-facing (consumed by FlowOS desktop) — SPEC.md §6
	mux.HandleFunc("POST /api/v1/app/verify", notImplemented("§6.1 (THE GOLDEN CONTRACT)"))
	mux.HandleFunc("POST /api/v1/app/login", notImplemented("§6.2"))
	mux.HandleFunc("POST /api/v1/app/register", notImplemented("§6.3"))
	mux.HandleFunc("POST /api/v1/app/otp/request", notImplemented("§6.4"))
	mux.HandleFunc("POST /api/v1/app/otp/verify", notImplemented("§6.4"))

	// Dashboard-facing — SPEC.md §7
	mux.HandleFunc("POST /api/v1/dash/login", notImplemented("§7"))
	mux.HandleFunc("POST /api/v1/dash/logout", notImplemented("§7"))
	mux.HandleFunc("GET /api/v1/dash/users", notImplemented("§7"))
	mux.HandleFunc("POST /api/v1/dash/users", notImplemented("§7"))
	mux.HandleFunc("GET /api/v1/dash/users/{id}", notImplemented("§7"))
	mux.HandleFunc("PATCH /api/v1/dash/users/{id}/status", notImplemented("§7"))
	mux.HandleFunc("POST /api/v1/dash/users/{id}/licenses", notImplemented("§7"))
	mux.HandleFunc("PATCH /api/v1/dash/licenses/{id}/status", notImplemented("§7"))
	mux.HandleFunc("POST /api/v1/dash/licenses/{id}/regenerate-key", notImplemented("§7"))
	mux.HandleFunc("POST /api/v1/dash/payments", notImplemented("§7"))
	mux.HandleFunc("POST /api/v1/dash/imports", notImplemented("§8"))
	mux.HandleFunc("GET /api/v1/dash/audit", notImplemented("§7"))
	mux.HandleFunc("GET /api/v1/dash/tenant", notImplemented("§7"))
	mux.HandleFunc("PATCH /api/v1/dash/tenant", notImplemented("§7"))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	addr := os.Getenv("HUB_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:8790"
	}
	log.Printf("hubd scaffold listening on %s (all business routes return 501 until built)", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
