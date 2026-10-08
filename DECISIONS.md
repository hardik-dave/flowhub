# DECISIONS.md — choices made where SPEC.md was silent

Format: date · decision · one line of why. The builder appends here
(AGENTS.md rule 13); reviewers read this file first.

- 2026-07-31 · Monorepo placement: hub/ lives inside fintech-platform
  for review convenience; it is a separate deployable with its own
  go.mod and NO code imports in either direction (ADR-016).
- 2026-07-31 · Stack pinned by spec §1 (chi/pgx/migrate/argon2id);
  scaffold ships stdlib-only so it compiles with zero deps.
- 2026-07-31 · Dev Postgres on host port 5433 to avoid colliding
  with any local 5432.
- 2026-10-08 · DB engine: **MySQL 8.0** replaces SPEC §1's PostgreSQL 16
  (owner decision, CTO-confirmed) — `go-sql-driver/mysql` + `database/sql`
  instead of pgx; golang-migrate unchanged (mysql driver). Supersedes the
  2026-07-31 Postgres entries above.
- 2026-10-08 · Data model: the owner's `db_schemas.txt` (PashuTrack
  pattern: BIGINT auto-increment PKs, tenant-agnostic `users` +
  `tenant_users` join, roles SUPERADMIN/ADMIN/EDITOR) is authoritative
  for identity — reverses the original SPEC §3 and MULTITENANT_PLAN §2
  deliberate differences; owner decision.
- 2026-10-08 · `users.status` = PENDING_VERIFICATION/ACTIVE/DEACTIVATED/
  BANNED, not db_schemas.txt's ACTIVE/DISABLED — §4 must split `paused`
  (DEACTIVATED) from `revoked` (BANNED) for the shipped desktop client
  (AGENTS.md rule 1); additive, nothing else in the model moves.
- 2026-10-08 · `grace_period_days` → `grace_working_days INT DEFAULT 5`
  (Mon–Fri) — §4's `add_working_days` is spec behaviour with mandatory
  golden tests; calendar days would make every grace test wrong.
- 2026-10-08 · tenant `status` vocabulary is ACTIVE/INACTIVE (owner's
  model); §4's mapping-table condition "tenant SUSPENDED" reads as
  "tenant INACTIVE" — response strings unchanged, condition renamed to
  the value that exists.
- 2026-10-08 · roles: SUPERADMIN added to the owner's enum as
  SUPERADMIN/ADMIN/EDITOR/ALGO_USER — the join model needs a role for
  end users and §6.2 returns `role`; SUPERADMIN in the house tenant =
  platform admin; EDITOR exists per owner's schema but ships no V1
  logic (SPEC §11 keeps its ban on EDITOR features).
- 2026-10-08 · `tenants.slug` added (absent from db_schemas.txt) —
  `tenant_slug` appears in every §6 request; `tenants.is_house` kept
  from SPEC §2.3 (house tenant flag beats assuming id=1 once the DB
  may already contain rows).
- 2026-10-08 · `users.contact_no` is the E.164 mobile column; API field
  stays `mobile` per §6.3 — column named as the owner's schema, wire
  contract named as the spec.
- 2026-10-08 · `users.mobile_verified`/`imported`/`import_batch_id` and
  the §6.3 PII columns (email, city, state, broker_client_code,
  referral_code) added to db_schemas.txt's users — §4.3 and §6.3/§8
  cannot function without them.
