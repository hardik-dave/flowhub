# FlowOS Hub — Build Specification v1.1 (multi-product)

**What this is:** the complete, self-contained specification for the
FlowOS Hub — the tenant-scoped licensing, user-management, and
subscription backend for the company's products. FlowOS (the desktop
trading platform) is the first product; the licensing model is
product-dimensional from day one (v1.1). **Identity is
product-agnostic; licenses are per-product**: one user row per person
per tenant, holding N licenses, each binding them to one product with
its own key, validity, and entitlements.
This document is written to be executed by a contractor or an AI
coding agent WITHOUT access to the FlowOS repo or its authors.
Where this spec is silent, choose the simplest option and record the
choice in DECISIONS.md. Do not invent scope.

**Golden rule:** the Hub's `/api/v1/app/verify` endpoint is a
contract already consumed by a shipped desktop client. Its response
shapes in §6.1 are law. Everything else in the Hub exists to feed
that endpoint truthfully.

---

## 1. Scale posture & stack (locked — do not "improve")

> **Amended 2026-10-08** — owner decision, CTO-confirmed: the
> database engine is **MySQL 8.0**, not PostgreSQL, and the identity
> model follows the owner's `db_schemas.txt` (PashuTrack pattern:
> BIGSERIAL ids, tenant-agnostic users joined via `tenant_users`).
> Driver: `go-sql-driver/mysql` instead of pgx. §6.1 is unchanged.
> Every deviation is logged in DECISIONS.md.

- Tenants: single digits. Users: hundreds to low thousands. Writes:
  signups/day + one license verify per user per morning. This is a
  small CRUD system with high correctness stakes, NOT a scale
  problem.
- **Backend:** Go 1.22+, single binary, modular monolith. Router:
  chi. DB access: `database/sql` + `go-sql-driver/mysql` (no ORM).
  Migrations: golang-migrate (mysql driver).
  Passwords & license keys: argon2id (alexedwards/argon2id).
