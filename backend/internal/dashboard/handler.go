// Package dashboard serves the §7 dashboard endpoints. Every request is
// authenticated by auth.RequireDashboard, which resolves an
// auth.Scope{TenantID, Role, IsPlatformAdmin}; every store call below is
// scoped by sc.TenantID except the documented platform-admin tenant
// paths (AGENTS.md rule 6). Status/role/license mutations write their
// audit_log row in the same transaction (rule 7).
package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/flowos/hub/internal/auth"
	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/license"
	"github.com/flowos/hub/internal/store"
)

// pageSize is the fixed dashboard page size.
const pageSize = 25

type Handler struct {
	store *store.Store
	now   func() time.Time
}

func New(st *store.Store, now func() time.Time) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{store: st, now: now}
}

var (
	slugRe     = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)
	usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)
	e164Re     = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

var (
	validUserStatuses    = map[string]bool{"ACTIVE": true, "DEACTIVATED": true, "BANNED": true}
	validLicenseStatuses = map[string]bool{"ACTIVE": true, "DISABLED": true}
	validTenantStatuses  = map[string]bool{"ACTIVE": true, "INACTIVE": true}
)

// --- shared helpers --------------------------------------------------

func mustScope(w http.ResponseWriter, r *http.Request) (auth.Scope, bool) {
	sc, ok := auth.ScopeFrom(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Authentication required.")
		return auth.Scope{}, false
	}
	return sc, true
}

