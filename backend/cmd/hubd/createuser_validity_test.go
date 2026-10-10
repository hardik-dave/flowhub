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

// TestCreateUserWithValidity covers the dashboard "create user" path
// setting a subscription window (mirrors CSV import): the created
// license gets subscription_valid_until, a MANUAL payment row is
// written, and the USER_CREATED audit row records valid_until — all in
// one request/transaction (AGENTS.md rules 7, 9).
func TestCreateUserWithValidity(t *testing.T) {
	e := newMAEnv(t)

	platform := config.PlatformAdmin{
		Username: fmt.Sprintf("cuwadm_%d", time.Now().UnixNano()),
		Password: "platformpass123",
		Mobile:   "+919876522000",
	}
	if _, err := user.Bootstrap(context.Background(), e.store, platform); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer registerCleanup(t, e.pool, platform.Username)()
	platformToken := loginDash(t, e, platform.Username, platform.Password)

	slug := fmt.Sprintf("cuw%d", time.Now().UnixNano())
	adminUsername := fmt.Sprintf("cuwad_%d", time.Now().UnixNano())
	rec := e.doJSON(t, http.MethodPost, "/api/v1/dash/tenants", map[string]any{
		"slug": slug, "name": "CUW Tenant", "start_date": "2026-01-01",
		"products": []string{"optionalyzer"},
		"admin": map[string]any{
			"username": adminUsername, "password": "tenantpass123", "mobile": "+919876522001",
		},
	}, platformToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tenant = %d %s", rec.Code, rec.Body)
	}
	var ct struct {
		TenantID int64 `json:"tenant_id"`
	}
	decode(t, rec, &ct)
	defer deleteTenant(t, e, ct.TenantID)

	tenantToken := loginDash(t, e, adminUsername, "tenantpass123")

	// Create a user WITH a subscription window → valid_until is returned.
	algoUser := fmt.Sprintf("cuwu_%d", time.Now().UnixNano())
	rec = e.doJSON(t, http.MethodPost, "/api/v1/dash/users", map[string]any{
		"username": algoUser, "first_name": "A", "mobile": "+919876522002",
		"product_code": "optionalyzer", "plan": "MONTHLY",
		"paid_at": "2026-10-10", "valid_from": "2026-10-10",
	}, tenantToken, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user = %d %s", rec.Code, rec.Body)
	}
	var cu struct {
		UserID     int64  `json:"user_id"`
		LicenseKey string `json:"license_key"`
		ValidUntil string `json:"valid_until"`
	}
	decode(t, rec, &cu)
	if cu.UserID == 0 || cu.LicenseKey == "" {
		t.Fatalf("create user body = %s", rec.Body)
	}
	if cu.ValidUntil != "2026-11-10" {
		t.Fatalf("valid_until = %q, want 2026-11-10", cu.ValidUntil)
	}
	assertAudit(t, e.pool, cu.UserID, "USER_CREATED")

	// The license row carries the window.
	var until time.Time
	if err := e.pool.QueryRow("SELECT subscription_valid_until FROM licenses WHERE user_id = ?", cu.UserID).Scan(&until); err != nil {
		t.Fatalf("license valid_until: %v", err)
	}
	if got := until.Format("2006-01-02"); got != "2026-11-10" {
		t.Fatalf("license subscription_valid_until = %s, want 2026-11-10", got)
	}

	// A MANUAL payment row is written.
	var n int
	var plan string
	if err := e.pool.QueryRow("SELECT COUNT(*), COALESCE(MAX(plan),'') FROM payments WHERE user_id = ?", cu.UserID).Scan(&n, &plan); err != nil {
		t.Fatalf("payments: %v", err)
	}
	if n != 1 || plan != "MONTHLY" {
		t.Fatalf("payments count=%d plan=%q, want 1 MONTHLY", n, plan)
	}

	// The USER_CREATED audit row records valid_until.
	var auditN int
	if err := e.pool.QueryRow(
		"SELECT COUNT(*) FROM audit_log WHERE action='USER_CREATED' AND subject_user_id=? AND `after` LIKE '%2026-11-10%'",
		cu.UserID).Scan(&auditN); err != nil {
		t.Fatalf("audit after: %v", err)
	}
	if auditN == 0 {
		t.Fatalf("USER_CREATED audit missing valid_until")
	}

	// Partial validity input → 400 (all-or-nothing, like import).
	rec = e.doJSON(t, http.MethodPost, "/api/v1/dash/users", map[string]any{
		"username": algoUser + "x", "mobile": "+919876522003",
		"product_code": "optionalyzer", "plan": "MONTHLY",
	}, tenantToken, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("partial validity = %d %s, want 400", rec.Code, rec.Body)
	}
}
