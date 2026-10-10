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

// newSLVTenant creates a tenant + admin and returns the tenant's dash
// token. The tenant is registered for cleanup.
func newSLVTenant(t *testing.T, e *maEnv, platformToken, tag string) string {
	t.Helper()
	slug := fmt.Sprintf("slv%s%d", tag, time.Now().UnixNano())
	adminUsername := fmt.Sprintf("slvad_%s_%d", tag, time.Now().UnixNano())
	rec := e.doJSON(t, http.MethodPost, "/api/v1/dash/tenants", map[string]any{
		"slug": slug, "name": "SLV Tenant", "start_date": "2026-01-01",
		"products": []string{"optionalyzer"},
		"admin": map[string]any{
			"username": adminUsername, "password": "tenantpass123", "mobile": "+919876523001",
		},
	}, platformToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tenant = %d %s", rec.Code, rec.Body)
	}
	var ct struct {
		TenantID int64 `json:"tenant_id"`
	}
	decode(t, rec, &ct)
	t.Cleanup(func() { deleteTenant(t, e, ct.TenantID) })
	return loginDash(t, e, adminUsername, "tenantpass123")
}

// slvUserLicense creates a user without a subscription window and
// returns the new user's id and their license id.
func slvUserLicense(t *testing.T, e *maEnv, token, username, mobile string) (int64, int64) {
	t.Helper()
	rec := e.doJSON(t, http.MethodPost, "/api/v1/dash/users", map[string]any{
		"username": username, "mobile": mobile, "product_code": "optionalyzer",
	}, token, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user = %d %s", rec.Code, rec.Body)
	}
	var cu struct {
		UserID int64 `json:"user_id"`
	}
	decode(t, rec, &cu)

	rec = e.doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/dash/users/%d", cu.UserID), nil, token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get user = %d %s", rec.Code, rec.Body)
	}
	var gu struct {
		User struct {
			Licenses []struct {
				ID int64 `json:"id"`
			} `json:"licenses"`
		} `json:"user"`
	}
	decode(t, rec, &gu)
	if len(gu.User.Licenses) != 1 {
		t.Fatalf("licenses = %d, want 1", len(gu.User.Licenses))
	}
	return cu.UserID, gu.User.Licenses[0].ID
}

// TestSetLicenseValidity covers the dashboard "set access window"
// action on an existing license: it writes subscription_valid_until, a
// MANUAL payment row, and a LICENSE_VALIDITY_SET audit row in one
// transaction; validates all-or-nothing input; and refuses to touch a
// license belonging to another tenant (AGENTS.md rules 6, 7, 9).
func TestSetLicenseValidity(t *testing.T) {
	e := newMAEnv(t)

	platform := config.PlatformAdmin{
		Username: fmt.Sprintf("slvadm_%d", time.Now().UnixNano()),
		Password: "platformpass123",
		Mobile:   "+919876523000",
	}
	if _, err := user.Bootstrap(context.Background(), e.store, platform); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer registerCleanup(t, e.pool, platform.Username)()
	platformToken := loginDash(t, e, platform.Username, platform.Password)

	tenantToken := newSLVTenant(t, e, platformToken, "a")
	userID, licenseID := slvUserLicense(t, e, tenantToken,
		fmt.Sprintf("slvu_%d", time.Now().UnixNano()), "+919876523002")

	// Set a MONTHLY window on the existing license.
	rec := e.doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/dash/licenses/%d/validity", licenseID), map[string]any{
		"plan": "MONTHLY", "paid_at": "2026-10-10", "valid_from": "2026-10-10",
	}, tenantToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("set validity = %d %s", rec.Code, rec.Body)
	}
	var sv struct {
		LicenseID  int64  `json:"license_id"`
		ValidUntil string `json:"valid_until"`
	}
	decode(t, rec, &sv)
	if sv.LicenseID != licenseID || sv.ValidUntil != "2026-11-10" {
		t.Fatalf("set validity body = %s, want license %d until 2026-11-10", rec.Body, licenseID)
	}

	// The license row carries the window.
	var until time.Time
	if err := e.pool.QueryRow("SELECT subscription_valid_until FROM licenses WHERE id = ?", licenseID).Scan(&until); err != nil {
		t.Fatalf("license valid_until: %v", err)
	}
	if got := until.Format("2006-01-02"); got != "2026-11-10" {
		t.Fatalf("license subscription_valid_until = %s, want 2026-11-10", got)
	}

	// A MANUAL payment row is written.
	var n int
	var plan, method string
	if err := e.pool.QueryRow(
		"SELECT COUNT(*), COALESCE(MAX(plan),''), COALESCE(MAX(method),'') FROM payments WHERE license_id = ?",
		licenseID).Scan(&n, &plan, &method); err != nil {
		t.Fatalf("payments: %v", err)
	}
	if n != 1 || plan != "MONTHLY" || method != "MANUAL" {
		t.Fatalf("payments count=%d plan=%q method=%q, want 1 MONTHLY MANUAL", n, plan, method)
	}

	// The audit row records the mutation (rule 7).
	assertAudit(t, e.pool, userID, "LICENSE_VALIDITY_SET")
	var auditN int
	if err := e.pool.QueryRow(
		"SELECT COUNT(*) FROM audit_log WHERE action='LICENSE_VALIDITY_SET' AND subject_user_id=? AND `after` LIKE '%2026-11-10%'",
		userID).Scan(&auditN); err != nil {
		t.Fatalf("audit after: %v", err)
	}
	if auditN == 0 {
		t.Fatalf("LICENSE_VALIDITY_SET audit missing valid_until")
	}

	// Partial input → 400 (all-or-nothing, like import).
	rec = e.doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/dash/licenses/%d/validity", licenseID), map[string]any{
		"plan": "MONTHLY",
	}, tenantToken, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("partial validity = %d %s, want 400", rec.Code, rec.Body)
	}

	// A license in another tenant is not found (cross-tenant guard).
	otherToken := newSLVTenant(t, e, platformToken, "b")
	_, otherLicenseID := slvUserLicense(t, e, otherToken,
		fmt.Sprintf("slvo_%d", time.Now().UnixNano()), "+919876523003")
	rec = e.doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/dash/licenses/%d/validity", otherLicenseID), map[string]any{
		"plan": "MONTHLY", "paid_at": "2026-10-10", "valid_from": "2026-10-10",
	}, tenantToken, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant validity = %d %s, want 404", rec.Code, rec.Body)
	}
}
