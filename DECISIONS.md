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
- 2026-10-08 · MySQL DDL fixes found on first apply (8.0.46):
  (a) TEXT columns cannot have a DEFAULT, so the four `status` columns
  (tenants, products, users, licenses) are VARCHAR(20) in SPEC §3 and
  the migration; (b) column-level `REFERENCES` is parsed-but-ignored
  by MySQL, so the migration emits explicit table-level FOREIGN KEY
  constraints (13 total) — audit_log.tenant_id, created_by/updated_by
  and import_batch_id stay FK-free by design; (c) `before`/`after` are
  reserved words and are backticked; (d) JSON `entitlements` DEFAULT
  (CAST(... AS JSON)) is valid on 8.0.13+ and verified.
- 2026-10-08 · 0001_init applied to the local `flowhub` DB:
  schema_migrations version=1 dirty=0, 11 tables + schema_migrations,
  house tenant id 1 (slug flowos, is_house), product flowos (FL)
  granted in tenant_products; 13 FKs, 11 CHECK constraints.
- 2026-10-08 · DB-gated migration integration test (skips without
  HUB_TEST_DATABASE_URL, runs in CI) asserts Up() applies cleanly,
  is idempotent (ErrNoChange) and seeds house tenant + product + grant.
- 2026-10-08 · M-A (CS-3): one `internal/store` repository layer
  wrapping `*sql.DB`; a `DBTX` interface (`*sql.DB`/`*sql.Tx`) lets the
  same methods compose inside `InTx`. Every tenant-data query takes
  tenant_id; `InsertAudit` is the single writer of audit_log and is
  always called in the caller's mutation transaction (rule 7).
- 2026-10-08 · argon2id via github.com/alexedwards/argon2id, fixed
  params m=65536 (64MiB), t=3, p=2, salt 16, key 32 — used for
  passwords AND license keys AND OTP codes (rule: if you type float
  near money stop; same seriousness for secrets). Params fixed (not
  runtime.NumCPU) so hashes are portable.
- 2026-10-08 · Sessions: 256-bit random token, base64url to the client,
  sha256-hex (64 chars) at rest. Sessions are NOT tenant-bound (SPEC
  §3), so `Principal{UserID, Audience, TokenHash}` and tenant scope is
  resolved per request from `tenant_users`. DASHBOARD 12h, APP 30d.
- 2026-10-08 · Endpoint status ladder implemented as decided: register
  404 unknown tenant / 400 unknown-or-ungranted product / 409 username
  taken; username `^[A-Za-z0-9._-]{3,64}$`, password ≥8, mobile E.164
  `^\+[1-9][0-9]{7,14}$`; login 401 masked / 409 {otp_required:true}
  when mobile unverified / 403 §6.1 reason when access fails / 200
  {session_token, user{id,username,first_name,role}, verify:<§6.1 body>}.
  A freshly registered user is 403 "awaiting activation or payment"
  until a payment sets validity — truthful per §4, not a bug.
- 2026-10-08 · §6.2 login must return the §6.1 body, so the verify
  service landed in M-A (not M-B). `CheckLicense` compares the presented
  key; `CheckAccount` skips the key (password proved identity).
  `entitlements` is null when no license; subscription dates are
  `2006-01-02`; the pure §4 rule stays in rule.go.
- 2026-10-08 · OTP: 6 digits, argon2id-hashed, 5-min expiry, 5 attempts,
  60s resend cooldown → 429. Purposes REGISTRATION and FIRST_LOGIN are
  wired (PASSWORD_RESET deferred — no reset UI in V1). Verification
  consumes the code, flips mobile_verified (+ PENDING→ACTIVE) and writes
  the MOBILE_VERIFIED audit row in ONE tx.
- 2026-10-08 · ConsoleSender (dev) writes the SMS body to stdout
  DIRECTLY, not through slog — an explicit, documented exception to
  AGENTS rule 8 (SPEC §6.4 mandates a console sink); production always
  uses MSG91Sender. MSG91Sender posts the Flow API with the DLT
  template variable assumed to be named "code" (TRAI approval pending).
- 2026-10-08 · First-boot platform admin: house tenant id 1, role
  SUPERADMIN, mobile_verified=true, ACTIVE, audited
  PLATFORM_ADMIN_BOOTSTRAPPED; idempotent (no-op if the username
  exists); driven by PLATFORM_ADMIN_* env, no-op when unset.
- 2026-10-08 · Dashboard login allows roles SUPERADMIN/ADMIN/EDITOR
  (ALGO_USER → 403); creates a DASHBOARD session and an ADMIN_LOGIN
  audit row; logout requires a live DASHBOARD bearer and deletes the
  session (no other session revocations in M-A; token_version is the
  later lever).
