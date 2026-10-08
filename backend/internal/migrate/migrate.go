// Package migrate applies backend/migrations at boot with
// golang-migrate (SPEC.md §1). Migrations load from disk via
// MIGRATIONS_DIR (DECISIONS.md — embed cannot reach a sibling
// directory and deploys ship the .sql files).
package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gomigrate "github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	sqlmysql "github.com/go-sql-driver/mysql"
)

// Up applies every pending migration; nil when already current.
// dir may be empty — the usual locations are probed.
func Up(dir, dsn string) error {
	path, err := resolveDir(dir)
	if err != nil {
		return err
	}

	cfg, err := sqlmysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("parse MYSQL_DSN: %w", err)
	}
	// golang-migrate's mysql driver executes whole files; the app
	// pool keeps the plain DSN (no multiStatements).
	cfg.MultiStatements = true

	pool, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(); err != nil {
		return fmt.Errorf("cannot reach MySQL at %s (%v) — is the server running? MYSQL_DSN lives in .env, see README", cfg.Addr, err)
	}

	drv, err := migmysql.WithInstance(pool, &migmysql.Config{})
	if err != nil {
		return fmt.Errorf("mysql migrate driver: %w", err)
	}

	m, err := gomigrate.NewWithDatabaseInstance(fileURL(path), "mysql", drv)
	if err != nil {
		return fmt.Errorf("load migrations from %s: %w", path, err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, gomigrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func resolveDir(dir string) (string, error) {
	candidates := []string{dir}
	if dir == "" {
		candidates = []string{"migrations", filepath.Join("backend", "migrations")}
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("migrations directory not found (tried: %s) — set MIGRATIONS_DIR in .env", strings.Join(candidates, ", "))
}

// fileURL builds a file:// URL golang-migrate parses correctly on
// both platforms: file:///abs/path on Unix, file://C:/abs/path on
// Windows (drive letter parses as host, rejoining to C:/abs/path).
func fileURL(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return "file://" + filepath.ToSlash(abs)
}
