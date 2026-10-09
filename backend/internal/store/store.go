// Package store is the repository layer. Every method that reads or
// writes tenant data takes an explicit context and is written so the
// caller can scope it by tenant_id (AGENTS.md rule 6). Mutations that
// change status/subscription/license/payment are composed inside a
// single InTx by the caller together with their audit_log row
// (AGENTS.md rule 7).
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a lookup misses. Handlers translate it
// into the §6 contract (usually not_found / 401), never a 500.
var ErrNotFound = errors.New("store: not found")

// DBTX is satisfied by *sql.DB and *sql.Tx so repository methods can be
// composed in a transaction without a second implementation.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store wraps the connection pool.
type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

// InTx runs fn in one transaction: commit on nil, rollback otherwise.
func (s *Store) InTx(ctx context.Context, fn func(DBTX) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// --- rows -----------------------------------------------------------

type Tenant struct {
	ID               int64
	Slug             string
	Name             string
	Status           string
	GraceWorkingDays int
	IsHouse          bool
	ContactPerson    string
	ContactNo        string
	StartDate        time.Time
	EndDate          *time.Time
}

type Product struct {
	ID        int64
	Code      string
	Name      string
	KeyPrefix string
	Status    string
}

// Role is tenant_users.role (SPEC §3).
type Role string

const (
	RoleSuperadmin Role = "SUPERADMIN"
	RoleAdmin      Role = "ADMIN"
	RoleEditor     Role = "EDITOR"
	RoleAlgoUser   Role = "ALGO_USER"
)

// DashboardRole reports whether a role may use the §7 dashboard.
func (r Role) DashboardRole() bool {
	return r == RoleSuperadmin || r == RoleAdmin || r == RoleEditor
}

type Membership struct {
	TenantID int64
	Role     Role
}

type User struct {
	ID               int64
	Username         string
	PasswordHash     string
	FirstName        string
	LastName         string
	ContactNo        string
	MobileVerified   bool
	Email            string
	City             string
	State            string
	BrokerClientCode string
	ReferralCode     string
	Status           string
	TokenVersion     int
	Imported         bool
	ImportBatchID    *int64
}

type License struct {
	ID                     int64
	TenantID               int64
	UserID                 int64
	ProductID              int64
	LicenseKeyHash         string
	LicenseKeyHint         string
	Status                 string
	SubscriptionValidUntil *time.Time
	Entitlements           json.RawMessage
}

type OTPCode struct {
	ID         int64
	UserID     int64
	Purpose    string
	CodeHash   string
	ExpiresAt  time.Time
	Attempts   int
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

type Session struct {
	ID        int64
	UserID    int64
	Audience  string
	TokenHash string
	ExpiresAt time.Time
}

// Audit is one append-only audit_log row. Before/After are raw JSON
// (nil writes SQL NULL); Reason is required for status changes.
type Audit struct {
	TenantID      int64
	ActorUserID   *int64
	Action        string
	SubjectUserID *int64
	Before        json.RawMessage
	After         json.RawMessage
	Reason        string
}

// --- selects --------------------------------------------------------

const tenantCols = "id, slug, name, status, grace_working_days, is_house, contact_person, contact_no, start_date, end_date"

func (s *Store) TenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+tenantCols+" FROM tenants WHERE slug = ?", slug)
	return scanTenant(row)
}

func (s *Store) TenantByID(ctx context.Context, id int64) (*Tenant, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+tenantCols+" FROM tenants WHERE id = ?", id)
	return scanTenant(row)
}

// ListTenants is the platform-admin tenant list (SPEC §7 / DECISIONS.md).
// It is deliberately NOT tenant-scoped — the only cross-tenant read.
func (s *Store) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+tenantCols+" FROM tenants ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func scanTenant(row interface{ Scan(...any) error }) (*Tenant, error) {
	var (
		t                      Tenant
		contactPerson, contact sql.NullString
		end                    sql.NullTime
	)
	if err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.Status, &t.GraceWorkingDays,
		&t.IsHouse, &contactPerson, &contact, &t.StartDate, &end); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	t.ContactPerson, t.ContactNo = contactPerson.String, contact.String
	if end.Valid {
		e := end.Time
		t.EndDate = &e
	}
	return &t, nil
}

const productCols = "id, code, name, key_prefix, status"

