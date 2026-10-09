// Package httpx is shared HTTP plumbing: the §6 JSON envelopes,
// structured request logging, CORS, and the §9 rate limit. No
// business logic here.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// DecodeJSON reads a single JSON object from the request body. It
// writes the §6 error envelope and returns false on malformed input,
// so handlers can `if !DecodeJSON(...) { return }`.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		Error(w, http.StatusBadRequest, "Could not read request body.")
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		Error(w, http.StatusBadRequest, "Malformed request body: "+friendlyJSONErr(err))
		return false
	}
	return true
}

func friendlyJSONErr(err error) string {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Sprintf("invalid JSON at byte %d", syntax.Offset)
	}
	var typ *json.UnmarshalTypeError
	if errors.As(err, &typ) {
		return fmt.Sprintf("field %q has the wrong type", typ.Field)
	}
	return err.Error()
}
