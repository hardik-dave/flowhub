package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/user"
)

// TestCrossTenantAllUsers exercises the platform-admin-only cross-tenant
// user list (GET /dash/all-users). It is the documented exception to
// per-tenant scoping (AGENTS.md rule 6): a platform admin sees users
// from every tenant, a tenant admin is refused.
func TestCrossTenantAllUsers(t *testing.T) {
	e := newMAEnv(t)

	admin := config.PlatformAdmin{
		Username: fmt.Sprintf("xplat_%d", time.Now().UnixNano()),
		Password: "adminpass123",
		Mobile:   "+919876510010",
	}
	if _, err := user.Bootstrap(context.Background(), e.store, admin); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer registerCleanup(t, e.pool, admin.Username)()
	platformToken := loginDash(t, e, admin.Username, admin.Password)

	prefix := fmt.Sprintf("xall%d", time.Now().UnixNano())
	mkTenant := func(suffix string) (int64, int64) {
		t.Helper()
		username := prefix + "_" + suffix
		rec := e.postJSON(t, "/api/v1/dash/tenants", map[string]any{
			"slug": prefix + suffix, "name": "Cross " + suffix,
			"start_date": "2026-01-01", "products": []string{"flowos"},
			"admin": map[string]any{
				"username": username, "password": "tenantpass123",
				"first_name": "Cross", "mobile": "+91987651" + fmt.Sprintf("%04d", time.Now().UnixNano()%10000),
			},
		}, platformToken)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create tenant %s = %d %s", suffix, rec.Code, rec.Body)
		}
		var body struct {
			TenantID    int64 `json:"tenant_id"`
			AdminUserID int64 `json:"admin_user_id"`
		}
		decode(t, rec, &body)
		if body.TenantID == 0 || body.AdminUserID == 0 {
			t.Fatalf("create tenant %s returned no ids: %s", suffix, rec.Body)
		}
		t.Cleanup(func() { deleteTenant(t, e, body.TenantID) })
		return body.TenantID, body.AdminUserID
	}
	tenantA, userA := mkTenant("a")
	tenantB, userB := mkTenant("b")

	// 1. Platform admin sees users from both tenants in one call.
	rec := e.doJSON(t, http.MethodGet, "/api/v1/dash/all-users?q="+prefix, nil, platformToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("all-users = %d %s", rec.Code, rec.Body)
	}
	var list struct {
		Users []struct {
			ID         int64  `json:"id"`
			TenantID   int64  `json:"tenant_id"`
			TenantSlug string `json:"tenant_slug"`
			TenantName string `json:"tenant_name"`
		} `json:"users"`
		Total int `json:"total"`
	}
	decode(t, rec, &list)
	seen := map[int64]int64{}
	for _, u := range list.Users {
		if u.TenantSlug == "" || u.TenantName == "" {
			t.Errorf("row %d missing tenant fields: %s", u.ID, rec.Body)
		}
		seen[u.ID] = u.TenantID
	}
	if len(seen) < 2 || seen[userA] != tenantA || seen[userB] != tenantB {
		t.Fatalf("cross-tenant rows = %v, want both %d@%d and %d@%d (%s)",
			seen, userA, tenantA, userB, tenantB, rec.Body)
	}

	// 2. tenant_id filter pins one tenant.
	rec = e.doJSON(t, http.MethodGet,
		fmt.Sprintf("/api/v1/dash/all-users?q=%s&tenant_id=%d", prefix, tenantA), nil, platformToken, "")
	decode(t, rec, &list)
	for _, u := range list.Users {
		if u.TenantID != tenantA {
			t.Fatalf("tenant_id filter leaked tenant %d: %s", u.TenantID, rec.Body)
		}
	}
	if len(list.Users) != 1 || list.Users[0].ID != userA {
		t.Fatalf("tenant_id filter rows = %s", rec.Body)
	}

	// 3. A tenant admin cannot use the cross-tenant endpoint (rule 6).
	tenantToken := loginDash(t, e, prefix+"_a", "tenantpass123")
	if rec := e.doJSON(t, http.MethodGet, "/api/v1/dash/all-users", nil, tenantToken, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("tenant admin all-users = %d, want 403", rec.Code)
	}
}
