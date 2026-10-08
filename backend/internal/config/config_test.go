package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRequiresMySQLDSN(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("MYSQL_DSN", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() with no MYSQL_DSN must fail with an actionable error")
	}
}

func TestLoadReadsDotEnvAndDefaults(t *testing.T) {
	dir := t.TempDir()
	content := "MYSQL_DSN=from-dotenv@tcp(h:3306)/db\nHUB_LISTEN=127.0.0.1:9999\n# a comment\nNO_EQUALS_LINE\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("HUB_LISTEN", "")
	t.Setenv("DASHBOARD_ORIGIN", "")
	t.Setenv("MIGRATIONS_DIR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.MySQLDSN != "from-dotenv@tcp(h:3306)/db" {
		t.Errorf("MySQLDSN = %q, want .env value", cfg.MySQLDSN)
	}
	if cfg.Listen != "127.0.0.1:9999" {
		t.Errorf("Listen = %q, want 127.0.0.1:9999", cfg.Listen)
	}
	if cfg.DashboardOrigin != "http://localhost:5173" {
		t.Errorf("DashboardOrigin = %q, want default", cfg.DashboardOrigin)
	}
	if cfg.MigrationsDir != "" {
		t.Errorf("MigrationsDir = %q, want empty when unset", cfg.MigrationsDir)
	}
}

func TestLoadRealEnvWinsOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MYSQL_DSN=dotenv-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("MYSQL_DSN", "real-env-value")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.MySQLDSN != "real-env-value" {
		t.Errorf("MySQLDSN = %q, want real environment to win over .env", cfg.MySQLDSN)
	}
}
