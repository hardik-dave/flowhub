module github.com/flowos/hub

go 1.25.11

// Dependencies (SPEC.md §1): github.com/go-chi/chi/v5,
// github.com/go-sql-driver/mysql, github.com/golang-migrate/migrate/v4,
// github.com/alexedwards/argon2id. Go floor raised from 1.22 by the
// dependency graph — see DECISIONS.md.

require (
	github.com/go-chi/chi/v5 v5.3.2
	github.com/go-sql-driver/mysql v1.10.1
	github.com/golang-migrate/migrate/v4 v4.20.1
)

require filippo.io/edwards25519 v1.2.0 // indirect
