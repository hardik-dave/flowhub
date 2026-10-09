package main

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/user"
)

func (e *maEnv) postCSV(t *testing.T, filename, content, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dash/imports", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// TestCSVImportAndFirstLoginOTP is the M-D acceptance test (§8/§10):
// import a CSV with per-row successes and failures, then drive the
// imported user through FIRST_LOGIN OTP to a valid app login.
func TestCSVImportAndFirstLoginOTP(t *testing.T) {
	e := newMAEnv(t)

	platform := config.PlatformAdmin{
		Username: fmt.Sprintf("cs5adm_%d", time.Now().UnixNano()),
		Password: "platformpass123",
		Mobile:   "+919876533000",
	}
	if _, err := user.Bootstrap(context.Background(), e.store, platform); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer registerCleanup(t, e.pool, platform.Username)()
	platformToken := loginDash(t, e, platform.Username, platform.Password)

	slug := fmt.Sprintf("i%d", time.Now().UnixNano())
	adminUsername := fmt.Sprintf("isadm_%d", time.Now().UnixNano())
	rec := e.doJSON(t, http.MethodPost, "/api/v1/dash/tenants", map[string]any{
		"slug": slug, "name": "Import Tenant", "start_date": "2026-01-01", "products": []string{"flowos"},
		"admin": map[string]any{"username": adminUsername, "password": "tenantpass123", "mobile": "+919876533001"},
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

	today := time.Now().UTC().Format("2006-01-02")
	good := fmt.Sprintf("imp_%d", time.Now().UnixNano())
	csv := strings.Join([]string{
		"username,first_name,last_name,mobile,email,city,state,broker_client_code,referral_code,product_code,plan,paid_at,valid_from",
		strings.Join([]string{good, "Imp", "One", "+919876533100", "", "", "", "BRK1", "REF1", "flowos", "MONTHLY", today, today}, ","),
		strings.Join([]string{good + "_dup", "Dup", "Mobile", "+919876533100", "", "", "", "", "", "flowos", "", "", ""}, ","),
		strings.Join([]string{good + "_bad", "Bad", "Mobile", "12345", "", "", "", "", "", "flowos", "", "", ""}, ","),
		strings.Join([]string{good + "_nop", "No", "Product", "+919876533101", "", "", "", "", "", "ghost", "", "", ""}, ","),
	}, "\n")

	rec = e.postCSV(t, "users.csv", csv, tenantToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("import = %d %s", rec.Code, rec.Body)
	}
	var imp struct {
		BatchID    int64 `json:"batch_id"`
		RowCount   int   `json:"row_count"`
		OKCount    int   `json:"ok_count"`
		ErrorCount int   `json:"error_count"`
		Errors     []struct {
			Row    int    `json:"row"`
			Reason string `json:"reason"`
		} `json:"errors"`
		Created []struct {
			Row        int    `json:"row"`
			Username   string `json:"username"`
			Password   string `json:"password"`
			LicenseKey string `json:"license_key"`
		} `json:"created"`
	}
	decode(t, rec, &imp)
	if imp.BatchID == 0 || imp.RowCount != 4 || imp.OKCount != 1 || imp.ErrorCount != 3 {
		t.Fatalf("import counts = %s", rec.Body)
	}
	if len(imp.Created) != 1 || imp.Created[0].Username != good || imp.Created[0].Password == "" || !strings.HasPrefix(imp.Created[0].LicenseKey, "FL-") {
		t.Fatalf("import created = %s", rec.Body)
	}
	password := imp.Created[0].Password

	var userID int64
	if err := e.pool.QueryRow("SELECT id FROM users WHERE username = ?", good).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	assertAudit(t, e.pool, userID, "USER_IMPORTED")

	// Imported user: app login → 409 otp_required (mobile_verified=false).
	loginBody := map[string]any{"tenant_slug": slug, "username": good, "password": password, "product_code": "flowos"}
	rec = e.postJSON(t, "/api/v1/app/login", loginBody, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("imported pre-verify login = %d %s, want 409", rec.Code, rec.Body)
	}

	// FIRST_LOGIN OTP: request → capture → verify.
	rec = e.postJSON(t, "/api/v1/app/otp/request", map[string]any{
		"tenant_slug": slug, "username": good, "purpose": "FIRST_LOGIN",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("first-login otp request = %d %s", rec.Code, rec.Body)
	}
	code := e.sender.lastCode()
	if code == "" {
		t.Fatal("FIRST_LOGIN OTP was not delivered")
	}
	rec = e.postJSON(t, "/api/v1/app/otp/verify", map[string]any{
		"tenant_slug": slug, "username": good, "purpose": "FIRST_LOGIN", "code": code,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("first-login otp verify = %d %s", rec.Code, rec.Body)
	}

	// Login now succeeds and the imported payment made the license valid.
	rec = e.postJSON(t, "/api/v1/app/login", loginBody, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("imported post-verify login = %d %s, want 200", rec.Code, rec.Body)
	}
	var ok struct {
		Verify struct {
			Status string `json:"status"`
		} `json:"verify"`
	}
	decode(t, rec, &ok)
	if ok.Verify.Status != "valid" {
		t.Fatalf("imported login verify status = %q, want valid (%s)", ok.Verify.Status, rec.Body)
	}
}

// TestLoginRateLimit verifies the §9 10/min per-IP ceiling (here set to
// 2) trips with 429.
func TestLoginRateLimit(t *testing.T) {
	e := newMAEnvLimited(t, 2)
	body := map[string]any{"tenant_slug": "flowos", "username": "nobody", "password": "wrongpass", "product_code": "flowos"}
	for i := 0; i < 2; i++ {
		if rec := e.postJSON(t, "/api/v1/app/login", body, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("login attempt %d = %d, want 401", i+1, rec.Code)
		}
	}
	if rec := e.postJSON(t, "/api/v1/app/login", body, ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third login = %d, want 429", rec.Code)
	}
}