func (s *Store) ProductByCode(ctx context.Context, code string) (*Product, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+productCols+" FROM products WHERE code = ?", code)
	var p Product
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.KeyPrefix, &p.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) ProductByID(ctx context.Context, id int64) (*Product, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+productCols+" FROM products WHERE id = ?", id)
	var p Product
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.KeyPrefix, &p.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) ProductGrantedToTenant(ctx context.Context, tenantID, productID int64) (bool, error) {
	row := s.db.QueryRowContext(ctx, "SELECT 1 FROM tenant_products WHERE tenant_id = ? AND product_id = ?", tenantID, productID)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

const userCols = "id, username, password_hash, first_name, last_name, contact_no, mobile_verified, email, city, state, status, token_version"

func (s *Store) UserByUsername(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE username = ?", username)
	return scanUser(row)
}

func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE id = ?", id)
	return scanUser(row)
}

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var (
		u                    User
		first, last, contact sql.NullString
		email, city, state   sql.NullString
	)
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &first, &last, &contact,
		&u.MobileVerified, &email, &city, &state, &u.Status, &u.TokenVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.FirstName, u.LastName, u.ContactNo = first.String, last.String, contact.String
	u.Email, u.City, u.State = email.String, city.String, state.String
	return &u, nil
}

func (s *Store) MembershipForUser(ctx context.Context, tenantID, userID int64) (*Membership, error) {
	row := s.db.QueryRowContext(ctx, "SELECT tenant_id, role FROM tenant_users WHERE tenant_id = ? AND user_id = ?", tenantID, userID)
	return scanMembership(row)
}

// FirstMembership returns any membership for a dashboard login; a user
// is expected to hold exactly one tenant in V1.
func (s *Store) FirstMembership(ctx context.Context, userID int64) (*Membership, error) {
	row := s.db.QueryRowContext(ctx, "SELECT tenant_id, role FROM tenant_users WHERE user_id = ? ORDER BY id LIMIT 1", userID)
	return scanMembership(row)
}

func scanMembership(row interface{ Scan(...any) error }) (*Membership, error) {
	var m Membership
	if err := row.Scan(&m.TenantID, &m.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

const licenseCols = "id, tenant_id, user_id, product_id, license_key_hash, license_key_hint, status, subscription_valid_until, entitlements"

func (s *Store) LicenseByUserProduct(ctx context.Context, tenantID, userID, productID int64) (*License, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+licenseCols+" FROM licenses WHERE tenant_id = ? AND user_id = ? AND product_id = ?",
		tenantID, userID, productID)
	return scanLicense(row)
}

func (s *Store) LicenseByID(ctx context.Context, tenantID, id int64) (*License, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+licenseCols+" FROM licenses WHERE tenant_id = ? AND id = ?", tenantID, id)
	return scanLicense(row)
}

