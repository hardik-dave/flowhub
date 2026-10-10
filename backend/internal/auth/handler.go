package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/store"
)

// Handler serves the §6.4 OTP endpoints and §7 dashboard login/logout.
type Handler struct {
	store      *store.Store
	otp        *OTP
	now        func() time.Time
	devEchoOTP bool
}

// NewHandler builds the OTP service too; the user package reuses it for
// the registration OTP. devEchoOTP is a dev-only convenience: when true,
// /app/otp/request echoes the plaintext code in its response so a local
// dashboard can auto-fill it, and the §6.4 resend cooldown is bypassed so
// the developer is never locked out. It must be false in production.
func NewHandler(st *store.Store, sender SmsSender, now func() time.Time, devEchoOTP bool) *Handler {
	if now == nil {
		now = time.Now
	}
	o := NewOTP(st, sender, now)
	o.SetSkipCooldown(devEchoOTP)
	return &Handler{store: st, otp: o, now: now, devEchoOTP: devEchoOTP}
}

// OTP exposes the underlying OTP service for the registration flow.
func (h *Handler) OTP() *OTP { return h.otp }

// --- §6.4 OTP --------------------------------------------------------

type otpRequest struct {
	TenantSlug string `json:"tenant_slug"`
	Username   string `json:"username"`
	Purpose    string `json:"purpose"`
}

func (h *Handler) OTPRequest(w http.ResponseWriter, r *http.Request) {
	var req otpRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Purpose = strings.ToUpper(strings.TrimSpace(req.Purpose))
	if req.Purpose != PurposeRegistration && req.Purpose != PurposeFirstLogin {
		httpx.Error(w, http.StatusBadRequest, "purpose must be REGISTRATION or FIRST_LOGIN.")
		return
	}
	tenant, user, ok := h.resolveMember(w, r, req.TenantSlug, req.Username)
	if !ok {
		return
	}
	_ = tenant

	expiresIn, code, err := h.otp.Issue(r.Context(), user.ID, user.ContactNo, req.Purpose)
	switch {
	case errors.Is(err, ErrCooldown):
		httpx.Error(w, http.StatusTooManyRequests, "A code was just sent. Please wait 60 seconds before requesting another.")
		return
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "Could not send the verification code.")
		return
	}
	resp := map[string]any{"otp_sent": true, "expires_in_seconds": expiresIn}
	if h.devEchoOTP {
		resp["dev_code"] = code
	}
	httpx.JSON(w, http.StatusOK, resp)
}

type otpVerify struct {
	TenantSlug string `json:"tenant_slug"`
	Username   string `json:"username"`
	Purpose    string `json:"purpose"`
	Code       string `json:"code"`
}

func (h *Handler) OTPVerify(w http.ResponseWriter, r *http.Request) {
	var req otpVerify
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Purpose = strings.ToUpper(strings.TrimSpace(req.Purpose))
	if req.Purpose != PurposeRegistration && req.Purpose != PurposeFirstLogin {
		httpx.Error(w, http.StatusBadRequest, "purpose must be REGISTRATION or FIRST_LOGIN.")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		httpx.Error(w, http.StatusBadRequest, "code is required.")
		return
	}
	tenant, user, ok := h.resolveMember(w, r, req.TenantSlug, req.Username)
	if !ok {
		return
	}

	rec, err := h.otp.Check(r.Context(), user.ID, req.Purpose, req.Code)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "That code is invalid or has expired.")
		return
	}

	subject := user.ID
	err = h.store.InTx(r.Context(), func(tx store.DBTX) error {
		at := h.now().UTC()
		if err := h.store.ConsumeOTP(r.Context(), tx, rec.ID, at); err != nil {
			return err
		}
		if err := h.store.MarkMobileVerified(r.Context(), tx, user.ID); err != nil {
			return err
		}
		return h.store.InsertAudit(r.Context(), tx, &store.Audit{
			TenantID:      tenant.ID,
			Action:        "MOBILE_VERIFIED",
			SubjectUserID: &subject,
			After:         []byte(`{"mobile_verified":true}`),
			Reason:        "Mobile verified via OTP",
		})
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not record verification.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"verified": true})
}

// resolveMember loads tenant + user and checks membership, writing the
// caller-appropriate error and returning ok=false on any miss.
func (h *Handler) resolveMember(w http.ResponseWriter, r *http.Request, tenantSlug, username string) (*store.Tenant, *store.User, bool) {
	ctx := r.Context()
	tenantSlug = strings.TrimSpace(tenantSlug)
	username = strings.TrimSpace(username)
	if tenantSlug == "" || username == "" {
		httpx.Error(w, http.StatusBadRequest, "tenant_slug and username are required.")
		return nil, nil, false
	}
	tenant, err := h.store.TenantBySlug(ctx, tenantSlug)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Unknown tenant.")
		return nil, nil, false
	}
	user, err := h.store.UserByUsername(ctx, username)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "No account matches that username for this tenant.")
		return nil, nil, false
	}
	if _, err := h.store.MembershipForUser(ctx, tenant.ID, user.ID); err != nil {
		httpx.Error(w, http.StatusNotFound, "No account matches that username for this tenant.")
		return nil, nil, false
	}
	return tenant, user, true
}

// --- §7 dashboard login/logout --------------------------------------

type dashLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) DashLogin(w http.ResponseWriter, r *http.Request) {
	var req dashLoginRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	user, err := h.store.UserByUsername(ctx, strings.TrimSpace(req.Username))
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid username or password.")
		return
	}
	match, err := VerifySecret(user.PasswordHash, req.Password)
	if err != nil || !match {
		httpx.Error(w, http.StatusUnauthorized, "Invalid username or password.")
		return
	}
	if user.Status != "ACTIVE" {
		httpx.Error(w, http.StatusForbidden, "This account is not active.")
		return
	}
	mem, err := h.store.FirstMembership(ctx, user.ID)
	if err != nil || !mem.Role.DashboardRole() {
		httpx.Error(w, http.StatusForbidden, "This account does not have dashboard access.")
		return
	}

	token, hash, err := NewSessionToken()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not start a session.")
		return
	}
	expires := h.now().UTC().Add(DashboardTTL * time.Second)
	actor := user.ID
	if err := h.store.InTx(ctx, func(tx store.DBTX) error {
		if _, err := h.store.InsertSession(ctx, tx, &store.Session{
			UserID: user.ID, Audience: AudienceDashboard, TokenHash: hash, ExpiresAt: expires,
		}); err != nil {
			return err
		}
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID:    mem.TenantID,
			ActorUserID: &actor,
			Action:      "ADMIN_LOGIN",
		})
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not start a session.")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"session_token": token,
		"user": map[string]any{
			"id":         user.ID,
			"username":   user.Username,
			"first_name": user.FirstName,
			"role":       string(mem.Role),
			"tenant_id":  mem.TenantID,
		},
	})
}

func (h *Handler) DashLogout(w http.ResponseWriter, r *http.Request) {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Authentication required.")
		return
	}
	if err := h.store.DeleteSession(r.Context(), p.TokenHash); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not sign out.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
