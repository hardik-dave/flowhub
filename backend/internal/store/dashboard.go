package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// This file holds the §7 dashboard repository surface. Every query
// except the documented platform-admin tenant paths is tenant-scoped
// (AGENTS.md rule 6).

// UserFilter narrows GET /dash/users. TenantID is used only by the
// platform-admin cross-tenant list (GET /dash/all-users): 0 means "any
// tenant", otherwise it pins one.
type UserFilter struct {
	Status      string
	ProductCode string
	Query       string
	TenantID    int64
	Offset      int
	Limit       int
}

const userColsAliased = "u.id, u.username, u.password_hash, u.first_name, u.last_name, u.contact_no, u.mobile_verified, u.email, u.city, u.state, u.status, u.token_version"

func userWhere(f UserFilter) (string, []any) {
	var b strings.Builder
	var args []any
	if f.Status != "" {
		b.WriteString(" AND u.status = ?")
		args = append(args, f.Status)
	}
	if f.Query != "" {
		like := "%" + f.Query + "%"
		b.WriteString(" AND (u.username LIKE ? OR u.first_name LIKE ? OR u.last_name LIKE ? OR u.contact_no LIKE ?)")
		args = append(args, like, like, like, like)
	}
	return b.String(), args
}

func (s *Store) ListUsers(ctx context.Context, tenantID int64, f UserFilter) ([]User, error) {
	q := "SELECT DISTINCT " + userColsAliased + " FROM users u" +
		" JOIN tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = ?"
	args := []any{tenantID}
	if f.ProductCode != "" {
		q += " JOIN licenses l ON l.user_id = u.id AND l.tenant_id = tu.tenant_id" +
			" JOIN products p ON p.id = l.product_id AND p.code = ?"
		args = append(args, f.ProductCode)
	}
	where, wargs := userWhere(f)
	q += " WHERE 1=1" + where + " ORDER BY u.id DESC LIMIT ? OFFSET ?"
	args = append(args, wargs...)
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (s *Store) CountUsers(ctx context.Context, tenantID int64, f UserFilter) (int, error) {
	q := "SELECT COUNT(DISTINCT u.id) FROM users u" +
		" JOIN tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = ?"
	args := []any{tenantID}
	if f.ProductCode != "" {
		q += " JOIN licenses l ON l.user_id = u.id AND l.tenant_id = tu.tenant_id" +
			" JOIN products p ON p.id = l.product_id AND p.code = ?"
		args = append(args, f.ProductCode)
	}
	where, wargs := userWhere(f)
	q += " WHERE 1=1" + where
	args = append(args, wargs...)

	var n int
	if err := s.db.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// UserWithTenant is a user joined to one of their tenant memberships.
// It backs the platform-admin cross-tenant list (GET /dash/all-users);
// every row is a distinct (tenant, user) pair.
type UserWithTenant struct {
	User
	TenantID   int64
	TenantSlug string
	TenantName string
	Role       Role
}

// allUsersFrom builds the shared FROM/WHERE clause for the cross-tenant
// user list. The caller supplies the SELECT and LIMIT/OFFSET.
func allUsersFrom(f UserFilter) (string, []any) {
	q := " FROM users u" +
		" JOIN tenant_users tu ON tu.user_id = u.id" +
		" JOIN tenants t ON t.id = tu.tenant_id"
	var args []any
	if f.ProductCode != "" {
		q += " JOIN licenses l ON l.user_id = u.id AND l.tenant_id = tu.tenant_id" +
			" JOIN products p ON p.id = l.product_id AND p.code = ?"
		args = append(args, f.ProductCode)
	}
	where, wargs := userWhere(f)
	q += " WHERE 1=1"
	if f.TenantID != 0 {
		q += " AND tu.tenant_id = ?"
		args = append(args, f.TenantID)
	}
	q += where
	args = append(args, wargs...)
	return q, args
}

func (s *Store) ListAllUsers(ctx context.Context, f UserFilter) ([]UserWithTenant, error) {
	from, args := allUsersFrom(f)
	q := "SELECT " + userColsAliased + ", tu.tenant_id, t.slug, t.name, tu.role" + from +
		" ORDER BY u.id DESC, tu.tenant_id ASC LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserWithTenant
	for rows.Next() {
		uw, err := scanUserWithTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *uw)
	}
	return out, rows.Err()
}

func (s *Store) CountAllUsers(ctx context.Context, f UserFilter) (int, error) {
	from, args := allUsersFrom(f)
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func scanUserWithTenant(row interface{ Scan(...any) error }) (*UserWithTenant, error) {
	var (
		uw                   UserWithTenant
		first, last, contact sql.NullString
		email, city, state   sql.NullString
		role                 string
	)
	if err := row.Scan(&uw.ID, &uw.Username, &uw.PasswordHash, &first, &last, &contact,
		&uw.MobileVerified, &email, &city, &state, &uw.Status, &uw.TokenVersion,
		&uw.TenantID, &uw.TenantSlug, &uw.TenantName, &role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	uw.FirstName, uw.LastName, uw.ContactNo = first.String, last.String, contact.String
	uw.Email, uw.City, uw.State = email.String, city.String, state.String
	uw.Role = Role(role)
	return &uw, nil
}

// UserLicense is a license joined with its product, for detail/list views.
type UserLicense struct {
	ID                     int64
	ProductID              int64
	ProductCode            string
	ProductName            string
	Status                 string
	KeyHint                string
	SubscriptionValidUntil *time.Time
	Entitlements           json.RawMessage
}

func (s *Store) ListUserLicenses(ctx context.Context, tenantID, userID int64) ([]UserLicense, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT l.id, l.product_id, p.code, p.name, l.status, l.license_key_hint, l.subscription_valid_until, l.entitlements "+
			"FROM licenses l JOIN products p ON p.id = l.product_id "+
			"WHERE l.tenant_id = ? AND l.user_id = ? ORDER BY l.id", tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserLicense
	for rows.Next() {
		var (
			ul    UserLicense
			valid sql.NullTime
			ent   []byte
		)
		if err := rows.Scan(&ul.ID, &ul.ProductID, &ul.ProductCode, &ul.ProductName, &ul.Status, &ul.KeyHint, &valid, &ent); err != nil {
			return nil, err
		}
		if valid.Valid {
			t := valid.Time
			ul.SubscriptionValidUntil = &t
		}
		ul.Entitlements = json.RawMessage(ent)
		out = append(out, ul)
	}
	return out, rows.Err()
}

// HasLicense reports whether (tenant, user, product) already holds a license.
func (s *Store) HasLicense(ctx context.Context, tenantID, userID, productID int64) (bool, error) {
	row := s.db.QueryRowContext(ctx, "SELECT 1 FROM licenses WHERE tenant_id = ? AND user_id = ? AND product_id = ?", tenantID, userID, productID)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Store) UpdateUserStatus(ctx context.Context, tx DBTX, tenantID, userID int64, status string, bumpTokenVersion bool) error {
	q := "UPDATE users SET status = ?"
	if bumpTokenVersion {
		q += ", token_version = token_version + 1"
	}
	q += " WHERE id = ? AND id IN (SELECT user_id FROM tenant_users WHERE tenant_id = ?)"
	_, err := tx.ExecContext(ctx, q, status, userID, tenantID)
	return err
}

func (s *Store) UpdateLicenseStatus(ctx context.Context, tx DBTX, tenantID, licenseID int64, status string) error {
	_, err := tx.ExecContext(ctx, "UPDATE licenses SET status = ? WHERE id = ? AND tenant_id = ?", status, licenseID, tenantID)
	return err
}

func (s *Store) UpdateLicenseKey(ctx context.Context, tx DBTX, tenantID, licenseID int64, hash, hint string) error {
	_, err := tx.ExecContext(ctx, "UPDATE licenses SET license_key_hash = ?, license_key_hint = ? WHERE id = ? AND tenant_id = ?", hash, hint, licenseID, tenantID)
	return err
}

// Payment is a payments row for the user-detail history.
type Payment struct {
	ID               int64
	TenantID         int64
	UserID           int64
	LicenseID        int64
	AmountMinorUnits int64
	Currency         string
	Method           string
	GatewayPaymentID string
	Plan             string
	PaidAt           time.Time
	ValidFrom        time.Time
	ValidUntil       time.Time
	RecordedBy       int64
	Note             string
	CreatedAt        time.Time
}

func (s *Store) ListPaymentsByUser(ctx context.Context, tenantID, userID int64) ([]Payment, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, tenant_id, user_id, license_id, amount_minor_units, currency, method, gateway_payment_id, plan, paid_at, valid_from, valid_until, recorded_by, note, created_at "+
			"FROM payments WHERE tenant_id = ? AND user_id = ? ORDER BY id DESC", tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Payment
	for rows.Next() {
		var (
			p             Payment
			gateway, note sql.NullString
		)
		if err := rows.Scan(&p.ID, &p.TenantID, &p.UserID, &p.LicenseID, &p.AmountMinorUnits, &p.Currency,
			&p.Method, &gateway, &p.Plan, &p.PaidAt, &p.ValidFrom, &p.ValidUntil, &p.RecordedBy, &note, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.GatewayPaymentID, p.Note = gateway.String, note.String
		out = append(out, p)
	}
	return out, rows.Err()
}

// AuditRow is one audit_log row for GET /dash/audit.
type AuditRow struct {
	ID            int64
	TenantID      int64
	ActorUserID   *int64
	Action        string
	SubjectUserID *int64
	Before        json.RawMessage
	After         json.RawMessage
	Reason        string
	At            time.Time
}

func (s *Store) ListAudit(ctx context.Context, tenantID int64, subjectUserID *int64, offset, limit int) ([]AuditRow, error) {
	q := "SELECT id, tenant_id, actor_user_id, action, subject_user_id, `before`, `after`, reason, at " +
		"FROM audit_log WHERE tenant_id = ?"
	args := []any{tenantID}
	if subjectUserID != nil {
		q += " AND subject_user_id = ?"
		args = append(args, *subjectUserID)
	}
	q += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var (
			a       AuditRow
			actor   sql.NullInt64
			subject sql.NullInt64
			before  []byte
			after   []byte
			reason  sql.NullString
		)
		if err := rows.Scan(&a.ID, &a.TenantID, &actor, &a.Action, &subject, &before, &after, &reason, &a.At); err != nil {
			return nil, err
		}
		if actor.Valid {
			v := actor.Int64
			a.ActorUserID = &v
		}
		if subject.Valid {
			v := subject.Int64
			a.SubjectUserID = &v
		}
		a.Before = json.RawMessage(before)
		a.After = json.RawMessage(after)
		a.Reason = reason.String
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CountAudit(ctx context.Context, tenantID int64, subjectUserID *int64) (int, error) {
	q := "SELECT COUNT(*) FROM audit_log WHERE tenant_id = ?"
	args := []any{tenantID}
	if subjectUserID != nil {
		q += " AND subject_user_id = ?"
		args = append(args, *subjectUserID)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) InsertTenant(ctx context.Context, tx DBTX, t *Tenant, createdBy int64) (int64, error) {
	res, err := tx.ExecContext(ctx,
		"INSERT INTO tenants (slug, name, contact_person, contact_no, start_date, end_date, grace_working_days, status, is_house, created_by) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		t.Slug, t.Name, nullIfEmpty(t.ContactPerson), nullIfEmpty(t.ContactNo), t.StartDate, t.EndDate,
		t.GraceWorkingDays, t.Status, t.IsHouse, createdBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) InsertTenantProduct(ctx context.Context, tx DBTX, tenantID, productID int64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO tenant_products (tenant_id, product_id) VALUES (?, ?)", tenantID, productID)
	return err
}

func (s *Store) UpdateTenantStatus(ctx context.Context, tx DBTX, id int64, status string) error {
	_, err := tx.ExecContext(ctx, "UPDATE tenants SET status = ? WHERE id = ?", status, id)
	return err
}

func (s *Store) UpdateTenantSettings(ctx context.Context, tx DBTX, id int64, name string, graceWorkingDays int) error {
	_, err := tx.ExecContext(ctx, "UPDATE tenants SET name = ?, grace_working_days = ? WHERE id = ?", name, graceWorkingDays, id)
	return err
}

// --- payments + import (SPEC §7/§8) ---------------------------------

func (s *Store) InsertPayment(ctx context.Context, tx DBTX, p *Payment) (int64, error) {
	res, err := tx.ExecContext(ctx,
		"INSERT INTO payments (tenant_id, user_id, license_id, amount_minor_units, currency, method, gateway_payment_id, plan, paid_at, valid_from, valid_until, recorded_by, note) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		p.TenantID, p.UserID, p.LicenseID, p.AmountMinorUnits, p.Currency, p.Method,
		nullIfEmpty(p.GatewayPaymentID), p.Plan, p.PaidAt, p.ValidFrom, p.ValidUntil, p.RecordedBy, nullIfEmpty(p.Note))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateLicenseValidUntil(ctx context.Context, tx DBTX, tenantID, licenseID int64, until time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE licenses SET subscription_valid_until = ? WHERE id = ? AND tenant_id = ?", until, licenseID, tenantID)
	return err
}

// TenantHasMobile reports whether any user already holds this contact_no
// in the tenant (SPEC §8 dedup on (tenant_id, mobile)).
func (s *Store) TenantHasMobile(ctx context.Context, tenantID int64, mobile string) (bool, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT 1 FROM users u JOIN tenant_users tu ON tu.user_id = u.id WHERE tu.tenant_id = ? AND u.contact_no = ? LIMIT 1",
		tenantID, mobile)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ImportBatch is one import_batches row (SPEC §8).
type ImportBatch struct {
	ID         int64
	TenantID   int64
	Filename   string
	RowCount   int
	OKCount    int
	ErrorCount int
	Errors     json.RawMessage
	CreatedBy  int64
}

func (s *Store) InsertImportBatch(ctx context.Context, tx DBTX, b *ImportBatch) (int64, error) {
	res, err := tx.ExecContext(ctx,
		"INSERT INTO import_batches (tenant_id, filename, row_count, ok_count, error_count, errors, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)",
		b.TenantID, b.Filename, b.RowCount, b.OKCount, b.ErrorCount, nullableJSON(b.Errors), b.CreatedBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateImportBatch(ctx context.Context, tx DBTX, id int64, okCount, errorCount int, errs json.RawMessage) error {
	_, err := tx.ExecContext(ctx,
		"UPDATE import_batches SET ok_count = ?, error_count = ?, errors = ? WHERE id = ?",
		okCount, errorCount, nullableJSON(errs), id)
	return err
}
