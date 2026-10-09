package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flowos/hub/internal/auth"
	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/dashboard"
	"github.com/flowos/hub/internal/db"
	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/migrate"
	"github.com/flowos/hub/internal/store"
	"github.com/flowos/hub/internal/user"
	"github.com/flowos/hub/internal/verify"
)

// captureSender stands in for ConsoleSender so the test can read the OTP
// that would otherwise only reach a developer's terminal.
type captureSender struct {
	mu   sync.Mutex
	code string
}

func (c *captureSender) SendOTP(_, code string) error {
	c.mu.Lock()
	c.code = code
	c.mu.Unlock()
	return nil
}

func (c *captureSender) lastCode() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.code
}

type maEnv struct {
	router http.Handler
	store  *store.Store
	pool   *sql.DB
	sender *captureSender
}

func newMAEnv(t *testing.T) *maEnv {
	return newMAEnvLimited(t, 1000)
}

// newMAEnvLimited builds the same harness with a different per-IP rate
// limit so the M-D rate-limit test can trip the §9 login ceiling.
func newMAEnvLimited(t *testing.T, limit int) *maEnv {
	t.Helper()
	dsn := os.Getenv("HUB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("HUB_TEST_DATABASE_URL not set — skipping M-A DB-backed test")
	}
	if err := migrate.Up(filepath.Join("..", "..", "migrations"), dsn); err != nil {
		t.Fatalf("migrate.Up: %v", err)
	}
	pool, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	st := store.New(pool)
	sender := &captureSender{}
	now := time.Now
	ah := auth.NewHandler(st, sender, now)
	vs := verify.NewService(st, now)
	h := &handlers{
		verify:   verify.NewHandler(vs),
		auth:     ah,
		user:     user.New(st, ah.OTP(), vs, now),
		dash:     dashboard.New(st, now),
		dashAuth: auth.RequireDashboard(st),
	}
	cfg := &config.Config{DashboardOrigin: "http://localhost:5173"}
	router := newRouter(cfg, httpx.NewRateLimiter(limit, time.Minute), h)
	return &maEnv{router: router, store: st, pool: pool, sender: sender}
}

func (e *maEnv) postJSON(t *testing.T, path string, body any, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
}

