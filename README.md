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