- **Database:** MySQL 8.0 (dev: 8.0.46). One database. Tenancy =
  `tenant_id` columns plus the `tenant_users` join for identity,
  scoping enforced in one repository layer (every query resolves the
  caller's tenant; no query without it except platform-admin paths).
- **Frontend:** React + Vite + TypeScript SPA (tenant admin
  dashboard). Minimal dependencies; no component framework required.
- **Deploy:** one VPS (Mumbai), Caddy for TLS reverse proxy,
  systemd service, nightly `mysqldump` with a RESTORE TESTED monthly.
- **Explicitly banned:** microservices, message queues, Kubernetes,
  per-tenant databases, ORMs, GraphQL, payment-gateway processing
  (V1 records payments; it does not move money).

## 2. Locked product decisions (from the owner — do not revisit)

1. No payment gateway in V1. Payments are recorded manually by
   tenant admins. Schema is gateway-ready (nullable gateway fields,
   `method` enum) for a later Razorpay integration.
2. Hub is a separate project/repo from FlowOS. Coupling is the wire
   contract in §6 only. No shared code.
3. Direct retail users belong to a seeded **house tenant**
   (slug `flowos`, `is_house = true`). Same code paths as any
   tenant; zero special-casing.
4. CSV import of users is V1. Imported users get
   `mobile_verified = false` and must complete OTP on first login.
5. Payment-lapse grace: **5 working days** default (Mon–Fri,
   skip Sat/Sun; public holidays out of scope), configurable per
   tenant (`grace_working_days`). NOTE: this is distinct from the
   FlowOS desktop's own 72h OFFLINE grace (server unreachable) —
   different mechanism, lives in the desktop, not here.
6. **Multi-product (v1.1):** licensing carries a product dimension.
   Seeded product: `flowos`. Users are shared across products within
   a tenant; licenses, payments, and entitlements are per-product.
   Tenants distribute only products granted to them
   (`tenant_products`). What this is NOT (banned in §11): bundles,
   per-product user roles, cross-product entitlement logic, SSO.

## 3. Data model (MySQL 8 DDL — authoritative)

> **Amended 2026-10-08** — MySQL dialect; `tenants`/`users`/
> `tenant_users` follow the owner's `db_schemas.txt` verbatim except
> where noted; the licensing/subscription core keeps this spec's
> original shape in the new dialect. Required deviations (logged in
> DECISIONS.md): `users.status` keeps four values so §4 can split
> `paused` from `revoked`; `grace_period_days` → `grace_working_days`
> (Mon–Fri); `slug`, `mobile_verified`, `is_house`, product/PII
> columns added because §4/§6/§7 cannot function without them.
> `CITEXT` is replaced by the default case-insensitive collation
> (`utf8mb4_0900_ai_ci`).

```sql
CREATE TABLE tenants (
    id                  BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    slug                VARCHAR(64) NOT NULL UNIQUE,   -- used in signup links & app login
    name                TEXT NOT NULL,
    contact_person      TEXT,
    contact_no          TEXT,
    start_date          DATE NOT NULL,
    end_date            DATE,
    is_house            BOOLEAN NOT NULL DEFAULT FALSE,  -- house tenant (§2.3): platform admins live here
    grace_working_days  INT NOT NULL DEFAULT 5 CHECK (grace_working_days BETWEEN 0 AND 30),
    status              TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','INACTIVE')),
    created_by          BIGINT,                        -- audit metadata; no FK (see DECISIONS.md)
    updated_by          BIGINT,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE products (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    code        VARCHAR(64) NOT NULL UNIQUE,   -- 'flowos'; used in API requests & CSV
    name        TEXT NOT NULL,
    key_prefix  VARCHAR(8) NOT NULL,           -- 'FL' → keys look like FL-XXXX-…
    status      TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','RETIRED')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE tenant_products (                -- which products a tenant may distribute
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id),
    product_id  BIGINT NOT NULL REFERENCES products(id),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, product_id)
);

CREATE TABLE users (                          -- tenant-agnostic identity (owner's model); membership via tenant_users
    id                  BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    username            VARCHAR(64) NOT NULL UNIQUE,  -- globally unique (owner's model — DECISIONS.md)
    password_hash       TEXT NOT NULL,                -- argon2id
    first_name          TEXT,
    last_name           TEXT,
    contact_no          VARCHAR(20),                  -- E.164, e.g. +91XXXXXXXXXX (API field name: mobile)
    mobile_verified     BOOLEAN NOT NULL DEFAULT FALSE,
    email               VARCHAR(191),                 -- optional
    city                TEXT,
    state               TEXT,
    broker_client_code  TEXT,                         -- the user's ID at the broker (optional)
    referral_code       TEXT,                         -- optional, free text V1
    status              TEXT NOT NULL DEFAULT 'PENDING_VERIFICATION'
                        CHECK (status IN ('PENDING_VERIFICATION','ACTIVE','DEACTIVATED','BANNED')),
    token_version       INT NOT NULL DEFAULT 1,       -- bump to invalidate all sessions
    imported            BOOLEAN NOT NULL DEFAULT FALSE,
    import_batch_id     BIGINT,
    created_by          BIGINT,                       -- audit metadata; no FK
    updated_by          BIGINT,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE tenant_users (                   -- per-tenant membership and role
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id),
    user_id     BIGINT NOT NULL REFERENCES users(id),
    role        TEXT NOT NULL CHECK (role IN ('SUPERADMIN','ADMIN','EDITOR','ALGO_USER')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, user_id)
);

CREATE TABLE licenses (
    id                       BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id                BIGINT NOT NULL REFERENCES tenants(id),
    user_id                  BIGINT NOT NULL REFERENCES users(id),
    product_id               BIGINT NOT NULL REFERENCES products(id),
    license_key_hash         TEXT NOT NULL,            -- argon2id; key shown ONCE at creation
    license_key_hint         VARCHAR(4) NOT NULL,      -- last 4 chars, for support convos
    status                   TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
                                                     -- DISABLED = this product revoked without banning the user
    subscription_valid_until DATE,                     -- denormalized from payments; NULL = never paid
    entitlements             JSON NOT NULL DEFAULT (CAST('{"max_activations":1,"tier":"RETAIL"}' AS JSON)),
    created_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, user_id, product_id)
);

CREATE TABLE payments (
    id                  BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id           BIGINT NOT NULL REFERENCES tenants(id),
    user_id             BIGINT NOT NULL REFERENCES users(id),   -- kept for query convenience
    license_id          BIGINT NOT NULL REFERENCES licenses(id),-- the product-subscription this payment extends
    amount_minor_units  BIGINT NOT NULL CHECK (amount_minor_units >= 0),  -- paise; COMPLIMENTARY = 0
    currency            VARCHAR(3) NOT NULL DEFAULT 'INR',
    method              TEXT NOT NULL CHECK (method IN ('MANUAL','COMPLIMENTARY','GATEWAY')),
    gateway_payment_id  TEXT,                          -- NULL until gateway exists
    plan                TEXT NOT NULL CHECK (plan IN ('MONTHLY','QUARTERLY','ANNUAL')),
    paid_at             DATE NOT NULL,
    valid_from          DATE NOT NULL,
    valid_until         DATE NOT NULL,                 -- valid_from + plan duration
    recorded_by         BIGINT NOT NULL,               -- the admin who entered it
    note                TEXT,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
-- On INSERT (same transaction): licenses.subscription_valid_until =
-- GREATEST(current value, valid_until). Never decreases automatically;
-- an admin correcting a mistake does so via an explicit adjustment
-- payment row or a status change — both audited.

CREATE TABLE otp_codes (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    purpose     TEXT NOT NULL CHECK (purpose IN ('REGISTRATION','FIRST_LOGIN','PASSWORD_RESET')),
    code_hash   TEXT NOT NULL,                         -- 6 digits, hashed; NEVER logged
    expires_at  DATETIME NOT NULL,                     -- supplied by app: now + 5 minutes
    attempts    INT NOT NULL DEFAULT 0,                -- max 5, then invalidated
    consumed_at DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    audience    TEXT NOT NULL CHECK (audience IN ('DASHBOARD','APP')),
    token_hash  VARCHAR(64) NOT NULL,                  -- random 256-bit, sha256-hex at rest
    expires_at  DATETIME NOT NULL,                     -- dashboard 12h, app 30d
    ip          VARCHAR(45),
    user_agent  TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE audit_log (
    id              BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,                   -- no FK: audit rows are append-only evidence
    actor_user_id   BIGINT,                            -- NULL = system
    action          TEXT NOT NULL,   -- e.g. USER_STATUS_CHANGED, PAYMENT_RECORDED, LICENSE_REGENERATED, USER_IMPORTED, ADMIN_LOGIN
    subject_user_id BIGINT,
    before          JSON,
    after           JSON,
    reason          TEXT,                              -- REQUIRED for status changes
    at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- Every mutation of user status, subscription, license, or payment
-- writes an audit row IN THE SAME TRANSACTION. No exceptions.

CREATE TABLE import_batches (
    id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id),
    filename    TEXT NOT NULL,
    row_count   INT NOT NULL,
    ok_count    INT NOT NULL,
    error_count INT NOT NULL,
    errors      JSON,                                 -- [{row, reason}]
    created_by  BIGINT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

Seed data (migration): product `{code: 'flowos', name: 'FlowOS', key_prefix: 'FL'}`;
house tenant `{id: 1, slug: 'flowos', name: 'FlowOS Direct', is_house: true,
start_date: today}` (id 1 reserved for the house tenant, per the owner's
`db_schemas.txt` convention) granted all products in tenant_products; one
platform-admin user under it with `tenant_users.role = 'SUPERADMIN'`
(credentials via env at first boot).

## 4. Access rule (the single source of truth)

A user may use a given product iff ALL of (evaluated per license):
1. `tenant.status = 'ACTIVE'` AND the product is granted to the
   tenant in `tenant_products` AND `product.status = 'ACTIVE'`
2. `user.status = 'ACTIVE'` (not PENDING_VERIFICATION / DEACTIVATED / BANNED)
3. `user.mobile_verified = true`
4. A license row exists for (user, product) with `status = 'ACTIVE'`
   and the presented key matches its hash
5. Subscription current: `today <= license.subscription_valid_until`
   OR within grace: `today <= add_working_days(license.subscription_valid_until, tenant.grace_working_days)`

`add_working_days(d, n)`: advance day-by-day from d, counting only
Mon–Fri, n times. Implement as a pure function WITH unit tests
(golden cases: Fri + 5 → next Fri; Sat/Sun starts normalize forward).

Status mapping to the FlowOS contract:
| Hub condition | verify `status` | `reason` template |
|---|---|---|
| All checks pass, subscription current | `valid` | "" |
| All pass, in grace window | `valid` (+ `warning`) | "Subscription expired on {date} — {n} working day(s) of grace remain. Renew to avoid interruption." |
| Grace exhausted / never paid | `paused` | "Subscription expired on {date} — renew to continue. Contact {tenant.name}." / "Account awaiting activation or payment. Contact {tenant.name}." |
| user DEACTIVATED | `paused` | "Deactivated by {tenant.name}: {audit reason}" |
| user BANNED or tenant INACTIVE | `revoked` | "Access removed by {tenant.name}. Contact them to restore access." |
| license DISABLED (this product only) | `revoked` | "This product's license was removed by {tenant.name}." |
| No such user / wrong key / no license for this product / product not granted to tenant | `not_found` | "License not recognised. Check your User ID and license key." |

**Enforcement is at login/verify only — the Hub must never push
mid-session kills to a trading terminal.** Lapses take effect at the
next app launch.

## 5. License keys

Format: `{product.key_prefix}-XXXX-XXXX-XXXX-XXXX` (crockford
base32, 80 bits random) — FlowOS keys read `FL-…`; the prefix is
support convenience only, never parsed for authorization.
Generated at registration/import; displayed EXACTLY ONCE in the
creation/import response; stored argon2id-hashed + last-4 hint.
Admin action "Regenerate key" (audited) invalidates the old one.
Rate-limit verify attempts per user (10/min) — a license key is a
credential and gets credential treatment.

## 6. API — app-facing (consumed by FlowOS desktop)

Base: `https://hub.<domain>/api/v1`. JSON, snake_case. All errors:
`{"error": "<human-readable, actionable>"}` with correct HTTP codes.

### 6.1 POST /app/verify — THE GOLDEN CONTRACT
Request:
```json
{ "tenant_slug": "alphabroker", "username": "hardik01", "product_code": "flowos", "license_key": "FL-9F4K-22XQ-7T1B-M2A8" }
```
Response `200` (always 200 for a processed check; the status field carries the verdict):
```json
{
  "status": "valid",
  "reason": "",
  "warning": "Subscription expired on 12 Sep 2026 — 3 working day(s) of grace remain. Renew to avoid interruption.",
  "entitlements": { "max_activations": 1, "tier": "RETAIL" },
  "subscription": { "valid_until": "2026-09-12", "grace": { "active": true, "until": "2026-09-17" } }
}
```
`status` ∈ `valid | paused | revoked | not_found` (exact strings —
the desktop's LicenseVerifier switches on them). `warning` empty
string when not in grace. `entitlements` echoes `licenses.entitlements` and is
additive-extensible (future: tier PRO_DESK, max_activations > 1).
`product_code` is REQUIRED in the request; each app sends its own
constant. Never rename or remove a response field; only add.

### 6.2 POST /app/login
`{tenant_slug, username, password, product_code}` →
- `200 {session_token, user: {id, username, first_name, role}, verify: <the full §6.1 body>}` —
  login answers the license question in the same call so the app
  makes ONE request at startup.
- `401 {"error": "Invalid username or password."}` (identical for
  unknown user vs wrong password)
- `403 {"error": "<the §6.1 paused/revoked reason>"}` when
  credentials are right but access rule fails
- `409 {"error": "Mobile verification required.", "otp_required": true}` for
  imported users' first login → client drives §6.4 then retries.

### 6.3 POST /app/register
`{tenant_slug, product_code, username, password, first_name, last_name, mobile, email?, city?, state?, broker_client_code?, referral_code?}`
→ `201 {user_id, license_key: "FL-… (SHOWN ONCE)", otp_sent: true}`.
Creates the user AND their license for `product_code` (tenant must
hold that product in tenant_products, else 400 with reason). If the
username already exists in the tenant with a verified mobile and
correct password is later established via login, an ADMIN can grant
an additional product via the dashboard instead — self-serve
cross-product signup for existing users is out of scope V1.
User is created `PENDING_VERIFICATION`; §6.4 flips to `ACTIVE` +
`mobile_verified` — access then still awaits payment/activation per §4.
Validation errors are specific: `400 {"error": "mobile must be E.164, e.g. +919876543210"}`.

### 6.4 POST /app/otp/request · POST /app/otp/verify
`{tenant_slug, username, purpose}` → `200 {otp_sent: true, expires_in_seconds: 300}` (resend cooldown 60s).
`{tenant_slug, username, purpose, code}` → `200 {verified: true}` |
`400` wrong/expired (max 5 attempts then invalidated).
OTP delivery behind an interface: `SmsSender` with `ConsoleSender`
(dev — prints to log) and `MSG91Sender` (prod, env-configured).
**TRAI DLT registration (sender ID + template approval) is a
paperwork prerequisite for production SMS — flag it in README as a
week-one owner action.**

## 7. API — dashboard-facing (tenant admins; house-tenant admins are platform admins)

Session cookie or bearer token (audience DASHBOARD). Every endpoint
scoped to the actor's tenant; house-tenant ADMINs may pass an
explicit `tenant_id` to act across tenants (audited).

- `POST /dash/login`, `POST /dash/logout`
- `GET /dash/users?status=&product=&q=&page=` — list/search (username, name, mobile, key hint)
- `POST /dash/users` — create (admin-created user; returns one-time key)
- `PATCH /dash/users/{id}/status` — `{status, reason}` — reason REQUIRED; audited
- `POST /dash/users/{id}/licenses` — `{product_code}` grant an additional product license (tenant must hold the product); returns one-time key; audited
- `PATCH /dash/licenses/{id}/status` — `{status, reason}` ACTIVE/DISABLED per product; audited
- `POST /dash/licenses/{id}/regenerate-key` — audited; returns one-time key
- `GET /dash/users/{id}` — detail incl. payment history + audit trail
- `POST /dash/payments` — `{license_id, amount_minor_units, method, plan, paid_at, valid_from, note}`;
  server computes valid_until (MONTHLY +1 month, QUARTERLY +3, ANNUAL +12, calendar arithmetic);
  updates the license's subscription_valid_until in-tx; audited
- `POST /dash/imports` — multipart CSV per §8 → import_batch result
- `GET /dash/audit?subject_user_id=&page=` — read-only audit view
- `GET /dash/tenant` / `PATCH /dash/tenant` — name, grace_working_days
- Platform-admin only: `POST /dash/tenants` (onboard a broker), `PATCH /dash/tenants/{id}/status`

Dashboard SPA pages (V1): Login · Users list (filters: status,
payment state) · User detail (status controls with mandatory reason
box, payment entry form, key regenerate, audit trail) · CSV import
(upload → per-row results) · Tenant settings. Money entered in
rupees in the UI, converted to paise at the API boundary, stored as
integers only.

## 8. CSV import format

Header row required, UTF-8:
`username,first_name,last_name,mobile,email,city,state,broker_client_code,referral_code,product_code,plan,paid_at,valid_from`
(`product_code` optional; defaults to `flowos`; row errors if the
tenant doesn't hold the product)
- Password: none — imported users get a system-generated reset link
  flow? NO (out of scope V1): imported users receive their
  credentials from the tenant; Hub sets a random password AND the
  import response includes per-row one-time initial passwords +
  license keys for the tenant to distribute. `mobile_verified=false`;
  first app login triggers FIRST_LOGIN OTP (§6.2 → 409 flow).
- If plan/paid_at/valid_from present → create a MANUAL payment row
  (amount 0 allowed with note "imported").
- Per-row failures don't abort the batch; result lists `{row, reason}`.
- Dedup: (tenant_id, username) or (tenant_id, mobile) conflict → row error.

## 9. Security non-negotiables

argon2id for passwords AND license keys · sessions as random 256-bit
tokens, sha256 at rest · rate limits: 10/min per IP on login &
verify, OTP cooldowns per §6.4 · TLS only (Caddy) · CORS locked to
the dashboard origin · no card/bank data anywhere (out of PCI scope
by design) · PII minimalism: collect only fields in §3 · OTP codes
and license keys NEVER in logs · structured request logging with
user/tenant IDs · nightly mysqldump + documented restore procedure ·
`.env` for secrets, never committed.

## 10. Milestones & acceptance (definition of done)

**M-A Schema + auth (week 1):** migrations apply clean; register →
OTP (console sender) → login round-trip; argon2id verified; seeds in.
**M-B Verify contract (week 2):** §6.1 passes the golden table-driven
acceptance suite — one test case per row of the §4 mapping table,
asserting EXACT status strings and reason templates, including the
product cases (no license for product → not_found; product not
granted to tenant → not_found; license DISABLED → revoked; second
product unaffected by first product's lapse); grace working-days
function unit-tested against golden dates.
**M-C Dashboard (weeks 3–4):** all §7 flows usable end-to-end;
every mutation produces an audit row (test asserts it); payment
entry updates validity in the same transaction.
**M-D Import + polish (week 5):** CSV import with per-row results;
FIRST_LOGIN OTP flow; rate limits verified by test; deploy scripts
(Caddyfile, systemd unit, backup cron) in `deploy/`.

Global DoD: `go vet` + `go test ./...` clean; a `docker compose up`
dev environment (mysql + hub + dashboard); README with runbook;
DECISIONS.md recording every choice this spec left open.

## 11. Out of scope for V1 (do not build)

Payment gateway processing · product bundles/suites · per-product
roles or per-product user profiles · cross-product entitlement logic
· self-serve cross-product signup · SSO/OAuth across products ·
EDITOR role (enum exists conceptually; ship ADMIN + ALGO_USER) · device fingerprinting (entitlements.
max_activations is a number, unenforced in V1) · email verification ·
SSO · analytics dashboards · webhooks to tenants · the Remote Share
relay (separate service, later, same box at most) · multi-language.
