# FlowOS Hub

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
day one). **Separate project sharing this monorepo** — own module,
own deploy, no code shared with the trading platform in either
direction. The only coupling is the wire contract: SPEC.md §6.1.

Start here, in order: `SPEC.md` (the build contract) → `AGENTS.md`
(execution rules) → `backend/migrations/0001_init.up.sql` (schema,
generated from the spec) → milestones in SPEC.md §10.

Already real in this scaffold: the migration, the working-days grace
function WITH its golden tests (`internal/workingdays`), the license
keygen (`internal/license`), and a stdlib route skeleton where every
endpoint 501s with its spec section — the route list is the API
surface; don't add to it.

Week-one owner action that no builder can do for you: TRAI DLT
registration for SMS OTP (SPEC.md §6.4). Start it before the code.

Dev quickstart: `docker compose up -d db` → apply migrations →
`go run ./cmd/hubd` (once go.mod deps are added per spec).
