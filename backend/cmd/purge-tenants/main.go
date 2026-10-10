// Command purge-tenants is a local-maintenance tool: it deletes every
// tenant except an allowlist (default 1,11,12) together with the rows
// that depend on them, in FK-safe order. It never deletes the house
// tenant (is_house = 1), and it refuses to delete a user that still has
// membership, license, or payment rows elsewhere.
//
// Dry-run by default; pass -apply to actually delete. Pass -backup <path>
// to write a JSON snapshot of everything it is about to remove.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/db"
)

func main() {
	keepFlag := flag.String("keep", "1,11,12", "comma-separated tenant ids to keep")
	apply := flag.Bool("apply", false, "actually delete (default is dry-run)")
	backup := flag.String("backup", "", "optional path to write a JSON backup of affected rows before deleting")
	flag.Parse()

	keep, err := parseKeep(*keepFlag)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	pool, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()

	plan, err := buildPlan(ctx, pool, keep)
	if err != nil {
		log.Fatal(err)
	}
	if len(plan) == 0 {
		fmt.Println("nothing to delete — every tenant is in the keep set or is the house tenant")
		return
	}

	for _, p := range plan {
		fmt.Printf("DELETE tenant %d %q (house=%v)  %s\n", p.ID, p.Slug, p.IsHouse, p.summary())
	}

	if !*apply {
		fmt.Printf("\ndry-run: %d tenant(s) would be deleted. Re-run with -apply to delete.\n", len(plan))
		return
	}

	if *backup != "" {
		if err := writeBackup(ctx, pool, *backup, plan); err != nil {
			log.Fatalf("backup: %v", err)
		}
		fmt.Printf("backup written to %s\n", *backup)
	}

	for _, p := range plan {
		if err := purgeTenant(ctx, pool, p); err != nil {
			log.Fatalf("purge tenant %d: %v", p.ID, err)
		}
		fmt.Printf("deleted tenant %d %q\n", p.ID, p.Slug)
	}
	fmt.Printf("done: %d tenant(s) deleted; kept %v\n", len(plan), sortedKeys(keep))
}

type tenantPlan struct {
	ID      int64
	Slug    string
	IsHouse bool
	Counts  map[string]int
}

func (p tenantPlan) summary() string {
	keys := make([]string, 0, len(p.Counts))
	for k := range p.Counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if p.Counts[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", k, p.Counts[k]))
		}
	}
	if len(parts) == 0 {
		return "(no dependents)"
	}
	return strings.Join(parts, " ")
}

func parseKeep(s string) (map[int64]bool, error) {
	out := map[int64]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid tenant id %q in -keep", part)
		}
		out[id] = true
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-keep must contain at least one id")
	}
	return out, nil
}

func sortedKeys(m map[int64]bool) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// childTables is every table that hangs off a tenant via tenant_id.
var childTables = []string{"tenant_users", "licenses", "payments", "import_batches", "audit_log", "tenant_products"}

func buildPlan(ctx context.Context, pool *sql.DB, keep map[int64]bool) ([]tenantPlan, error) {
	rows, err := pool.QueryContext(ctx, "SELECT id, slug, is_house FROM tenants ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tenants []tenantPlan
	for rows.Next() {
		var p tenantPlan
		if err := rows.Scan(&p.ID, &p.Slug, &p.IsHouse); err != nil {
			return nil, err
		}
		tenants = append(tenants, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var plan []tenantPlan
	for _, p := range tenants {
		if keep[p.ID] || p.IsHouse {
			continue
		}
		p.Counts = map[string]int{}
		for _, tbl := range childTables {
			var n int
			if err := pool.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tbl+" WHERE tenant_id = ?", p.ID).Scan(&n); err != nil {
				return nil, err
			}
			p.Counts[tbl] = n
		}
		plan = append(plan, p)
	}
	return plan, nil
}

func purgeTenant(ctx context.Context, pool *sql.DB, p tenantPlan) error {
	tx, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	userIDs, err := tenantUserIDs(ctx, tx, p.ID)
	if err != nil {
		return err
	}

	// FK-safe order: payments before licenses before users; tenant-scoped
	// children before the tenant row itself.
	for _, q := range []string{
		"DELETE FROM payments WHERE tenant_id = ?",
		"DELETE FROM licenses WHERE tenant_id = ?",
		"DELETE FROM import_batches WHERE tenant_id = ?",
		"DELETE FROM audit_log WHERE tenant_id = ?",
		"DELETE FROM tenant_users WHERE tenant_id = ?",
		"DELETE FROM tenant_products WHERE tenant_id = ?",
	} {
		if _, err := tx.ExecContext(ctx, q, p.ID); err != nil {
			return err
		}
	}

	for _, uid := range userIDs {
		stillReferenced, err := userStillReferenced(ctx, tx, uid)
		if err != nil {
			return err
		}
		if stillReferenced {
			continue // shared with a kept tenant — leave the user row
		}
		for _, q := range []string{
			"DELETE FROM sessions WHERE user_id = ?",
			"DELETE FROM otp_codes WHERE user_id = ?",
			"DELETE FROM users WHERE id = ?",
		} {
			if _, err := tx.ExecContext(ctx, q, uid); err != nil {
				return err
			}
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM tenants WHERE id = ?", p.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func tenantUserIDs(ctx context.Context, tx *sql.Tx, tenantID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT user_id FROM tenant_users WHERE tenant_id = ?", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// userStillReferenced reports whether any row (membership, license, or
// payment in ANY tenant) still points at the user — a guard against
// deleting a user that survives in a kept tenant.
func userStillReferenced(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	for _, q := range []string{
		"SELECT COUNT(*) FROM tenant_users WHERE user_id = ?",
		"SELECT COUNT(*) FROM licenses WHERE user_id = ?",
		"SELECT COUNT(*) FROM payments WHERE user_id = ?",
	} {
		var n int
		if err := tx.QueryRowContext(ctx, q, userID).Scan(&n); err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

func writeBackup(ctx context.Context, pool *sql.DB, path string, plan []tenantPlan) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	root := map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"tenants":      []any{},
	}
	tenantsOut := root["tenants"].([]any)
	for _, p := range plan {
		entry := map[string]any{
			"tenant": dumpRows(ctx, pool, "SELECT * FROM tenants WHERE id = ?", p.ID),
		}
		for _, tbl := range childTables {
			entry[tbl] = dumpRows(ctx, pool, "SELECT * FROM "+tbl+" WHERE tenant_id = ?", p.ID)
		}
		if ids := queryUserIDs(ctx, pool, p.ID); len(ids) > 0 {
			entry["users"] = dumpRows(ctx, pool, "SELECT * FROM users WHERE id IN ("+placeholders(len(ids))+")", int64Args(ids)...)
		}
		tenantsOut = append(tenantsOut, entry)
	}
	root["tenants"] = tenantsOut

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(root)
}

func queryUserIDs(ctx context.Context, pool *sql.DB, tenantID int64) []int64 {
	rows, err := pool.QueryContext(ctx, "SELECT user_id FROM tenant_users WHERE tenant_id = ?", tenantID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func dumpRows(ctx context.Context, pool *sql.DB, query string, args ...any) []map[string]any {
	rows, err := pool.QueryContext(ctx, query, args...)
	if err != nil {
		return []map[string]any{{"error": err.Error()}}
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return []map[string]any{{"error": err.Error()}}
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return out
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				m[c] = string(b)
			} else {
				m[c] = vals[i]
			}
		}
		out = append(out, m)
	}
	return out
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func int64Args(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
