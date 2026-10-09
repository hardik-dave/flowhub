package migrate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// DB-gated (AGENTS.md rule 9): runs in CI where HUB_TEST_DATABASE_URL
// points at a throwaway MySQL (see .github/workflows/ci.yml); skipped
// locally when unset.
func TestUpAppliesSchemaAndSeeds(t *testing.T) {
	dsn := os.Getenv("HUB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("HUB_TEST_DATABASE_URL not set — skipping DB-backed migration test")
	}
	dir := filepath.Join("..", "..", "migrations")

	if err := Up(dir, dsn); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if err := Up(dir, dsn); err != nil {
		t.Fatalf("Up() must be idempotent (ErrNoChange), got: %v", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got[name] = true
	}
	rows.Close()

	for _, want := range []string{
		"tenants", "products", "tenant_products", "users", "tenant_users",
		"licenses", "payments", "otp_codes", "sessions", "audit_log",
		"import_batches", "schema_migrations",
	} {
		if !got[want] {
			t.Errorf("table missing after Up(): %s", want)
		}
	}

	var house int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM tenants WHERE id = 1 AND slug = 'flowos' AND is_house = TRUE",
	).Scan(&house); err != nil {
		t.Fatal(err)
	}
	if house != 1 {
		t.Errorf("house tenant seed missing (id=1, slug=flowos, is_house=TRUE): count=%d", house)
	}

	var product int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM products WHERE code = 'flowos' AND key_prefix = 'FL'",
	).Scan(&product); err != nil {
		t.Fatal(err)
	}
	if product != 1 {
		t.Errorf("product seed missing (flowos/FL): count=%d", product)
	}

	var grant int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tenant_products tp
		JOIN tenants t ON t.id = tp.tenant_id
		JOIN products p ON p.id = tp.product_id
		WHERE t.slug = 'flowos' AND p.code = 'flowos'`).Scan(&grant); err != nil {
		t.Fatal(err)
	}
	if grant != 1 {
		t.Errorf("house tenant must be granted flowos: count=%d", grant)
	}
}