func pageOf(r *http.Request) (page, offset int) {
	page = 1
	if v := strings.TrimSpace(r.URL.Query().Get("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	return page, (page - 1) * pageSize
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil && id > 0
}

func dateStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02")
	return &s
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func isDuplicate(err error) bool {
	var me *mysqlDriver.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// --- view types ------------------------------------------------------

type licenseView struct {
	ID          int64   `json:"id"`
	ProductCode string  `json:"product_code"`
	ProductName string  `json:"product_name"`
	Status      string  `json:"status"`
	KeyHint     string  `json:"key_hint"`
	ValidUntil  *string `json:"valid_until"`
}

type userView struct {
	ID             int64         `json:"id"`
	Username       string        `json:"username"`
	FirstName      string        `json:"first_name"`
	LastName       string        `json:"last_name"`
	Mobile         string        `json:"mobile"`
	Email          string        `json:"email"`
	City           string        `json:"city"`
	State          string        `json:"state"`
	Status         string        `json:"status"`
	MobileVerified bool          `json:"mobile_verified"`
	Role           string        `json:"role"`
	Licenses       []licenseView `json:"licenses"`
}

func (h *Handler) buildUserView(ctx context.Context, tenantID int64, u *store.User) (userView, error) {
	v := userView{
		ID: u.ID, Username: u.Username, FirstName: u.FirstName, LastName: u.LastName,
		Mobile: u.ContactNo, Email: u.Email, City: u.City, State: u.State,
		Status: u.Status, MobileVerified: u.MobileVerified,
	}
	if m, err := h.store.MembershipForUser(ctx, tenantID, u.ID); err == nil {
		v.Role = string(m.Role)
	}
	lics, err := h.store.ListUserLicenses(ctx, tenantID, u.ID)
	if err != nil {
		return v, err
	}
	// Start non-nil so users with no licenses serialize as [] rather than
	// null; the dashboard client expects a list (and would crash on null).
	v.Licenses = make([]licenseView, 0, len(lics))
	for _, l := range lics {
		v.Licenses = append(v.Licenses, licenseView{
			ID: l.ID, ProductCode: l.ProductCode, ProductName: l.ProductName,
			Status: l.Status, KeyHint: l.KeyHint, ValidUntil: dateStr(l.SubscriptionValidUntil),
		})
	}
	return v, nil
}

type paymentView struct {
	ID               int64     `json:"id"`
	AmountMinorUnits int64     `json:"amount_minor_units"`
	Currency         string    `json:"currency"`
	Method           string    `json:"method"`
	Plan             string    `json:"plan"`
	PaidAt           time.Time `json:"paid_at"`
	ValidFrom        time.Time `json:"valid_from"`
	ValidUntil       time.Time `json:"valid_until"`
	Note             string    `json:"note,omitempty"`
}

type auditView struct {
	ID            int64           `json:"id"`
	Action        string          `json:"action"`
	ActorUserID   *int64          `json:"actor_user_id"`
	SubjectUserID *int64          `json:"subject_user_id"`
	Before        json.RawMessage `json:"before,omitempty"`
	After         json.RawMessage `json:"after,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	At            time.Time       `json:"at"`
}

func auditViews(rows []store.AuditRow) []auditView {
	out := make([]auditView, 0, len(rows))
	for _, a := range rows {
		out = append(out, auditView{
			ID: a.ID, Action: a.Action, ActorUserID: a.ActorUserID, SubjectUserID: a.SubjectUserID,
			Before: a.Before, After: a.After, Reason: a.Reason, At: a.At,
		})
	}
	return out
}

type tenantView struct {
	ID               int64   `json:"id"`
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	Status           string  `json:"status"`
	IsHouse          bool    `json:"is_house"`
	GraceWorkingDays int     `json:"grace_working_days"`
	ContactPerson    string  `json:"contact_person"`
	ContactNo        string  `json:"contact_no"`
	StartDate        string  `json:"start_date"`
	EndDate          *string `json:"end_date"`
	// Products lists the product codes this tenant may distribute. Populated
	// only on GET /dash/tenant (the acting tenant's own view); omitted
	// elsewhere and when empty.
	Products []string `json:"products,omitempty"`
}

func tenantViewOf(t *store.Tenant) tenantView {
	return tenantView{
		ID: t.ID, Slug: t.Slug, Name: t.Name, Status: t.Status, IsHouse: t.IsHouse,
		GraceWorkingDays: t.GraceWorkingDays, ContactPerson: t.ContactPerson, ContactNo: t.ContactNo,
		StartDate: t.StartDate.UTC().Format("2006-01-02"), EndDate: dateStr(t.EndDate),
	}
}

// --- §7 users --------------------------------------------------------

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	f := store.UserFilter{
		Status:      strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status"))),
		ProductCode: strings.TrimSpace(r.URL.Query().Get("product")),
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
	}
	if f.Status != "" && !validUserStatuses[f.Status] {
		httpx.Error(w, http.StatusBadRequest, "status must be ACTIVE, DEACTIVATED or BANNED.")
		return
	}
	page, offset := pageOf(r)
	f.Offset, f.Limit = offset, pageSize

	users, err := h.store.ListUsers(ctx, sc.TenantID, f)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not list users.")
		return
	}
	total, err := h.store.CountUsers(ctx, sc.TenantID, f)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not list users.")
		return
	}
	out := make([]userView, 0, len(users))
	for i := range users {
		v, err := h.buildUserView(ctx, sc.TenantID, &users[i])
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "Could not list users.")
			return
		}
		out = append(out, v)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"users": out, "page": page, "page_size": pageSize, "total": total,
	})
}

type createUserRequest struct {
	Username         string `json:"username"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	Mobile           string `json:"mobile"`
	Email            string `json:"email"`
	City             string `json:"city"`
	State            string `json:"state"`
	BrokerClientCode string `json:"broker_client_code"`
	ReferralCode     string `json:"referral_code"`
	ProductCode      string `json:"product_code"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var req createUserRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Mobile = strings.TrimSpace(req.Mobile)
	req.ProductCode = strings.TrimSpace(req.ProductCode)

	if !usernameRe.MatchString(req.Username) {
		httpx.Error(w, http.StatusBadRequest, "username must be 3-64 characters: letters, digits, dot, underscore or hyphen.")
		return
	}
	if !e164Re.MatchString(req.Mobile) {
		httpx.Error(w, http.StatusBadRequest, "mobile must be E.164, e.g. +919876543210")
		return
	}
	if req.ProductCode == "" {
		httpx.Error(w, http.StatusBadRequest, "product_code is required.")
		return
	}
	product, err := h.store.ProductByCode(ctx, req.ProductCode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Error(w, http.StatusBadRequest, "Unknown product: "+req.ProductCode+".")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "Could not create the user.")
		return
	}
	granted, err := h.store.ProductGrantedToTenant(ctx, sc.TenantID, product.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not create the user.")
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
		httpx.Error(w, http.StatusInternalServerError, "Could not create the user.")
		return
	}

	password, err := auth.RandomPassword()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not create the user.")
		return
	}
	passwordHash, err := auth.HashSecret(password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not create the user.")
		return
	}
	key, err := license.NewKey(product.KeyPrefix)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not create the user.")
		return
	}
	keyHash, err := auth.HashSecret(key)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not create the user.")
		return
	}

	var userID int64
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		id, err := h.store.InsertUser(ctx, tx, &store.User{
			Username: req.Username, PasswordHash: passwordHash,
			FirstName: req.FirstName, LastName: req.LastName, ContactNo: req.Mobile,
			MobileVerified: false, Email: req.Email, City: req.City, State: req.State,
			Status: "PENDING_VERIFICATION",
		})
		if err != nil {
			return err
		}
		userID = id
		if err := h.store.InsertMembership(ctx, tx, sc.TenantID, id, store.RoleAlgoUser); err != nil {
			return err
		}
		if _, err := h.store.InsertLicense(ctx, tx, &store.License{
			TenantID: sc.TenantID, UserID: id, ProductID: product.ID,
			LicenseKeyHash: keyHash, LicenseKeyHint: license.Hint(key), Status: "ACTIVE",
		}); err != nil {
			return err
		}
		actor := sc.UserID
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID: sc.TenantID, ActorUserID: &actor, Action: "USER_CREATED", SubjectUserID: &id,
			After:  mustJSON(map[string]any{"username": req.Username, "product_code": req.ProductCode, "role": string(store.RoleAlgoUser)}),
			Reason: "Created by admin",
		})
	})
	if err != nil {
		if isDuplicate(err) {
			httpx.Error(w, http.StatusConflict, "That username is already taken.")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "Could not create the user.")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"user_id": userID, "username": req.Username, "password": password, "license_key": key,
	})
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	id, ok := pathID(r)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "No such user.")
		return
	}
	if _, err := h.store.MembershipForUser(ctx, sc.TenantID, id); err != nil {
		httpx.Error(w, http.StatusNotFound, "No such user.")
		return
	}
	u, err := h.store.UserByID(ctx, id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "No such user.")
		return
	}
	view, err := h.buildUserView(ctx, sc.TenantID, u)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not load the user.")
		return
	}
	payments, err := h.store.ListPaymentsByUser(ctx, sc.TenantID, id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not load the user.")
		return
	}
	pv := make([]paymentView, 0, len(payments))
	for _, p := range payments {
		pv = append(pv, paymentView{
			ID: p.ID, AmountMinorUnits: p.AmountMinorUnits, Currency: p.Currency, Method: p.Method,
			Plan: p.Plan, PaidAt: p.PaidAt, ValidFrom: p.ValidFrom, ValidUntil: p.ValidUntil, Note: p.Note,
		})
	}
	auditRows, err := h.store.ListAudit(ctx, sc.TenantID, &id, 0, 50)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not load the user.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"user": view, "payments": pv, "audit": auditViews(auditRows),
	})
}

type statusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (h *Handler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	id, ok := pathID(r)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "No such user.")
		return
	}
	var req statusRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	reason := strings.TrimSpace(req.Reason)
	if !validUserStatuses[status] {
		httpx.Error(w, http.StatusBadRequest, "status must be ACTIVE, DEACTIVATED or BANNED.")
		return
	}
	if reason == "" {
		httpx.Error(w, http.StatusBadRequest, "A reason is required for a status change.")
		return
	}
	if _, err := h.store.MembershipForUser(ctx, sc.TenantID, id); err != nil {
		httpx.Error(w, http.StatusNotFound, "No such user.")
		return
	}
	u, err := h.store.UserByID(ctx, id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "No such user.")
		return
	}
	bump := status == "DEACTIVATED" || status == "BANNED"
	actor := sc.UserID
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		if err := h.store.UpdateUserStatus(ctx, tx, sc.TenantID, id, status, bump); err != nil {
			return err
		}
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID: sc.TenantID, ActorUserID: &actor, Action: "USER_STATUS_CHANGED", SubjectUserID: &id,
			Before: mustJSON(map[string]any{"status": u.Status}),
			After:  mustJSON(map[string]any{"status": status}),
			Reason: reason,
		})
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not change the user's status.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

type grantRequest struct {
	ProductCode string `json:"product_code"`
}

func (h *Handler) GrantLicense(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	id, ok := pathID(r)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "No such user.")
		return
	}
	var req grantRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.ProductCode = strings.TrimSpace(req.ProductCode)
	if req.ProductCode == "" {
		httpx.Error(w, http.StatusBadRequest, "product_code is required.")
		return
	}
	if _, err := h.store.MembershipForUser(ctx, sc.TenantID, id); err != nil {
		httpx.Error(w, http.StatusNotFound, "No such user.")
		return
	}
	product, err := h.store.ProductByCode(ctx, req.ProductCode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Error(w, http.StatusBadRequest, "Unknown product: "+req.ProductCode+".")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "Could not grant the license.")
		return
	}
	granted, err := h.store.ProductGrantedToTenant(ctx, sc.TenantID, product.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not grant the license.")
		return
	}
	if !granted {
		httpx.Error(w, http.StatusBadRequest, "Tenant does not distribute product "+req.ProductCode+".")
		return
	}
	exists, err := h.store.HasLicense(ctx, sc.TenantID, id, product.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not grant the license.")
		return
	}
	if exists {
		httpx.Error(w, http.StatusConflict, "This user already has a license for "+req.ProductCode+".")
		return
	}
	key, err := license.NewKey(product.KeyPrefix)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not grant the license.")
		return
	}
	keyHash, err := auth.HashSecret(key)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not grant the license.")
		return
	}
	actor := sc.UserID
	var licenseID int64
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		lid, err := h.store.InsertLicense(ctx, tx, &store.License{
			TenantID: sc.TenantID, UserID: id, ProductID: product.ID,
			LicenseKeyHash: keyHash, LicenseKeyHint: license.Hint(key), Status: "ACTIVE",
		})
		if err != nil {
			return err
		}
		licenseID = lid
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID: sc.TenantID, ActorUserID: &actor, Action: "LICENSE_GRANTED", SubjectUserID: &id,
			After:  mustJSON(map[string]any{"product_code": req.ProductCode}),
			Reason: "Granted by admin",
		})
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not grant the license.")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"license_id": licenseID, "license_key": key})
}

// --- §7 licenses -----------------------------------------------------

func (h *Handler) UpdateLicenseStatus(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	id, ok := pathID(r)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "No such license.")
		return
	}
	var req statusRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	reason := strings.TrimSpace(req.Reason)
	if !validLicenseStatuses[status] {
		httpx.Error(w, http.StatusBadRequest, "status must be ACTIVE or DISABLED.")
		return
	}
	if reason == "" {
		httpx.Error(w, http.StatusBadRequest, "A reason is required for a status change.")
		return
	}
	lic, err := h.store.LicenseByID(ctx, sc.TenantID, id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "No such license.")
		return
	}
	actor := sc.UserID
	subject := lic.UserID
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		if err := h.store.UpdateLicenseStatus(ctx, tx, sc.TenantID, id, status); err != nil {
			return err
		}
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID: sc.TenantID, ActorUserID: &actor, Action: "LICENSE_STATUS_CHANGED", SubjectUserID: &subject,
			Before: mustJSON(map[string]any{"status": lic.Status}),
			After:  mustJSON(map[string]any{"status": status}),
			Reason: reason,
		})
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not change the license status.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

func (h *Handler) RegenerateLicenseKey(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	id, ok := pathID(r)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "No such license.")
		return
	}
	lic, err := h.store.LicenseByID(ctx, sc.TenantID, id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "No such license.")
		return
	}
	product, err := h.store.ProductByID(ctx, lic.ProductID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not regenerate the key.")
		return
	}
	key, err := license.NewKey(product.KeyPrefix)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not regenerate the key.")
		return
	}
	keyHash, err := auth.HashSecret(key)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not regenerate the key.")
		return
	}
	actor := sc.UserID
	subject := lic.UserID
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		if err := h.store.UpdateLicenseKey(ctx, tx, sc.TenantID, id, keyHash, license.Hint(key)); err != nil {
			return err
		}
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID: sc.TenantID, ActorUserID: &actor, Action: "LICENSE_REGENERATED", SubjectUserID: &subject,
			After:  mustJSON(map[string]any{"product_code": product.Code}),
			Reason: "Key regenerated by admin",
		})
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not regenerate the key.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"license_id": id, "license_key": key})
}

// --- §7 audit --------------------------------------------------------

func (h *Handler) ListAudit(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var subject *int64
	if v := strings.TrimSpace(r.URL.Query().Get("subject_user_id")); v != "" {
		sid, err := strconv.ParseInt(v, 10, 64)
		if err != nil || sid <= 0 {
			httpx.Error(w, http.StatusBadRequest, "subject_user_id must be a positive integer.")
			return
		}
		subject = &sid
	}
	page, offset := pageOf(r)
	rows, err := h.store.ListAudit(ctx, sc.TenantID, subject, offset, pageSize)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not load the audit trail.")
		return
	}
	total, err := h.store.CountAudit(ctx, sc.TenantID, subject)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not load the audit trail.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"entries": auditViews(rows), "page": page, "page_size": pageSize, "total": total,
	})
}

// --- §7 tenant -------------------------------------------------------

func (h *Handler) GetTenant(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	tenant, err := h.store.TenantByID(r.Context(), sc.TenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not load the workspace.")
		return
	}
	view := tenantViewOf(tenant)
	codes, err := h.store.GrantedProductCodes(r.Context(), sc.TenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not load the workspace.")
		return
	}
	view.Products = codes
	httpx.JSON(w, http.StatusOK, view)
}

type updateTenantRequest struct {
	Name             *string `json:"name"`
	GraceWorkingDays *int    `json:"grace_working_days"`
}

func (h *Handler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var req updateTenantRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tenant, err := h.store.TenantByID(ctx, sc.TenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not load the workspace.")
		return
	}
	name := tenant.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			httpx.Error(w, http.StatusBadRequest, "name cannot be empty.")
			return
		}
	}
	grace := tenant.GraceWorkingDays
	if req.GraceWorkingDays != nil {
		grace = *req.GraceWorkingDays
		if grace < 0 || grace > 30 {
			httpx.Error(w, http.StatusBadRequest, "grace_working_days must be between 0 and 30.")
			return
		}
	}
	actor := sc.UserID
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		if err := h.store.UpdateTenantSettings(ctx, tx, sc.TenantID, name, grace); err != nil {
			return err
		}
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID: sc.TenantID, ActorUserID: &actor, Action: "TENANT_UPDATED",
			Before: mustJSON(map[string]any{"name": tenant.Name, "grace_working_days": tenant.GraceWorkingDays}),
			After:  mustJSON(map[string]any{"name": name, "grace_working_days": grace}),
			Reason: "Workspace settings updated",
		})
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not update the workspace.")
		return
	}
	tenant.Name, tenant.GraceWorkingDays = name, grace
	httpx.JSON(w, http.StatusOK, tenantViewOf(tenant))
}

// --- §7 platform admin: tenants --------------------------------------

func (h *Handler) ListTenants(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	if !sc.IsPlatformAdmin {
		httpx.Error(w, http.StatusForbidden, "Platform admin access required.")
		return
	}
	tenants, err := h.store.ListTenants(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not list tenants.")
		return
	}
	out := make([]tenantView, 0, len(tenants))
	for i := range tenants {
		out = append(out, tenantViewOf(&tenants[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tenants": out})
}

type createTenantAdmin struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
}

type createTenantRequest struct {
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	ContactPerson    string            `json:"contact_person"`
	ContactNo        string            `json:"contact_no"`
	StartDate        string            `json:"start_date"`
	EndDate          string            `json:"end_date"`
	GraceWorkingDays *int              `json:"grace_working_days"`
	Products         []string          `json:"products"`
	Admin            createTenantAdmin `json:"admin"`
}

func (h *Handler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	if !sc.IsPlatformAdmin {
		httpx.Error(w, http.StatusForbidden, "Platform admin access required.")
		return
	}
	ctx := r.Context()
	var req createTenantRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.Name = strings.TrimSpace(req.Name)
	req.Admin.Username = strings.TrimSpace(req.Admin.Username)
	req.Admin.Mobile = strings.TrimSpace(req.Admin.Mobile)

	if !slugRe.MatchString(req.Slug) {
		httpx.Error(w, http.StatusBadRequest, "slug must be 1-64 lowercase letters, digits or hyphens.")
		return
	}
	if req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "name is required.")
		return
	}
	if !usernameRe.MatchString(req.Admin.Username) {
		httpx.Error(w, http.StatusBadRequest, "admin.username must be 3-64 characters: letters, digits, dot, underscore or hyphen.")
		return
	}
	if !e164Re.MatchString(req.Admin.Mobile) {
		httpx.Error(w, http.StatusBadRequest, "admin.mobile must be E.164, e.g. +919876543210")
		return
	}
	if req.Admin.Password != "" && len(req.Admin.Password) < 8 {
		httpx.Error(w, http.StatusBadRequest, "admin.password must be at least 8 characters.")
		return
	}
	if len(req.Products) == 0 {
		httpx.Error(w, http.StatusBadRequest, "At least one product_code in products is required.")
		return
	}
	start, err := time.Parse("2006-01-02", strings.TrimSpace(req.StartDate))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "start_date must be YYYY-MM-DD.")
		return
	}
	var end *time.Time
	if s := strings.TrimSpace(req.EndDate); s != "" {
		e, err := time.Parse("2006-01-02", s)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "end_date must be YYYY-MM-DD.")
			return
		}
		end = &e
	}
	grace := 5
	if req.GraceWorkingDays != nil {
		grace = *req.GraceWorkingDays
		if grace < 0 || grace > 30 {
			httpx.Error(w, http.StatusBadRequest, "grace_working_days must be between 0 and 30.")
			return
		}
	}
	if _, err := h.store.TenantBySlug(ctx, req.Slug); err == nil {
		httpx.Error(w, http.StatusConflict, "That tenant slug is already taken.")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusInternalServerError, "Could not create the tenant.")
		return
	}
	if _, err := h.store.UserByUsername(ctx, req.Admin.Username); err == nil {
		httpx.Error(w, http.StatusConflict, "That username is already taken.")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusInternalServerError, "Could not create the tenant.")
		return
	}

	products := make([]*store.Product, 0, len(req.Products))
	seen := map[int64]bool{}
	for _, code := range req.Products {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		p, err := h.store.ProductByCode(ctx, code)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				httpx.Error(w, http.StatusBadRequest, "Unknown product: "+code+".")
				return
			}
			httpx.Error(w, http.StatusInternalServerError, "Could not create the tenant.")
			return
		}
		if !seen[p.ID] {
			seen[p.ID] = true
			products = append(products, p)
		}
	}
	if len(products) == 0 {
		httpx.Error(w, http.StatusBadRequest, "At least one valid product_code in products is required.")
		return
	}

	generated := req.Admin.Password == ""
	password := req.Admin.Password
	if generated {
		p, err := auth.RandomPassword()
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "Could not create the tenant.")
			return
		}
		password = p
	}
	passwordHash, err := auth.HashSecret(password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not create the tenant.")
		return
	}

	actor := sc.UserID
	var tenantID, adminID int64
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		tid, err := h.store.InsertTenant(ctx, tx, &store.Tenant{
			Slug: req.Slug, Name: req.Name, ContactPerson: req.ContactPerson, ContactNo: req.ContactNo,
			StartDate: start, EndDate: end, GraceWorkingDays: grace, Status: "ACTIVE", IsHouse: false,
		}, actor)
		if err != nil {
			return err
		}
		tenantID = tid
		for _, p := range products {
			if err := h.store.InsertTenantProduct(ctx, tx, tid, p.ID); err != nil {
				return err
			}
		}
		uid, err := h.store.InsertUser(ctx, tx, &store.User{
			Username: req.Admin.Username, PasswordHash: passwordHash,
			FirstName: req.Admin.FirstName, LastName: req.Admin.LastName, ContactNo: req.Admin.Mobile,
			MobileVerified: true, Email: req.Admin.Email, Status: "ACTIVE",
		})
		if err != nil {
			return err
		}
		adminID = uid
		if err := h.store.InsertMembership(ctx, tx, tid, uid, store.RoleAdmin); err != nil {
			return err
		}
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID: tid, ActorUserID: &actor, Action: "TENANT_CREATED", SubjectUserID: &uid,
			After:  mustJSON(map[string]any{"slug": req.Slug, "name": req.Name, "products": req.Products}),
			Reason: "Tenant onboarded by platform admin",
		})
	})
	if err != nil {
		if isDuplicate(err) {
			httpx.Error(w, http.StatusConflict, "That tenant slug or username is already taken.")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "Could not create the tenant.")
		return
	}

	resp := map[string]any{
		"tenant_id": tenantID, "admin_user_id": adminID, "admin_username": req.Admin.Username,
	}
	if generated {
		resp["password"] = password
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateTenantStatus(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	if !sc.IsPlatformAdmin {
		httpx.Error(w, http.StatusForbidden, "Platform admin access required.")
		return
	}
	ctx := r.Context()
	id, ok := pathID(r)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "No such tenant.")
		return
	}
	var req statusRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	reason := strings.TrimSpace(req.Reason)
	if !validTenantStatuses[status] {
		httpx.Error(w, http.StatusBadRequest, "status must be ACTIVE or INACTIVE.")
		return
	}
	if reason == "" {
		httpx.Error(w, http.StatusBadRequest, "A reason is required for a status change.")
		return
	}
	tenant, err := h.store.TenantByID(ctx, id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "No such tenant.")
		return
	}
	actor := sc.UserID
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		if err := h.store.UpdateTenantStatus(ctx, tx, id, status); err != nil {
			return err
		}
		return h.store.InsertAudit(ctx, tx, &store.Audit{
			TenantID: id, ActorUserID: &actor, Action: "TENANT_STATUS_CHANGED",
			Before: mustJSON(map[string]any{"status": tenant.Status}),
			After:  mustJSON(map[string]any{"status": status}),
			Reason: reason,
		})
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not change the tenant status.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}
