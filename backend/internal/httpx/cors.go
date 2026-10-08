package httpx

import "net/http"

// CORS locks the browser to the dashboard SPA origin (SPEC.md §9).
// Anything else gets no CORS headers, so the browser refuses it.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := origin != "" && origin == allowedOrigin
			w.Header().Add("Vary", "Origin")
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			}
			if r.Method == http.MethodOptions {
				if allowed {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Max-Age", "600")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