- 2026-10-08 · `httpx.DecodeJSON` added: 1 MiB body cap, lenient on
  unknown fields (forward-compatible clients), 400 with a friendly
  message on malformed JSON. Audit actions so far: USER_REGISTERED,
  MOBILE_VERIFIED, ADMIN_LOGIN, PLATFORM_ADMIN_BOOTSTRAPPED.
- 2026-10-08 · M-A acceptance is a DB-gated test in cmd/hubd
  (register→OTP→login ladder + audit assertions) plus unit tests for
  argon2id/session/OTP. Verified locally against MySQL 8.0.46:
  `go vet ./...` clean, `go test ./... -count=1` all ok.
- 2026-10-09 · CS-4/M-C: dashboard tenancy follows PashuTrack — a
  session is NOT tenant-bound, so `auth.RequireDashboard` resolves
  `Scope{UserID,TenantID,Role,IsPlatformAdmin}` from the caller's first
  `tenant_users` row per request (V1 = one tenant per person). ALGO_USER
  is rejected. This middleware also sets `Principal` (so DashLogout's
  TokenHash lookup still works) and rejects non-ACTIVE users.
- 2026-10-09 · Platform admins (house-tenant SUPERADMIN) may target
  another tenant with the `X-Tenant-ID` header — the single documented
  cross-tenant path (SPEC §7 act-as). Non-platform callers get 403;
  the target tenant is recorded on the audit row. `Access-Control-Allow-
  Headers` now includes `X-Tenant-ID`.
