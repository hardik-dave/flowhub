# FlowOS Hub

> **SCOPE NOTE SUPERSEDED (2026-10-08):** the Aug-2026 six-area scope
> below is not the build target — SPEC.md §1–§10 (with §11's
> out-of-scope list) governs; the referenced docs/wbs/WBS_hub.md does
> not exist in this repo. Kept for history only (DECISIONS.md).
>
> **SCOPE (Aug 2026):** The Hub is a six-area multi-tenant SaaS —
> (1) multi-tenant/multi-product core, (2) licensing & verify, (3)
> subscription & payment admin (admin/editor RBAC), (4) copy-trading
> operations console (visibility only — signals NEVER route through the
> Hub), (5) support ticketing with diagnostics upload, (6) auto-update
> distribution (Phase 2, deferred). Full build handoff: docs/wbs/WBS_hub.md.
> Failure-domain rule: the Hub down costs visibility + new logins, never a
> live trade.


Tenant-scoped licensing, user-management, and subscription backend
for the company's products (FlowOS first; product-dimensional from
day one). **Separate project** — own module, own deploy, no code
shared with the trading platform in either direction. The only
coupling is the wire contract: SPEC.md §6.1.

Start here, in order: `SPEC.md` (the build contract) → `AGENTS.md`
(execution rules) → `DECISIONS.md` (every owner override, read this
before trusting any spec text) → `backend/migrations/` (schema) →
milestones in SPEC.md §10.

Stack (per the 2026-10-08 amendment): Go + chi +
`go-sql-driver/mysql` + golang-migrate + argon2id; MySQL 8; React +
Vite + TS dashboard.

Already real in this scaffold: the working-days grace function WITH
its golden tests (`internal/workingdays`), the license keygen
(`internal/license`), and a stdlib route skeleton where every
endpoint 501s with its spec section — the route list is the API
surface; don't add to it.

Week-one owner action that no builder can do for you: TRAI DLT
registration for SMS OTP (SPEC.md §6.4). Start it before the code.

Dev quickstart: copy `.env.example` → `.env` (set `MYSQL_DSN`) →
create the database (`CREATE DATABASE flowhub`) → `go run ./cmd/hubd`
(applies migrations at startup). Alternative for other machines:
`docker compose up -d db` (MySQL 8 on host port 3307).

## Runbook

- Build / test: `cd backend && go vet ./... && go test ./... -count=1`.
  DB-backed tests run when `HUB_TEST_DATABASE_URL` is set.
- Launch: run `hubd` with CWD = `backend/` so `migrations/` resolves;
  it applies migrations, then bootstraps the platform admin from
  `PLATFORM_ADMIN_*` (idempotent) and listens on `HUB_LISTEN`.
- Deploy: `deploy/Caddyfile.example` (TLS + reverse proxy),
  `deploy/hubd.service.example` (systemd unit, `EnvironmentFile` =
  `.env`).
- Backups: `deploy/backup-mysql.sh.example` does a nightly
  `mysqldump` (gzip, 14-day retention); `deploy/backup.cron.example`
  schedules it. Restore with
  `gunzip -c flowhub_<stamp>.sql.gz | mysql -u root -p flowhub`.
- Secrets: `.env` only, never committed; OTP codes and license keys
  are never logged.

## Dashboard (React + Vite)

Dev: `cd dashboard && npm i && npm run dev` → http://localhost:5173.
Build: `cd dashboard && npm run build && npm run test`.
API proxy: `/api` → `http://127.0.0.1:8790` (vite.config.ts).
Docker: `docker compose up -d db dashboard hubd` (hubd builds from ./backend).
