package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alexedwards/argon2id"
	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	d, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	pass := "Admin@123!"
	params := &argon2id.Params{Memory: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLength: 16, KeyLength: 32}
	hash, err := argon2id.CreateHash(pass, params)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := d.ExecContext(ctx, "UPDATE users SET password_hash=?, token_version=token_version+1, updated_at=CURRENT_TIMESTAMP WHERE id=7 AND username='platform-admin'", hash); err != nil {
		log.Fatal(err)
	}
	fmt.Println("ok id=7 username=platform-admin password=" + pass)
}
