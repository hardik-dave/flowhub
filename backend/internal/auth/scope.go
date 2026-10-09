package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/store"
)

// Scope is the §7 dashboard auth context, modelled on the PashuTrack
// AuthContext{UserID, TenantID, Role}: tenant and role are resolved
// from membership (LIMIT 1) and every dashboard query is scoped by it.
// Platform admins (house-tenant SUPERADMIN) may override TenantID via
// the X-Tenant-ID header — the one documented cross-tenant path (SPEC §7).
type Scope struct {
	UserID          int64
	TenantID        int64
	Role            store.Role
	IsPlatformAdmin bool
	TokenHash       string
}

const scopeKey ctxKey = 1

func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey, s)
}

func ScopeFrom(ctx context.Context) (Scope, bool) {
	s, ok := ctx.Value(scopeKey).(Scope)
	return s, ok
}

// RequireDashboard authenticates a DASHBOARD session, resolves the
// actor's tenant/role, and injects both the Principal (for logout) and
// the Scope. Non-dashboard roles and inactive accounts are rejected.
func RequireDashboard(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			user, hash, ok := authenticate(w, r, st, AudienceDashboard)
			if !ok {
				return
			}
			if user.Status != "ACTIVE" {
				httpx.Error(w, http.StatusForbidden, "This account is not active.")
				return
			}
			mem, err := st.FirstMembership(ctx, user.ID)
			if err != nil || !mem.Role.DashboardRole() {
				httpx.Error(w, http.StatusForbidden, "This account does not have dashboard access.")
				return
			}
			tenant, err := st.TenantByID(ctx, mem.TenantID)
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "Could not resolve the workspace.")
				return
			}
			sc := Scope{
				UserID:          user.ID,
				TenantID:        mem.TenantID,
				Role:            mem.Role,
				IsPlatformAdmin: mem.Role == store.RoleSuperadmin && tenant.IsHouse,
				TokenHash:       hash,
			}
			if hdr := strings.TrimSpace(r.Header.Get("X-Tenant-ID")); hdr != "" {
				tid, perr := strconv.ParseInt(hdr, 10, 64)
				if perr != nil || tid <= 0 {
					httpx.Error(w, http.StatusBadRequest, "X-Tenant-ID must be a positive integer tenant id.")
					return
				}
				if !sc.IsPlatformAdmin {
					httpx.Error(w, http.StatusForbidden, "Cross-tenant access is restricted to platform admins.")
					return
				}
				if tid != sc.TenantID {
					if _, terr := st.TenantByID(ctx, tid); terr != nil {
						httpx.Error(w, http.StatusNotFound, "Unknown tenant.")
						return
					}
					sc.TenantID = tid
				}
			}
			ctx = WithPrincipal(ctx, Principal{UserID: user.ID, Audience: AudienceDashboard, TokenHash: hash})
			ctx = WithScope(ctx, sc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticate resolves a live bearer session of the given audience to
// its user, writing the (deliberately uninformative) 401 on failure.
func authenticate(w http.ResponseWriter, r *http.Request, st *store.Store, audience string) (*store.User, string, bool) {
	token, ok := bearerToken(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Authentication required.")
		return nil, "", false
	}
	hash := HashToken(token)
	sess, err := st.SessionByTokenHash(r.Context(), hash)
	if err != nil || sess.Audience != audience {
		httpx.Error(w, http.StatusUnauthorized, "Authentication required.")
		return nil, "", false
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		httpx.Error(w, http.StatusUnauthorized, "Session expired. Sign in again.")
		return nil, "", false
	}
	user, err := st.UserByID(r.Context(), sess.UserID)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Authentication required.")
		return nil, "", false
	}
	return user, hash, true
}
