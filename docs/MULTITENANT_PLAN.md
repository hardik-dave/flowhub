# Multi-Tenant Management System — FlowOS Hub

**Scope:** Implement the tenant-scoped multi-tenant management system for FlowOS Hub (licensing, user-management, subscription backend) per `SPEC.md`.

**Reference:** PashuTrack API (`pashutrackapi`) — its tenant-scoping pattern is the template; where Hub's SPEC differs, Hub's SPEC wins.

## 1. Reference Analysis (pashutrackapi)

| Aspect | PashuTrack Pattern |
|--------|--------------------|
| Tenant model | `tenants` table (BIGSERIAL id), seed tenant for the product owner |
| User→tenant | Tenant-agnostic `users` + `tenant_users` join (per-tenant role: SUPERADMIN/ADMIN/EDITOR) |
| Tenant identity in requests | JWT claims carry `tenant_id`; auth middleware injects `AuthContext{UserID, TenantID, Role}` into request context |
| Query isolation | Every SQL query includes `WHERE tenant_id = $N` (parameterized from auth context). No RLS — application-level isolation |
| Tenant mgmt API | None — tenants seeded directly in DB |
| Tenant switching | None — `LIMIT 1` at login bakes one tenant into the token |

## 2. Hub vs PashuTrack — Deliberate Differences

| Aspect | PashuTrack | FlowOS Hub (SPEC) |
|--------|-----------|---------------------|
| Tenant ID | `BIGSERIAL` (int64) | `UUID` |
| Auth | JWT (24h) | Random 256-bit session tokens, sha256-hashed at rest (12h DASHBOARD / 30d APP) |
| User→tenant | Join table, tenant-agnostic users | `users.tenant_id` FK directly (one user row per person per tenant) |
| Tenant switching | None | House-tenant ADMINs may pass explicit `tenant_id` (audited) |
| Tenant mgmt API | None | Full: onboard, suspend/activate, self-service settings (`GET/PATCH /dash/tenants`, `GET/PATCH /dash/tenant`) |
| Roles | SUPERADMIN/ADMIN/EDITOR | ADMIN / ALGO_USER (spec §11: no EDITOR in V1) |

## 3. Current State of Hub (July 2026 scaffold)

Already built:
- `backend/migrations/0001_init.{up,down}.sql` — full schema (10 tables + seeds), matches SPEC §3
- `internal/license/keygen.go` — Crockford base32 `FL-XXXX-XXXX-XXXX-XXXX` keys + hint
- `internal/workingdays/` — grace-period working-day arithmetic + golden tests
- `cmd/hubd/main.go` — stdlib route skeleton, every business route → 501 (lists the API surface)
- `docker-compose.yml` (Postgres only), `deploy/` examples, `.env.example`, CI workflow

Not built:
- All go.mod deps · DB connection · migration runner · auth/sessions · OTP · register/login · verify — everything returns 501
- Platform-admin tenant routes (`POST /dash/tenants`, `PATCH /dash/tenants/{id}/status`) absent from the scaffold route list
- Dashboard SPA (placeholder README only)

## 4. Target Structure

```
backend/
  cmd/hubd/main.go              # entry: migrate + first-boot platform admin + serve
  cmd/migrate/main.go            # standalone migration runner (optional)
  internal/
    config/config.go             # env loading (DATABASE_URL, HUB_LISTEN, SESSION_SECRET, PLATFORM_ADMIN_*, MSG91_*)
    db/db.go                     # pgx pool, Ping, Close
    auth/
      session.go                 # 256-bit token gen; sha256 lookup; DASHBOARD/APP audiences
      middleware.go              # bearer → AuthContext{UserID, TenantID, Role, Audience}; house-tenant cross-tenant override
      password.go                # argon2id
      otp.go                     # 6-digit gen/verify, attempts, expiry; SmsSender interface (Console/MSG91)
    verify/service.go            # §6.1 golden contract + §4 access-rule evaluation
    tenant/
      repository.go              # tenant CRUD (platform-admin paths exempt from tenant param — documented)
      handler.go                 # GET/PATCH /dash/tenant; POST /dash/tenants; PATCH /dash/tenants/{id}/status
    user/
      repository.go              # user CRUD, always takes tenant_id
      handler.go                 # register + /dash/users CRUD status flows
    license/
      keygen.go                  # (exists)
      repository.go              # license CRUD, always takes tenant_id
      handler.go                 # grant / status / regenerate-key
    payment/
      repository.go              # payment insert + in-tx license validity update
      handler.go                 # POST /dash/payments
    audit/
      repository.go              # write (same tx as mutation) + read (GET /dash/audit)
    import/
      handler.go                 # CSV import, per-row results
    workingdays/                 # (exists, keep)
  migrations/                    # (exist, keep)
dashboard/                       # React + Vite + TS (new)
deploy/                          # finalize Caddyfile, systemd unit, backup cron
```

## 5. Implementation Phases

### Phase 1 — Foundation (M-A, week 1)

