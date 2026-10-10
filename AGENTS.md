# AGENTS.md — rules for whoever (or whatever) builds FlowHub

You are building from SPEC.md. That document is the
contract. These rules govern HOW you execute it.

1. **The §6.1 /app/verify response is consumed by a shipped desktop
   client.** Its field names, status strings (`valid|paused|revoked|
   not_found`), and reason-template style are law. `product_code` is
   required in requests. Never rename, remove, or restructure —
   only add optional fields.
2. **Multi-product means a product dimension on licenses — nothing
   more.** No bundles, no per-product roles, no cross-product logic,
   no SSO. Identity stays product-agnostic; one user row per person
   per tenant.
3. **Do not expand scope.** §11's out-of-scope list is binding. If a
   feature feels obviously useful, note it in DECISIONS.md and move
   on. Do not build it.
4. **Do not "upgrade" the stack.** No ORM, no GraphQL, no
   microservices, no Kubernetes, no message queues, no per-tenant
   databases. chi + go-sql-driver/mysql + golang-migrate + React/Vite
   as specified (MySQL per the 2026-10-08 amendment — DECISIONS.md).
5. **Money is integer paise (`amount_minor_units BIGINT`)
   everywhere** — API, DB, tests. Rupee conversion happens only at
   the dashboard UI edge. If you type `float` near money, stop.
6. **Every query is tenant-scoped.** The repository layer takes
   tenant_id on every call. A cross-tenant read/write path outside
   the documented platform-admin routes is a critical bug.
7. **Every status/subscription/license/payment mutation writes an
   audit_log row in the same transaction**, with a non-empty reason
   for status changes. A mutation without its audit row fails review.
8. **Secrets and codes never appear in logs** — no OTP codes, no
   license keys, no password material, at any log level.
9. **Tests ship with the code, same commit.** Non-negotiable
   minimums: the §4 mapping table as table-driven acceptance tests
   with EXACT string assertions; add_working_days golden-date unit
   tests; audit-row-per-mutation assertions; rate-limit tests.
10. **Never modify a failing test to make it pass.** If a test blocks
   you, the code is wrong or the spec is ambiguous — record which in
   DECISIONS.md and, if ambiguous, choose the simplest reading.
11. **Errors are human-readable and actionable** ("mobile must be
    E.164, e.g. +919876543210"), and auth failures are deliberately
    uninformative ("Invalid username or password." — identical for
    unknown user and wrong password).
12. **Small changes.** One milestone section per PR/change-set. Run
    `go vet && go test ./...` before claiming anything done, and
    paste the actual output, not a summary of it.
13. When the spec is silent, pick the simplest option that keeps §6.1
    truthful, and record it in DECISIONS.md with one line of why.