- 2026-10-08 · `username` stays globally UNIQUE (owner's model) despite
  per-tenant semantics — a join model can't scope uniqueness without
  denormalizing; risk flagged to owner: two brokers can't both hold
  `trader01`.
- 2026-10-08 · no UNIQUE on `contact_no` — the same person may be a
  client of two tenants; per-tenant mobile dedup happens app-side in
  register/import (spec's `(tenant_id, mobile)` rule lives in code).
- 2026-10-08 · `licenses` unique on `(tenant_id, user_id, product_id)`
  instead of `(user_id, product_id)` — the join model lets one user
  belong to two tenants; the tenant pin keeps every license row
  unambiguous and tenant-scoped (AGENTS.md rule 6).
- 2026-10-08 · `created_by`/`updated_by` on tenants/users are plain
  BIGINT columns with no FK — avoids circular DDL ordering
  (tenants↔users); audit_log is the authoritative who-did-it record.
- 2026-10-08 · `token_version` (owner's column) is the session
  invalidation lever — bump it to revoke all of a user's sessions;
  tokens themselves are still random 256-bit sha256 per §3.
- 2026-10-08 · DATETIME replaces TIMESTAMPTZ (MySQL has no tz-aware
  type); DSN uses `parseTime=true&loc=UTC` so Go round-trips UTC;
  TEXT columns carrying UNIQUE constraints become VARCHAR(n) —
  MySQL index limits, no semantic change.
- 2026-10-08 · `GET /dash/tenants` added beyond SPEC §7's bullet list —
  §7's SPA page list requires a platform-admin tenant page; additive
  route only (AGENTS.md rule 3 satisfied: it appears in MULTITENANT_PLAN).
- 2026-10-08 · Dev DB = native MySQL 8.0.46 on 127.0.0.1:3306 (service
  MySQL80), database `flowhub` — docker compose unsatisfiable on this
  machine (no WSL, no hypervisor); compose file kept for CI/other
  machines with host port 3307 to avoid colliding with 3306.
- 2026-10-08 · README's six-area scope note (copy-trading console,
  ticketing, admin/editor RBAC, WBS_hub.md) ignored — owner decision;
  SPEC §11 governs.
- 2026-10-08 · No per-change-set commits at owner request (repo exists
  but changes stay in the working tree) — AGENTS.md rule 9/12 "same
  commit" is enforced at change-set level instead.
- 2026-10-08 · §6.2: credentials valid but any non-`valid` verdict
  (including `not_found`) → 403 carrying the §6.1 reason — simplest
  reading of "the §6.1 paused/revoked reason" once a logged-in user
  necessarily exists.
- 2026-10-08 · PENDING_VERIFICATION and NULL `subscription_valid_until`
  → `paused` with "Account awaiting activation or payment. Contact
  {tenant.name}."; a dated expiry past grace uses the "Subscription
  expired on {date} — renew to continue." template (§4 gives one row
  two templates without saying which fires when).
- 2026-10-08 · Grace remaining = Mon–Fri days strictly after `today`
  through `grace.until`; `today == grace.until` is still `valid` with
  "0 working day(s) of grace remain" — §4's check is `today <=` grace
  end (§6.1 example numbers are illustrative, not grace=5-consistent).
- 2026-10-08 · `not_found` still emits
  `subscription: {valid_until: null, grace: {active: false, until: null}}`
  — never remove a response field (AGENTS.md rule 1).
- 2026-10-08 · Dashboard auth = bearer token in localStorage (§7 allows
  cookie or bearer; bearer avoids the CSRF dance); CORS locked to a
  `DASHBOARD_ORIGIN` env var.
- 2026-10-08 · Migrations load from disk via `MIGRATIONS_DIR`, not
  `go:embed` — embed cannot reach a sibling directory and deploys ship
  the .sql files anyway.
- 2026-10-08 · Rate limit: 10/min per IP on login+verify (§9); §5's
  per-user wording folded into the same per-IP limit as the simplest
  reading.
- 2026-10-08 · Product `RETIRED` → `not_found` (same §4 row as
  "product not granted to tenant"); §4 has no retired-product row and
  the product is simply unavailable for use.
- 2026-10-08 · §4 evaluation order: identification failures first
  (wrong key never discloses an account exists), then revoked causes
  (tenant INACTIVE / user BANNED / license DISABLED), then paused
  causes (DEACTIVATED / PENDING / unverified / never paid / expired),
  then subscription. Revoked beats paused when both hold.
- 2026-10-08 · `mobile_verified = false` → `paused` + "Account
  awaiting activation or payment." — §4 condition 3 has no mapping
  table row; unverified means not activated, same as PENDING.
- 2026-10-08 · `grace.until` is null whenever `grace.active` is false
  (§6.1 only shows `until` in the in-grace example).
- 2026-10-08 · Dates in reason/warning strings render as `02 Jan 2006`
  (zero-padded; the §6.1 example "12 Sep 2026" leaves single digits
  ambiguous).
- 2026-10-08 · §4 rule lives in `internal/verify.Evaluate` as a pure
  function (AccessInput in, Verdict out) — DB fetch stays in the
  service layer; the M-B suite runs without MySQL.
- 2026-10-08 · The §6.1 example body is asserted verbatim by giving
  the test fixture `grace_working_days = 4` (default 5 has its own
  case) — proves exact template strings end to end.
- 2026-10-08 · go directive 1.22 → 1.25.11 (floor pulled up by the
  golang-migrate v4.20.1 / x-package graph); CI reads the version
  from backend/go.mod via go-version-file.
- 2026-10-08 · hubd loads .env itself (./.env then ../.env, plain
  KEY=VALUE, real environment always wins) — no godotenv dependency;
  CWD is always backend/ because go run needs the module root.
- 2026-10-08 · MIGRATIONS_DIR unset → probe `migrations` then
  `backend/migrations`; explicit value is used as-is.
- 2026-10-08 · Boot order config → pool → migrations → router, and it
  is fail-fast: unreachable MySQL or a missing directory is a fatal
  error with an actionable message (systemd restarts handle the race).
- 2026-10-08 · golang-migrate gets a DSN *copy* with
  multiStatements=true (its mysql driver requires it); the app pool
  keeps the plain DSN.
- 2026-10-08 · file:// source URL = "file://" + ToSlash(abs): on
  Windows the drive letter parses as host and rejoins to C:/… (matches
  golang-migrate's file.parseURL; the file:///C:/ form does NOT).
- 2026-10-08 · Client IP = first X-Forwarded-For hop, else X-Real-IP,
  else socket host — hubd sits behind Caddy (§9), so direct exposure
  of the port must be firewalled.
- 2026-10-08 · 10/min per-IP limit mounted on POST /app/verify,
  /app/login, /dash/login (§9's "login & verify"); the OTP endpoints
  get their own §6.4 cooldowns when auth lands.
- 2026-10-08 · Route surface = 22 business endpoints (5 §6 + 16 §7 +
  GET /dash/tenants) + /healthz, locked by TestRouteSurface; the
  scaffold had been missing POST /dash/tenants and PATCH
  /dash/tenants/{id}/status which SPEC §7 line 378 does list.
- 2026-10-08 · argon2id dependency fetched with the rest; password
  hashing code arrives with M-A (CS-3), not before.
