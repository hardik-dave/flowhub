// Package config loads hubd's runtime configuration from the
// environment plus an optional .env file. Real environment variables
// always win over .env values; secrets never appear in code (SPEC.md §9).
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Listen          string
	DashboardOrigin string
	MySQLDSN        string
	MigrationsDir   string
	SessionSecret   string
	PlatformAdmin   PlatformAdmin
	Msg91           Msg91
}

type PlatformAdmin struct {
	Username string
	Password string
	Mobile   string
}

type Msg91 struct {
	AuthKey       string
	SenderID      string
	DLTTemplateID string
}

// Load reads .env (cwd, then parent) and builds the config.
func Load() (*Config, error) {
	loadDotEnv(".env")
	loadDotEnv(filepathJoinParent(".env"))

	cfg := &Config{
		Listen:          envOr("HUB_LISTEN", "127.0.0.1:8790"),
		DashboardOrigin: envOr("DASHBOARD_ORIGIN", "http://localhost:5173"),
		MySQLDSN:        os.Getenv("MYSQL_DSN"),
		MigrationsDir:   os.Getenv("MIGRATIONS_DIR"),
		SessionSecret:   os.Getenv("SESSION_SECRET"),
		PlatformAdmin: PlatformAdmin{
			Username: os.Getenv("PLATFORM_ADMIN_USERNAME"),
			Password: os.Getenv("PLATFORM_ADMIN_PASSWORD"),
			Mobile:   os.Getenv("PLATFORM_ADMIN_MOBILE"),
		},
		Msg91: Msg91{
			AuthKey:       os.Getenv("MSG91_AUTH_KEY"),
			SenderID:      os.Getenv("MSG91_SENDER_ID"),
			DLTTemplateID: os.Getenv("MSG91_DLT_TEMPLATE_ID"),
		},
	}
	if cfg.MySQLDSN == "" {
		return nil, fmt.Errorf("MYSQL_DSN is not set — copy .env.example to .env (repo root or backend/) and set it, see README")
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func filepathJoinParent(name string) string {
	return ".." + string(os.PathSeparator) + name
}

// loadDotEnv parses a KEY=VALUE file. Keys already present in the
// environment are not overwritten.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		if k == "" || os.Getenv(k) != "" {
			continue
		}
		os.Setenv(k, v)
	}
}
