package httpx

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

// RequestLog emits one structured line per request (SPEC.md §9).
// user/tenant IDs join the line once auth lands.
func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http",
			"req_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}