- 2026-10-09 · CS-4 ships 13 of the 14 remaining §7 endpoints. POST
  /dash/payments stays 501 (deferred). Consequence, recorded here: with
  no payment writer, nothing sets `licenses.subscription_valid_until`,
  so admin-created/granted licenses verify as `paused` ("awaiting
  activation or payment") until a payment lands or the date is set
  manually. `POST /dash/users` and license grants create licenses with
  status ACTIVE but no validity window, which is truthful per §4.
- 2026-10-09 · Secret handling: `POST /dash/users` and license
  grant/regenerate return the one-time password / license key in the
  201/200 body only (never logged, only argon2id hashes + last-4 hint
  stored). `POST /dash/tenants` creates tenant + product grants + an
  ADMIN user in one tx; the admin password is returned only when the
  caller did not supply one. Audit actions added: USER_CREATED,
  USER_STATUS_CHANGED, LICENSE_GRANTED, LICENSE_STATUS_CHANGED,
  LICENSE_REGENERATED, TENANT_UPDATED, TENANT_CREATED,
  TENANT_STATUS_CHANGED. Every status/license/tenant mutation writes its
  audit row in the same tx; user DEACTIVATED/BANNED bumps token_version.
- 2026-10-09 · Validation: slug `^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`,
  reusing username/E.164 rules from register; grace_working_days 0–30;
  tenant statuses ACTIVE/INACTIVE, license ACTIVE/DISABLED, user
  ACTIVE/DEACTIVATED/BANNED. CS-4 acceptance is a DB-gated test in
  cmd/hubd (`TestDashboardTenantLifecycle`) covering onboarding, the
  user lifecycle, audit-row-per-mutation, token_version bump, and
  cross-tenant confinement. Verified locally: `go vet ./...` clean,
  `go test ./... -count=1` all ok.
- 2026-10-09 · CS-5/M-D CSV import (SPEC §8): multipart `file` (≤5 MiB,
  ≤5000 data rows). Header row required with `username` and `mobile`;
  other columns matched by name, `product_code` defaults to `flowos`.
  Dedup on (tenant, username) — globally unique in schema — and
  (tenant, mobile) via an in-memory set plus a tenant-scoped query.
  Imported users are `imported=true`, `mobile_verified=false`,
  `PENDING_VERIFICATION`, so first app login drives FIRST_LOGIN OTP.
  Each successful row gets its own password + license key (returned
  once), an ALGO_USER membership, an ACTIVE license, an optional MANUAL
  payment (amount 0, note "imported") and a USER_IMPORTED audit row;
  the whole batch runs in one tx, and `import_batches` records counts +
  the per-row `{row, reason}` list. `row` is the 1-based data-row
  ordinal (header excluded) — a spreadsheet line number would be
  off-by-one/ambiguous with quoted newlines.
- 2026-10-09 · If `plan`/`paid_at`/`valid_from` are present they must
  all be present and valid; `valid_until` = valid_from + (MONTHLY 1 /
  QUARTERLY 3 / ANNUAL 12) calendar months, and the license's
  subscription_valid_until is set in the same tx. Rows missing the
  trio simply have no payment (license stays paused until §7 payments).
- 2026-10-09 · M-D deploy artifacts: added
  `deploy/backup-mysql.sh.example` + `deploy/backup.cron.example`
  (Caddyfile and systemd unit already existed) and a README Runbook
  (build/test, launch, deploy, backup/restore, secrets). M-D acceptance
  tests in cmd/hubd: `TestCSVImportAndFirstLoginOTP` (import with mixed
  per-row results → FIRST_LOGIN OTP → valid login) and
  `TestLoginRateLimit` (the §9 per-IP login ceiling trips 429).
  Verified locally: `go vet ./...` clean, `go test ./... -count=1` all
  ok.
- 2026-10-09 · Dashboard scaffold (CS-D): Vite + React + TS, React Router,
  TanStack Query, Tailwind CSS, Vitest + RTL. Money conversion at the
  UI edge (`lib/money.ts`) — integer paise only on the wire. Auth token +
  act-as tenant (`X-Tenant-ID`) stored in localStorage; platform admins
  can act on another tenant.
- 2026-10-09 · GET /dash/users has no "payment state" filter in the
  backend (SPEC lists filters: status, payment state). The dashboard
  shows a derived badge per user licenses/validity but does not send a
  server-side payment-state filter (deferred to a later change-set) —
  recorded as a documented gap.
- 2026-10-09 · Dashboard pages cover all §7 flows plus an API tools
  page (/app/*) for end-to-end system checks; payments remain read-only
  (POST /dash/payments 501). One-time secrets (passwords/license keys)
  are rendered in modals/tables at creation time only and never logged.
- 2026-10-09 · Frontend tests: 22 passing (money utils, API client, UI
  components, LoginPage). Docker compose extended with a `dashboard`
  service (Vite dev server, npm i on start).
- 2026-10-09 · Blank-screen fix: `buildUserView` now initializes
  `Licenses` non-nil so a license-less user serializes `licenses: []`
  instead of `null` (matching the other list handlers and the TS type);
  the dashboard also guards null lists, and an `ErrorBoundary` now shows
  a readable message instead of a white screen if a render ever throws.
  `ErrorText.children` made optional and `skipLibCheck` enabled in the
  dashboard tsconfig so `npm run typecheck` is green.
- 2026-10-09 · Product catalogue (Part 1 of the dashboard-actions work).
  `products` is seeded, never user-created (SPEC §3 seed data): migration
  `0002_products` idempotently upserts `flowos/FlowOS/FL`,
  `optionalyzer/Optionalyzer/OP`, `dhansanketai/DhanSanket AI/DS`,
  `pashutrack/PashuTrack/PT`, `teleflow/TeleFlow/TF`. The display names
  and key prefixes for the four non-flowos products were supplied by the
  owner (not in SPEC). `GET /dash/tenant` gains an additive
  `products []string` field (omitted when empty via `omitempty`, so the
  nil-slice→`null` trap from the blank-screen fix cannot recur) listing
  the acting tenant's granted product codes — this is what the dashboard
  grant-license picker filters on. Backed by a new tenant-scoped store
  method `GrantedProductCodes`.
- 2026-10-09 · Dashboard actions (Part 2). The dashboard's disabled
  controls are now wired to the §7 endpoints: user status change (reason
  required), license grant (picker limited to the acting tenant's granted
  `products` minus those the user already holds), license
  disable/enable, license-key regenerate (confirm first; new key shown
  once), and the user audit trail (rendered from `GET /dash/users/{id}`'s
  existing `audit[]`, no extra call). Platform admins can create tenants
  (product multi-select from the five seeded codes, default `flowos`) and
  change tenant status. One-time secrets (admin password, license keys)
  render only in a modal at creation time. `POST /dash/payments` still
  501, so the payments card stays a read-only placeholder. The
  grant-license picker reads a new `GET /dash/tenant` query; shared
  `StatusModal`/`OneTimeSecret` components avoid triplicating the
  status+reason and one-time-secret UI. Frontend tests: 35 passing (8
  files).

- 2026-10-10 · Dashboard user creation with access (no CSV). The
  "Create user" modal/endpoint gained optional `plan`, `paid_at`,
  `valid_from` (all-or-nothing, same validation as CSV import). When
  supplied it writes `licenses.subscription_valid_until`, a `MANUAL`
  payment row, and records `valid_until` in the `USER_CREATED` audit row
  — same transaction, mirroring `ImportCSV`. The product field is now a
  dropdown of the acting tenant's granted products instead of a hardcoded
  `flowos` text default. Rationale: the owner asked for easy per-tenant
  user creation through the UI; reusing the exact import semantics keeps
  one meaning of "valid" and §6.1 truthful.

- 2026-10-10 · Dashboard set access window + user-detail mobile
  verification. Added `POST /dash/licenses/{id}/validity` (any dashboard
  role in the acting tenant; tenant-scoped) that sets an existing
  license's `subscription_valid_until` from `plan` + `valid_from` +
  `paid_at`, writing a `MANUAL` ₹0 payment and a `LICENSE_VALIDITY_SET`
  audit row in one transaction — the same window semantics as CSV
  import / create-user, factored into a shared `parseWindow` helper.
  Rationale: a created user with no window verifies as paused forever;
  this closes that gap without a one-off DB edit. The user detail page
  also gained a mobile-verification card (Send/Resend + Verify) that
  calls the existing `/app/otp/request` + `/app/otp/verify`
  (`FIRST_LOGIN`) endpoints, so an admin can drive first-login
  verification without the API-tools page; in dev the code is read from
  the hubd console. Backend tests: `TestSetLicenseValidity` (window,
   payment, audit, 400 partial, 404 cross-tenant). Frontend tests: 39
   passing (8 files).

- 2026-10-10 · Mobile-verify card UX fix. The card only revealed the
  code field after a successful OTP request, so a second click inside the
  §6.4 60s cooldown (HTTP 429) left the admin stuck on "Send code" with
  no way to enter the code they received. The card now treats a 429 as
  "a code is already pending": it shows the code field and a notice, and
  disables "Resend" for a 60s countdown. Frontend tests: 40 passing (8
  files).

- 2026-10-10 · Dev console SMS sender writes to **stderr**, not stdout
  (`auth/sender.go`). slog's default logger already prints to stderr, so
  the `[dev-sms]` code now shares that stream — visible in a terminal
  that merges streams and captured by a `2>&1` redirect, while stdout
  stays clean. Still the SPEC §6.4 dev transport, not the structured
  logger (AGENTS rule 8 exemption unchanged).

- 2026-10-10 · Dev-only OTP echo (`DEV_ECHO_OTP`, default false). When
  it is true **and** `MSG91_*` is unconfigured, `/app/otp/request` adds
  an optional `dev_code` field and the dashboard auto-fills the code —
  one click to verify, no console/log hunting. `OTP.Issue` now returns
  the plaintext code for this; the handler echoes it, but it is still
  never logged (AGENTS rule 8 is about logs; the access log records only
  method/path/status). The same flag also **bypasses the §6.4 60s resend
  cooldown** in dev (`OTP.SetSkipCooldown`), so repeated Send clicks can
  never trap the developer in a 429 dead-end; the cooldown logic itself
  is unchanged and still enforced whenever the flag is off (asserted by
  `TestOTPRequestDevEcho`). Both guards are required so production (which
  always configures MSG91 per §6.4) can never echo or skip the cooldown.
  Additive response field only — §6.1 `/app/verify` is untouched (AGENTS
  rule 1).

- 2026-10-10 · Dev tenant cleanup. The local DB had accumulated throwaway
  tenants from integration runs; there is no delete-tenant endpoint (SPEC
  §7 lists only create/list/status), so added `cmd/purge-tenants`: a
  dry-run-by-default local tool that deletes every tenant except a
  `-keep` allowlist (default `1,11,12`), never the house tenant
  (`is_house=1`), in FK-safe order, with an optional `-backup <path>`
  JSON snapshot. It refuses to delete a user that still has membership,
  license, or payment rows in any tenant. Root cause of the buildup:
  `deleteTenant` (test helper) deleted `licenses` before `payments`, so
  the license delete hit the payments FK, failed silently, and the tenant
  delete then failed — fixed the ordering and made the helper surface
  errors. Keeping tenants 1 (`flowos`, platform admin's home) and 11/12
  (the only real ones). This is local maintenance tooling, not product
  scope.

- 2026-10-10 · Platform-admin cross-tenant user list. SPEC §7's
  `GET /dash/users` is tenant-scoped, so a platform admin could only see
  one tenant at a time via the act-as header; §7 gave no "all tenants"
  read. Added `GET /api/v1/dash/all-users`: the documented platform-admin
  exception to per-tenant scoping (AGENTS rule 6), gated on
  `sc.IsPlatformAdmin` exactly like `GET /dash/tenants` (403
  otherwise), ignoring act-as. It returns one row per `(tenant, user)`
  membership (users ⋈ tenant_users ⋈ tenants), each carrying
  `tenant_id`/`tenant_slug`/`tenant_name`; filters by tenant/status/
  product/q with the same `pageSize` pagination. Read-only — the
  dashboard page only offers View, which switches the act-as tenant to
  that row's tenant then opens the existing tenant-scoped
  `/users/{id}` (so licenses/role load correctly); mutations stay on the
  tenant-scoped pages to keep audit/scoping clear (AGENTS rule 6). New
  §7 route → `TestRouteSurface` count 17→18. SPEC §7 did not list this
  endpoint; recorded here per rule 13 (it keeps §6.1 truthful and is the
  documented platform-admin exception).