// TestRegisterOTPVerifyLogin is the M-A acceptance round-trip (§10):
// register → OTP (captured) → login, asserting the §6.2 status ladder
// and that mutations wrote their audit rows.
func TestRegisterOTPVerifyLogin(t *testing.T) {
	e := newMAEnv(t)
	username := fmt.Sprintf("matest_%d", time.Now().UnixNano())
	cleanup := registerCleanup(t, e.pool, username)
	defer cleanup()

	const password = "supersecret1"
	const mobile = "+919876500000"
	loginBody := map[string]any{
		"tenant_slug": "flowos", "username": username, "password": password, "product_code": "flowos",
	}

	// 1. register → 201, key shown once, registration OTP delivered.
	rec := e.postJSON(t, "/api/v1/app/register", map[string]any{
		"tenant_slug": "flowos", "product_code": "flowos", "username": username,
		"password": password, "first_name": "M", "last_name": "A", "mobile": mobile,
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d %s", rec.Code, rec.Body)
	}
	var reg struct {
		UserID     int64  `json:"user_id"`
		LicenseKey string `json:"license_key"`
		OTPSent    bool   `json:"otp_sent"`
	}
	decode(t, rec, &reg)
	if reg.UserID == 0 || !reg.OTPSent || !strings.HasPrefix(reg.LicenseKey, "FL-") {
		t.Fatalf("register body = %s", rec.Body)
	}
	code := e.sender.lastCode()
	if code == "" {
		t.Fatal("registration did not deliver an OTP")
	}

	// 2. login before verification → 409 otp_required (§6.2).
	rec = e.postJSON(t, "/api/v1/app/login", loginBody, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("pre-verify login = %d %s, want 409", rec.Code, rec.Body)
	}
	var otpBody struct {
		OTPRequired bool `json:"otp_required"`
	}
	decode(t, rec, &otpBody)
	if !otpBody.OTPRequired {
		t.Fatalf("409 body missing otp_required: %s", rec.Body)
	}

	// 3. verify the registration OTP → 200 verified.
	rec = e.postJSON(t, "/api/v1/app/otp/verify", map[string]any{
		"tenant_slug": "flowos", "username": username, "purpose": "REGISTRATION", "code": code,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("otp/verify = %d %s", rec.Code, rec.Body)
	}
	assertAudit(t, e.pool, reg.UserID, "MOBILE_VERIFIED")
	assertAudit(t, e.pool, reg.UserID, "USER_REGISTERED")

	// 4. login now succeeds on credentials but is 403 — no payment yet (§4).
	rec = e.postJSON(t, "/api/v1/app/login", loginBody, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unpaid login = %d %s, want 403", rec.Code, rec.Body)
	}
	var fail struct {
		Error string `json:"error"`
	}
	decode(t, rec, &fail)
	if !strings.Contains(fail.Error, "awaiting activation or payment") {
		t.Fatalf("403 reason = %q, want awaiting-activation template", fail.Error)
	}

	// 5. verify with a wrong key → not_found (exact §6.1 string).
	rec = e.postJSON(t, "/api/v1/app/verify", map[string]any{
		"tenant_slug": "flowos", "username": username, "product_code": "flowos",
		"license_key": "FL-0000-0000-0000-0000",
	}, "")
	var v struct {
		Status string `json:"status"`
	}
	decode(t, rec, &v)
	if rec.Code != http.StatusOK || v.Status != "not_found" {
		t.Fatalf("wrong-key verify = %d %s, want 200 not_found", rec.Code, rec.Body)
	}

	// 6. record a subscription window, login → 200 valid with §6.1 body.
	if _, err := e.pool.Exec(
		"UPDATE licenses SET subscription_valid_until = DATE_ADD(CURDATE(), INTERVAL 30 DAY) WHERE user_id = ?",
		reg.UserID); err != nil {
		t.Fatalf("extend subscription: %v", err)
	}
	rec = e.postJSON(t, "/api/v1/app/login", loginBody, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("paid login = %d %s, want 200", rec.Code, rec.Body)
	}
	var ok struct {
		SessionToken string `json:"session_token"`
		User         struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"user"`
		Verify struct {
			Status       string          `json:"status"`
			Entitlements json.RawMessage `json:"entitlements"`
		} `json:"verify"`
	}
	decode(t, rec, &ok)
	if ok.SessionToken == "" || ok.User.Role != "ALGO_USER" {
		t.Fatalf("login body = %s", rec.Body)
	}
	if ok.Verify.Status != "valid" || len(ok.Verify.Entitlements) == 0 {
		t.Fatalf("login verify = %s, want valid with entitlements", rec.Body)
	}

	// 7. verify with the real key → valid.
	rec = e.postJSON(t, "/api/v1/app/verify", map[string]any{
		"tenant_slug": "flowos", "username": username, "product_code": "flowos",
		"license_key": reg.LicenseKey,
	}, "")
	decode(t, rec, &v)
	if v.Status != "valid" {
		t.Fatalf("real-key verify status = %q, want valid (%s)", v.Status, rec.Body)
	}
}

// TestBootstrapAndDashboardLogin covers the first-boot platform admin
// and the §7 dashboard session lifecycle.
func TestBootstrapAndDashboardLogin(t *testing.T) {
	e := newMAEnv(t)
	admin := config.PlatformAdmin{
		Username: fmt.Sprintf("maadmin_%d", time.Now().UnixNano()),
		Password: "adminpass123",
		Mobile:   "+919876500001",
	}
	cleanup := registerCleanup(t, e.pool, admin.Username)
	defer cleanup()

	created, err := user.Bootstrap(context.Background(), e.store, admin)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if !created {
		t.Fatal("Bootstrap first run = false, want true")
	}
	created, err = user.Bootstrap(context.Background(), e.store, admin)
	if err != nil || created {
		t.Fatalf("Bootstrap second run = (%v, %v), want (false, nil)", created, err)
	}

	rec := e.postJSON(t, "/api/v1/dash/login", map[string]any{
		"username": admin.Username, "password": admin.Password,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("dash login = %d %s", rec.Code, rec.Body)
	}
	var dl struct {
		SessionToken string `json:"session_token"`
		User         struct {
			Role     string `json:"role"`
			TenantID int64  `json:"tenant_id"`
		} `json:"user"`
	}
	decode(t, rec, &dl)
	if dl.SessionToken == "" || dl.User.Role != "SUPERADMIN" || dl.User.TenantID != 1 {
		t.Fatalf("dash login body = %s", rec.Body)
	}

	rec = e.postJSON(t, "/api/v1/dash/logout", map[string]any{}, dl.SessionToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout = %d %s", rec.Code, rec.Body)
	}
	rec = e.postJSON(t, "/api/v1/dash/logout", map[string]any{}, dl.SessionToken)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("second logout = %d, want 401", rec.Code)
	}

	var adminID int64
	if err := e.pool.QueryRow("SELECT id FROM users WHERE username = ?", admin.Username).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	assertAudit(t, e.pool, adminID, "PLATFORM_ADMIN_BOOTSTRAPPED")
	assertAudit(t, e.pool, adminID, "ADMIN_LOGIN")
}

func assertAudit(t *testing.T, pool *sql.DB, userID int64, action string) {
	t.Helper()
	var n int
	if err := pool.QueryRow(
		"SELECT COUNT(*) FROM audit_log WHERE action = ? AND (subject_user_id = ? OR actor_user_id = ?)",
		action, userID, userID).Scan(&n); err != nil {
		t.Fatalf("audit count %s: %v", action, err)
	}
	if n == 0 {
		t.Errorf("no audit_log row for action %s (user %d)", action, userID)
	}
}

// registerCleanup removes every row a test user touched so repeated
// local runs stay idempotent.
func registerCleanup(t *testing.T, pool *sql.DB, username string) func() {
	return func() {
		var id int64
		if err := pool.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&id); err != nil {
			return
		}
		for _, q := range []string{
			"DELETE FROM sessions WHERE user_id = ?",
			"DELETE FROM otp_codes WHERE user_id = ?",
			"DELETE FROM audit_log WHERE subject_user_id = ? OR actor_user_id = ?",
			"DELETE FROM licenses WHERE user_id = ?",
			"DELETE FROM tenant_users WHERE user_id = ?",
			"DELETE FROM users WHERE id = ?",
		} {
			if strings.Count(q, "?") == 2 {
				_, _ = pool.Exec(q, id, id)
			} else {
				_, _ = pool.Exec(q, id)
			}
		}
	}
}
