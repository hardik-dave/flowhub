package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDirExplicit(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sql")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveDir(sub)
	if err != nil || got != sub {
		t.Fatalf("resolveDir(%q) = %q, %v; want the directory itself", sub, got, err)
	}
}

func TestResolveDirProbesDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "migrations"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	got, err := resolveDir("")
	if err != nil || got != "migrations" {
		t.Fatalf("resolveDir(\"\") = %q, %v; want migrations", got, err)
	}
}

func TestResolveDirMissingIsActionable(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := resolveDir("nope"); err == nil {
		t.Fatal("missing directory must return an error")
	} else if !strings.Contains(err.Error(), "MIGRATIONS_DIR") {
		t.Errorf("error must point at MIGRATIONS_DIR, got: %v", err)
	}
}

func TestFileURLShape(t *testing.T) {
	u := fileURL(t.TempDir())
	if !strings.HasPrefix(u, "file://") {
		t.Fatalf("fileURL() = %q, want file:// prefix", u)
	}
}
