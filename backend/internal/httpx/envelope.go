// Package httpx is shared HTTP plumbing: the §6 JSON envelopes,
// structured request logging, CORS, and the §9 rate limit. No
// business logic here.
package httpx

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as the response body with status (SPEC.md §6:
// snake_case JSON; errors always {"error": "..."}).
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes the single-field error envelope (SPEC.md §6).
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}
