// Package db is the MySQL pool plus the transaction helper every
// mutation goes through (AGENTS.md rule 6/7: tenant-scoped queries,
// audit row in the same tx).
package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func Open(dsn string) (*sql.DB, error) {
	pool, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	pool.SetMaxOpenConns(20)
	pool.SetMaxIdleConns(10)
	pool.SetConnMaxLifetime(30 * time.Minute)
	return pool, nil
}

// InTx runs fn inside a transaction: commit on nil, rollback otherwise.
func InTx(pool *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := pool.Begin()
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
