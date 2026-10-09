package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/store"
)

// Principal is the authenticated caller. Sessions are not tenant-bound
// (SPEC §3), so tenant scope is resolved per request from membership.
type Principal struct {
	UserID    int64
	Audience  string
	TokenHash string
}

type ctxKey int

const principalKey ctxKey = iota

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// Require rejects requests without a live bearer token of the given
// audience (DASHBOARD or APP). Failures are deliberately uninformative.
func Require(st *store.Store, audience string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "Authentication required.")
				return
			}
			hash := HashToken(token)
			sess, err := st.SessionByTokenHash(r.Context(), hash)
			if err != nil || sess.Audience != audience {
				httpx.Error(w, http.StatusUnauthorized, "Authentication required.")
				return
			}
			if time.Now().UTC().After(sess.ExpiresAt) {
				httpx.Error(w, http.StatusUnauthorized, "Session expired. Sign in again.")
				return
			}
			p := Principal{UserID: sess.UserID, Audience: sess.Audience, TokenHash: hash}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	return token, token != ""
}