1. Add deps to `go.mod`: chi/v5, pgx/v5, golang-migrate/v4, alexedwards/argon2id, google/uuid. Run `go mod tidy`.
2. `internal/config` — load `.env`; `internal/db` — pgx pool.
3. Main: run migrations at startup; first-boot create platform admin from `PLATFORM_ADMIN_*` under the house tenant (`is_house=true`), role ADMIN, `mobile_verified=true`, audited.
4. `internal/auth` — session tokens (PashuTrack middleware pattern, sessions table instead of JWT); argon2id; OTP with `SmsSender` (`ConsoleSender` dev / `MSG91Sender` prod).
5. Register → OTP → login round-trip (spec §6.3/§6.4/§6.2):
   - `POST /app/register` — validate (E.164 mobile etc.), verify tenant holds product, create user `PENDING_VERIFICATION`, create license (key shown once), send OTP, audit.
   - `POST /app/otp/request|verify` — cooldowns, max 5 attempts, flip to ACTIVE + mobile_verified, audit.
   - `POST /app/login` — single startup call: session + user + full §6.1 verify body; 401 masked errors, 403 access reasons, 409 requires OTP.
   - `POST /dash/login|logout` — DASHBOARD audience session.
6. Write audit row for every user-status mutation in the same transaction.

### Phase 2 — Verify Contract + Scoping (M-B, week 2)

1. `internal/verify/service.go` — evaluate §4 matrix:
   - tenant ACTIVE + product granted + product ACTIVE; user ACTIVE; mobile_verified; license ACTIVE + key matches; subscription current or within `add_working_days(valid_until, grace)`.
   - Map to exact strings: `valid` / `valid`+warning / `paused` / `revoked` / `not_found` with the exact §4 reason templates.
2. Enforce tenant scoping in one repository layer — every method takes `tenant_id`.
3. House-tenant ADMIN cross-tenant override: `X-Tenant-ID` header → override scope for request, written to audit_log (actor + target tenant).
4. Rate limits: 10/min per IP on login & verify; OTP cooldowns.

### Phase 3 — Tenant Management + Dashboard (M-C, weeks 3–4)

1. **Platform-admin tenant endpoints** (`internal/tenant`):
   - `POST /dash/tenants` — onboard broker: tenant row + admin user + product grant; audited.
   - `PATCH /dash/tenants/{id}/status` — suspend/activate; reason required; audited.
2. **Self-service:** `GET/PATCH /dash/tenant` (name, `grace_working_days`); audited.
3. **User endpoints:** list/search, create (one-time key), detail (payments + audit trail), status change (reason required).
4. **License endpoints:** grant additional product, status per product, regenerate key (old invalidated, shown once).
5. **Payments:** record MANUAL/COMPLIMENTARY; server computes `valid_until` (MONTHLY+1m/QUARTERLY+3m/ANNUAL+12m calendar arithmetic); update `licenses.subscription_valid_until` in same tx; audited.
6. **Audit view:** `GET /dash/audit?subject_user_id=&page=`.
7. **Dashboard SPA:** Login · Users (filters by status/product/payment) · User detail (status + reason box, payment entry, regenerate key, audit trail) · CSV import · Tenant settings · Platform-admin: tenant list/onboard/suspend. Money as integer paise in API; rupees only at the UI edge.

### Phase 4 — Import + Polish (M-D, week 5)

1. `POST /dash/imports` — multipart CSV, header required, UTF-8; per-row user + license (+ optional MANUAL payment, amount 0 for imported rows); per-row failures don't abort batch → `{row, reason}`; responses carry per-row one-time passwords + license keys; dedup on `(tenant_id, username)` / `(tenant_id, mobile)`.
2. FIRST_LOGIN OTP flow for imported users (409 → otp → retry).
3. Deploy: finalize Caddyfile, systemd unit, nightly `pg_dump` + monthly restore test documented.
4. Global DoD: `go vet && go test ./...` clean (paste actual output); `docker compose up` (postgres + hub + dashboard); README runbook.

## 6. Tests That Ship With Code (non-negotiable)

- §4 access-rule matrix as table-driven acceptance tests asserting **exact** status strings + reason templates (incl. product cases: no license → `not_found`, product not granted → `not_found`, license DISABLED → `revoked`, second product unaffected by first product's lapse).
- `add_working_days` golden-date tests (existing file kept/extended).
- Audit-row-per-mutation assertions (register / status changes / payments / license ops / tenant ops).
- Rate-limit tests (login, verify, OTP).
- Cross-tenant guard tests: tenant A cannot read/modify tenant B data; cross-tenant act-as writes an audit row.

## 7. Decisions to Record in DECISIONS.md

1. Session tokens (256-bit sha256) instead of JWT — per SPEC §3 sessions table.
2. `users.tenant_id` FK directly (spec model) instead of PashuTrack's join table.
3. House-tenant cross-tenant act-as via `X-Tenant-ID` header — audited.
4. UUID PKs (spec) vs PashuTrack's BIGSERIAL.
5. No RLS — app-layer scoping in one repository layer (matches both spec and PashuTrack).
6. Migrations applied at binary startup (plus optional standalone runner) — simplest option per AGENTS.md rule 13.

## 8. Out of Scope (SPEC §11 — do not build)

Payment gateway processing · bundles/suites · per-product roles · cross-product entitlement logic · self-serve cross-product signup · SSO/OAuth · EDITOR role · device fingerprinting/max_activations enforcement · email verification · analytics dashboards · webhooks · Remote Share relay · multi-language.