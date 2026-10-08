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