func scanLicense(row interface{ Scan(...any) error }) (*License, error) {
	var (
		l     License
		valid sql.NullTime
		ent   []byte
	)
	if err := row.Scan(&l.ID, &l.TenantID, &l.UserID, &l.ProductID, &l.LicenseKeyHash,
		&l.LicenseKeyHint, &l.Status, &valid, &ent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if valid.Valid {
		t := valid.Time
		l.SubscriptionValidUntil = &t
	}
	l.Entitlements = json.RawMessage(ent)
	return &l, nil
}

// LatestStatusReason returns the reason on the most recent status-change
// audit row for a user, for the §4 "Deactivated by …: {reason}" template.
func (s *Store) LatestStatusReason(ctx context.Context, tenantID, userID int64) (string, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(reason, '') FROM audit_log WHERE tenant_id = ? AND subject_user_id = ? AND reason IS NOT NULL ORDER BY id DESC LIMIT 1",
		tenantID, userID)
	var reason string
	if err := row.Scan(&reason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return reason, nil
}

// LatestOTP returns the newest OTP row for a user/purpose (any state);
// the caller checks consumed/expiry/cooldown.
func (s *Store) LatestOTP(ctx context.Context, userID int64, purpose string) (*OTPCode, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, user_id, purpose, code_hash, expires_at, attempts, consumed_at, created_at "+
			"FROM otp_codes WHERE user_id = ? AND purpose = ? ORDER BY id DESC LIMIT 1", userID, purpose)
	var (
		o        OTPCode
		consumed sql.NullTime
	)
	if err := row.Scan(&o.ID, &o.UserID, &o.Purpose, &o.CodeHash, &o.ExpiresAt, &o.Attempts, &consumed, &o.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if consumed.Valid {
		t := consumed.Time
		o.ConsumedAt = &t
	}
	return &o, nil
}

func (s *Store) SessionByTokenHash(ctx context.Context, hash string) (*Session, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, user_id, audience, token_hash, expires_at FROM sessions WHERE token_hash = ?", hash)
	var sess Session
	if err := row.Scan(&sess.ID, &sess.UserID, &sess.Audience, &sess.TokenHash, &sess.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &sess, nil
}

// --- inserts/updates (tx-composable) --------------------------------

func (s *Store) InsertUser(ctx context.Context, tx DBTX, u *User) (int64, error) {
	res, err := tx.ExecContext(ctx,
		"INSERT INTO users (username, password_hash, first_name, last_name, contact_no, mobile_verified, email, city, state, broker_client_code, referral_code, status, imported, import_batch_id) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		u.Username, u.PasswordHash, nullIfEmpty(u.FirstName), nullIfEmpty(u.LastName), nullIfEmpty(u.ContactNo),
		u.MobileVerified, nullIfEmpty(u.Email), nullIfEmpty(u.City), nullIfEmpty(u.State),
		nullIfEmpty(u.BrokerClientCode), nullIfEmpty(u.ReferralCode), u.Status, u.Imported, u.ImportBatchID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) InsertMembership(ctx context.Context, tx DBTX, tenantID, userID int64, role Role) error {
	_, err := tx.ExecContext(ctx,
		"INSERT INTO tenant_users (tenant_id, user_id, role) VALUES (?, ?, ?)", tenantID, userID, string(role))
	return err
}

func (s *Store) InsertLicense(ctx context.Context, tx DBTX, l *License) (int64, error) {
	res, err := tx.ExecContext(ctx,
		"INSERT INTO licenses (tenant_id, user_id, product_id, license_key_hash, license_key_hint, status, subscription_valid_until, entitlements) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		l.TenantID, l.UserID, l.ProductID, l.LicenseKeyHash, l.LicenseKeyHint, l.Status, l.SubscriptionValidUntil, defaultEntitlements(l.Entitlements))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) InsertOTP(ctx context.Context, tx DBTX, o *OTPCode) (int64, error) {
	res, err := tx.ExecContext(ctx,
		"INSERT INTO otp_codes (user_id, purpose, code_hash, expires_at) VALUES (?, ?, ?, ?)",
		o.UserID, o.Purpose, o.CodeHash, o.ExpiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) IncrementOTPAttempts(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE otp_codes SET attempts = attempts + 1 WHERE id = ?", id)
	return err
}

func (s *Store) ConsumeOTP(ctx context.Context, tx DBTX, id int64, at time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE otp_codes SET consumed_at = ? WHERE id = ?", at, id)
	return err
}

// MarkMobileVerified flips mobile_verified and, when status is the
// pending placeholder, promotes to ACTIVE (§6.3/§6.4).
func (s *Store) MarkMobileVerified(ctx context.Context, tx DBTX, userID int64) error {
	_, err := tx.ExecContext(ctx,
		"UPDATE users SET mobile_verified = TRUE, status = CASE WHEN status = 'PENDING_VERIFICATION' THEN 'ACTIVE' ELSE status END WHERE id = ?",
		userID)
	return err
}

func (s *Store) InsertSession(ctx context.Context, tx DBTX, sess *Session) (int64, error) {
	res, err := tx.ExecContext(ctx,
		"INSERT INTO sessions (user_id, audience, token_hash, expires_at) VALUES (?, ?, ?, ?)",
		sess.UserID, sess.Audience, sess.TokenHash, sess.ExpiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}

func (s *Store) InsertAudit(ctx context.Context, tx DBTX, a *Audit) error {
	_, err := tx.ExecContext(ctx,
		"INSERT INTO audit_log (tenant_id, actor_user_id, action, subject_user_id, `before`, `after`, reason) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)",
		a.TenantID, a.ActorUserID, a.Action, a.SubjectUserID, nullableJSON(a.Before), nullableJSON(a.After), nullIfEmpty(a.Reason))
	return err
}

// --- helpers --------------------------------------------------------

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

// DefaultEntitlements is the seeded license entitlement value
// (SPEC §3). A NULL/empty value cannot be inserted because the column
// is NOT NULL, so inserts fall back to this.
const DefaultEntitlements = `{"max_activations":1,"tier":"RETAIL"}`

func defaultEntitlements(raw json.RawMessage) any {
	if len(raw) == 0 {
		return []byte(DefaultEntitlements)
	}
	return []byte(raw)
}
