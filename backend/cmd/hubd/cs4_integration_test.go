package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/user"
)

func (e *maEnv) doJSON(t *testing.T, method, path string, body any, bearer, tenantHdr string) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if tenantHdr != "" {
		req.Header.Set("X-Tenant-ID", tenantHdr)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// deleteTenant removes every row a test tenant owns, children first.
func deleteTenant(t *testing.T, e *maEnv, tenantID int64) {
	t.Helper()
	rows, err := e.pool.Query("SELECT user_id FROM tenant_users WHERE tenant_id = ?", tenantID)
	if err != nil {
		t.Fatalf("list tenant users: %v", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	exec := func(q string, args ...any) {
		if _, err := e.pool.Exec(q, args...); err != nil {
			t.Errorf("cleanup %q: %v", q, err)
		}
	}
	for _, id := range ids {
		exec("DELETE FROM sessions WHERE user_id = ?", id)
		exec("DELETE FROM otp_codes WHERE user_id = ?", id)
	}
	// payments reference licenses and users, so they must be removed before
	// licenses (the other order silently left orphan tenants behind).
	exec("DELETE FROM payments WHERE tenant_id = ?", tenantID)
	exec("DELETE FROM licenses WHERE tenant_id = ?", tenantID)
	exec("DELETE FROM audit_log WHERE tenant_id = ?", tenantID)
	exec("DELETE FROM import_batches WHERE tenant_id = ?", tenantID)
	exec("DELETE FROM tenant_users WHERE tenant_id = ?", tenantID)
	exec("DELETE FROM tenant_products WHERE tenant_id = ?", tenantID)
	for _, id := range ids {
		exec("DELETE FROM users WHERE id = ?", id)
	}
	exec("DELETE FROM tenants WHERE id = ?", tenantID)
}

func loginDash(t *testing.T, e *maEnv, username, password string) string {
	t.Helper()
	rec := e.postJSON(t, "/api/v1/dash/login", map[string]any{"username": username, "password": password}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("dash login %s = %d %s", username, rec.Code, rec.Body)
	}
	var dl struct {
		SessionToken string `json:"session_token"`
	}
	decode(t, rec, &dl)
	if dl.SessionToken == "" {
		t.Fatalf("dash login %s returned no token", username)
	}
	return dl.SessionToken
}

// TestDashboardTenantLifecycle is the CS-4 acceptance test: platform
// admin onboards a tenant + admin, the tenant admin runs the user
// lifecycle (create → inspect → status → license grant/regenerate/
// disable), and every mutation is asserted to have written its audit
// row (AGENTS.md rules 6, 7).
func TestDashboardTenantLifecycle(t *testing.T) {
	e := newMAEnv(t)

	platform := config.PlatformAdmin{
		Username: fmt.Sprintf("cs4adm_%d", time.Now().UnixNano()),
		Password: "platformpass123",
		Mobile:   "+919876511000",
	}
	if _, err := user.Bootstrap(context.Background(), e.store, platform); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer registerCleanup(t, e.pool, platform.Username)()
	platformToken := loginDash(t, e, platform.Username, platform.Password)

	// The bootstrapped platform admin owns zero licenses. The users list
	// must serialize the field as [] (not null), or the dashboard client
	// crashes while rendering the default route.
	prec := e.doJSON(t, http.MethodGet, "/api/v1/dash/users?q="+platform.Username, nil, platformToken, "")
	if prec.Code != http.StatusOK {
		t.Fatalf("list house users = %d %s", prec.Code, prec.Body)
	}
	if !bytes.Contains(prec.Body.Bytes(), []byte(`"licenses":[]`)) {
		t.Fatalf("license-less user must serialize licenses as [], got %s", prec.Body)
	}

	// 1. Onboard a tenant with its admin.
	slug := fmt.Sprintf("t%d", time.Now().UnixNano())
	adminUsername := fmt.Sprintf("tsadm_%d", time.Now().UnixNano())
	rec := e.doJSON(t, http.MethodPost, "/api/v1/dash/tenants", map[string]any{
		"slug": slug, "name": "Test Tenant", "contact_person": "Owner", "contact_no": "+919876511001",
		"start_date": "2026-01-01", "grace_working_days": 7, "products": []string{"flowos", "optionalyzer"},
		"admin": map[string]any{
			"username": adminUsername, "password": "tenantpass123", "first_name": "T",
			"last_name": "Admin", "mobile": "+919876511002",
		},
	}, platformToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tenant = %d %s", rec.Code, rec.Body)
	}
	var ct struct {
		TenantID      int64  `json:"tenant_id"`
		AdminUserID   int64  `json:"admin_user_id"`
		AdminUsername string `json:"admin_username"`
	}
	decode(t, rec, &ct)
	if ct.TenantID == 0 || ct.AdminUserID == 0 {
		t.Fatalf("create tenant body = %s", rec.Body)
	}
	defer deleteTenant(t, e, ct.TenantID)
	assertAudit(t, e.pool, ct.AdminUserID, "TENANT_CREATED")

	// unknown product → 400; duplicate slug → 409.
	rec = e.doJSON(t, http.MethodPost, "/api/v1/dash/tenants", map[string]any{
		"slug": slug + "x", "name": "Bad", "start_date": "2026-01-01", "products": []string{"nope"},
		"admin": map[string]any{"username": adminUsername + "x", "password": "tenantpass123", "mobile": "+919876511003"},
	}, platformToken, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown-product tenant = %d %s, want 400", rec.Code, rec.Body)
	}

	// 2. Platform-admin tenant list includes it.
	rec = e.doJSON(t, http.MethodGet, "/api/v1/dash/tenants", nil, platformToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list tenants = %d %s", rec.Code, rec.Body)
	}
	var lt struct {
		Tenants []struct {
			ID   int64  `json:"id"`
			Slug string `json:"slug"`
		} `json:"tenants"`
	}
	decode(t, rec, &lt)
	found := false
	for _, tn := range lt.Tenants {
		if tn.ID == ct.TenantID && tn.Slug == slug {
			found = true
		}
	}
	if !found {
		t.Fatalf("new tenant %d missing from list: %s", ct.TenantID, rec.Body)
	}

	// 3. Tenant admin logs in and creates a user; secrets returned once.
	tenantToken := loginDash(t, e, adminUsername, "tenantpass123")
	algoUser := fmt.Sprintf("cs4u_%d", time.Now().UnixNano())
	rec = e.doJSON(t, http.MethodPost, "/api/v1/dash/users", map[string]any{
		"username": algoUser, "first_name": "A", "last_name": "U", "mobile": "+919876511004",
		"product_code": "flowos",
	}, tenantToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user = %d %s", rec.Code, rec.Body)
	}
	var cu struct {
		UserID     int64  `json:"user_id"`
		Password   string `json:"password"`
		LicenseKey string `json:"license_key"`
	}
	decode(t, rec, &cu)
	if cu.UserID == 0 || cu.Password == "" || cu.LicenseKey == "" {
		t.Fatalf("create user body = %s", rec.Body)
	}
	assertAudit(t, e.pool, cu.UserID, "USER_CREATED")

	// 4. User detail shows the license with a key hint.
	rec = e.doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/dash/users/%d", cu.UserID), nil, tenantToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get user = %d %s", rec.Code, rec.Body)
	}
	var detail struct {
		User struct {
			Status   string `json:"status"`
			Role     string `json:"role"`
			Licenses []struct {
				ID      int64  `json:"id"`
				Status  string `json:"status"`
				KeyHint string `json:"key_hint"`
			} `json:"licenses"`
		} `json:"user"`
	}
	decode(t, rec, &detail)
	if len(detail.User.Licenses) != 1 || detail.User.Role != "ALGO_USER" {
		t.Fatalf("get user body = %s", rec.Body)
	}
	licenseID := detail.User.Licenses[0].ID

	// 5. Status change requires a reason; valid one bumps token_version.
	rec = e.doJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/dash/users/%d/status", cu.UserID),
		map[string]any{"status": "DEACTIVATED", "reason": ""}, tenantToken, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty-reason status = %d %s, want 400", rec.Code, rec.Body)
	}
	rec = e.doJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/dash/users/%d/status", cu.UserID),
		map[string]any{"status": "DEACTIVATED", "reason": "Non-payment"}, tenantToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate = %d %s", rec.Code, rec.Body)
	}
	assertAudit(t, e.pool, cu.UserID, "USER_STATUS_CHANGED")
	var tv int
	if err := e.pool.QueryRow("SELECT token_version FROM users WHERE id = ?", cu.UserID).Scan(&tv); err != nil {
		t.Fatal(err)
	}
	if tv == 0 {
		t.Fatalf("token_version = %d after deactivation, want > 0", tv)
	}

	// 6. Duplicate license grant → 409; regenerate returns a new key.
	rec = e.doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/dash/users/%d/licenses", cu.UserID),
		map[string]any{"product_code": "flowos"}, tenantToken, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate grant = %d %s, want 409", rec.Code, rec.Body)
	}
	rec = e.doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/dash/licenses/%d/regenerate-key", licenseID), nil, tenantToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("regenerate = %d %s", rec.Code, rec.Body)
	}
	var rk struct {
		LicenseKey string `json:"license_key"`
	}
	decode(t, rec, &rk)
	if rk.LicenseKey == "" || rk.LicenseKey == cu.LicenseKey {
		t.Fatalf("regenerated key = %q (old %q)", rk.LicenseKey, cu.LicenseKey)
	}
	assertAudit(t, e.pool, cu.UserID, "LICENSE_REGENERATED")

	// 7. Disable the license; reason required.
	rec = e.doJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/dash/licenses/%d/status", licenseID),
		map[string]any{"status": "DISABLED", "reason": "Refund"}, tenantToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("disable license = %d %s", rec.Code, rec.Body)
	}
	assertAudit(t, e.pool, cu.UserID, "LICENSE_STATUS_CHANGED")

	// 8. Tenant self-service + audit trail.
	rec = e.doJSON(t, http.MethodGet, "/api/v1/dash/tenant", nil, tenantToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get tenant = %d %s", rec.Code, rec.Body)
	}
	var tvw struct {
		Slug             string   `json:"slug"`
		GraceWorkingDays int      `json:"grace_working_days"`
		Products         []string `json:"products"`
	}
	decode(t, rec, &tvw)
	if tvw.Slug != slug || tvw.GraceWorkingDays != 7 {
		t.Fatalf("tenant view = %s", rec.Body)
	}
	if len(tvw.Products) != 2 || tvw.Products[0] != "flowos" || tvw.Products[1] != "optionalyzer" {
		t.Fatalf("tenant products = %v, want [flowos optionalyzer]", tvw.Products)
	}
	rec = e.doJSON(t, http.MethodPatch, "/api/v1/dash/tenant",
		map[string]any{"grace_working_days": 3}, tenantToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("patch tenant = %d %s", rec.Code, rec.Body)
	}
	rec = e.doJSON(t, http.MethodGet, "/api/v1/dash/audit", nil, tenantToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit = %d %s", rec.Code, rec.Body)
	}

	// 9. Cross-tenant confinement (rule 6).
	// tenant admin cannot list tenants
	if rec := e.doJSON(t, http.MethodGet, "/api/v1/dash/tenants", nil, tenantToken, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("non-platform list tenants = %d, want 403", rec.Code)
	}
	// non-platform admin cannot use X-Tenant-ID
	if rec := e.doJSON(t, http.MethodGet, "/api/v1/dash/tenant", nil, tenantToken, "1"); rec.Code != http.StatusForbidden {
		t.Fatalf("non-platform X-Tenant-ID = %d, want 403", rec.Code)
	}
	// platform admin acting as the tenant sees its user
	rec = e.doJSON(t, http.MethodGet, "/api/v1/dash/users?q="+algoUser, nil, platformToken, fmt.Sprintf("%d", ct.TenantID))
	if rec.Code != http.StatusOK {
		t.Fatalf("platform act-as list = %d %s", rec.Code, rec.Body)
	}
	var pl struct {
		Users []struct {
			ID int64 `json:"id"`
		} `json:"users"`
	}
	decode(t, rec, &pl)
	if len(pl.Users) != 1 || pl.Users[0].ID != cu.UserID {
		t.Fatalf("platform act-as users = %s", rec.Body)
	}
	// platform admin without the header stays in its own tenant
	rec = e.doJSON(t, http.MethodGet, "/api/v1/dash/users?q="+algoUser, nil, platformToken, "")
	decode(t, rec, &pl)
	if len(pl.Users) != 0 {
		t.Fatalf("platform default tenant leaked user: %s", rec.Body)
	}

	// 10. Deactivate the tenant.
	rec = e.doJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/dash/tenants/%d/status", ct.TenantID),
		map[string]any{"status": "INACTIVE", "reason": "Trial ended"}, platformToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("tenant status = %d %s", rec.Code, rec.Body)
	}
}
