// Package user serves the app-facing account endpoints: §6.3 register
// and §6.2 login. Login answers the license question in the same call
// by delegating to the verify service (SPEC §6.2).
package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/flowos/hub/internal/auth"
	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/license"
	"github.com/flowos/hub/internal/store"
	"github.com/flowos/hub/internal/verify"
)

// Handler serves §6.2 and §6.3.
type Handler struct {
	store    *store.Store
	otp      *auth.OTP
	verifier *verify.Service
	now      func() time.Time
}

func New(st *store.Store, otp *auth.OTP, verifier *verify.Service, now func() time.Time) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{store: st, otp: otp, verifier: verifier, now: now}
}

var (
	usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)
	// E.164: '+' then 8–15 digits, first digit non-zero.
	e164Re = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

// --- §6.3 register ---------------------------------------------------

type registerRequest struct {
	TenantSlug       string `json:"tenant_slug"`
	ProductCode      string `json:"product_code"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	Mobile           string `json:"mobile"`
	Email            string `json:"email"`
	City             string `json:"city"`
	State            string `json:"state"`
	BrokerClientCode string `json:"broker_client_code"`
	ReferralCode     string `json:"referral_code"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)
	req.ProductCode = strings.TrimSpace(req.ProductCode)
	req.Username = strings.TrimSpace(req.Username)
	req.Mobile = strings.TrimSpace(req.Mobile)

	if req.TenantSlug == "" {
		httpx.Error(w, http.StatusBadRequest, "tenant_slug is required.")
		return
	}
	if req.ProductCode == "" {
		httpx.Error(w, http.StatusBadRequest, "product_code is required.")
		return
	}
	if !usernameRe.MatchString(req.Username) {
		httpx.Error(w, http.StatusBadRequest, "username must be 3-64 characters: letters, digits, dot, underscore or hyphen.")
		return
	}
	if len(req.Password) < 8 {
		httpx.Error(w, http.StatusBadRequest, "password must be at least 8 characters.")
		return
	}
	if !e164Re.MatchString(req.Mobile) {
		httpx.Error(w, http.StatusBadRequest, "mobile must be E.164, e.g. +919876543210")
		return
	}

	tenant, err := h.store.TenantBySlug(ctx, req.TenantSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "Unknown tenant.")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "Could not complete registration.")
		return
	}
	product, err := h.store.ProductByCode(ctx, req.ProductCode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Error(w, http.StatusBadRequest, "Unknown product: "+req.ProductCode+".")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "Could not complete registration.")
		return
	}
	granted, err := h.store.ProductGrantedToTenant(ctx, tenant.ID, product.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not complete registration.")
		return
	}
	if !granted {
		httpx.Error(w, http.StatusBadRequest, "Tenant does not distribute product "+req.ProductCode+".")
		return
	}
	if _, err := h.store.UserByUsername(ctx, req.Username); err == nil {
		httpx.Error(w, http.StatusConflict, "That username is already taken.")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusInternalServerError, "Could not complete registration.")
		return
	}

	passwordHash, err := auth.HashSecret(req.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not complete registration.")
		return
	}
	key, err := license.NewKey(product.KeyPrefix)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not complete registration.")
		return
	}
	keyHash, err := auth.HashSecret(key)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not complete registration.")
		return
	}

	var userID int64
	otpRec, code := h.otp.Prepare(0, auth.PurposeRegistration)
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		id, err := h.store.InsertUser(ctx, tx, &store.User{
			Username:       req.Username,
			PasswordHash:   passwordHash,
			FirstName:      req.FirstName,
			LastName:       req.LastName,
			ContactNo:      req.Mobile,
			MobileVerified: false,
			Email:          req.Email,
			City:           req.City,
			State:          req.State,
			Status:         "PENDING_VERIFICATION",
		})
		if err != nil {
			return err
		}
		userID = id
		if err := h.store.InsertMembership(ctx, tx, tenant.ID, id, store.RoleAlgoUser); err != nil {
			return err
		}
		if _, err := h.store.InsertLicense(ctx, tx, &store.License{
			TenantID:       tenant.ID,
			UserID:         id,
			ProductID:      product.ID,
			LicenseKeyHash: keyHash,
			LicenseKeyHint: license.Hint(key),
			Status:         "ACTIVE",
		}); err != nil {
			return err
		}
		otpRec.UserID = id
		if _, err := h.store.InsertOTP(ctx, tx, otpRec); err != nil {
			return err
		}
		after, _ := json.Marshal(map[string]any{"username": req.Username, "product_code": req.ProductCode})
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID:      tenant.ID,
			Action:        "USER_REGISTERED",
			SubjectUserID: &id,
			After:         after,
			Reason:        "Self-service registration",
		})
	})
	if err != nil {
		if isDuplicate(err) {
			httpx.Error(w, http.StatusConflict, "That username is already taken.")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "Could not complete registration.")
		return
	}

	h.otp.Deliver(req.Mobile, code)
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"user_id":     userID,
		"license_key": key,
		"otp_sent":    true,
	})
}

// --- §6.2 login ------------------------------------------------------

type loginRequest struct {
	TenantSlug  string `json:"tenant_slug"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	ProductCode string `json:"product_code"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)
	req.Username = strings.TrimSpace(req.Username)
	req.ProductCode = strings.TrimSpace(req.ProductCode)

	tenant, err := h.store.TenantBySlug(ctx, req.TenantSlug)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid username or password.")
		return
	}
	user, err := h.store.UserByUsername(ctx, req.Username)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid username or password.")
		return
	}
	mem, err := h.store.MembershipForUser(ctx, tenant.ID, user.ID)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid username or password.")
		return
	}
	match, err := auth.VerifySecret(user.PasswordHash, req.Password)
	if err != nil || !match {
		httpx.Error(w, http.StatusUnauthorized, "Invalid username or password.")
		return
	}

	if !user.MobileVerified {
		httpx.JSON(w, http.StatusConflict, map[string]any{
			"error":        "Mobile verification required.",
			"otp_required": true,
		})
		return
	}
	if req.ProductCode == "" {
		httpx.Error(w, http.StatusBadRequest, "product_code is required.")
		return
	}

	verdict, err := h.verifier.CheckAccount(ctx, req.TenantSlug, req.Username, req.ProductCode)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not process the license check.")
		return
	}
	if verdict.Status != verify.StatusValid {
		reason := verdict.Reason
		if reason == "" {
			reason = verify.ReasonNotFound
		}
		httpx.Error(w, http.StatusForbidden, reason)
		return
	}

	token, hash, err := auth.NewSessionToken()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not start a session.")
		return
	}
	if err := h.store.InTx(ctx, func(tx store.DBTX) error {
		_, err := h.store.InsertSession(ctx, tx, &store.Session{
			UserID:    user.ID,
			Audience:  auth.AudienceApp,
			TokenHash: hash,
			ExpiresAt: h.now().UTC().Add(auth.AppTTL * time.Second),
		})
		return err
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
		},
		"verify": verdict,
	})
}

func isDuplicate(err error) bool {
	var me *mysqlDriver.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
