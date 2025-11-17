# Shadow DB Bootstrap API

**Status**: Draft – proposal to expose a developer-facing API for preparing and borrowing the Lockplane shadow database so external tooling (e.g., SQLAlchemy `create_all()`) can populate schema definitions before running `lockplane plan`.

## Background & Motivation
- Today: Users must provision an external "scratch" database if they want an ORM or DSL to emit DDL that Lockplane can diff.
- Request: Allow developers to reuse the existing shadow database, as long as it starts from a clean state. They could call `create_all()` against it, then ask Lockplane to diff shadow ↔ target, effectively treating the shadow DB as the "from" state.
- We already clean/reset the shadow DB for each validation. Providing a small API that does "clean + return connection string" makes this workflow easier without requiring extra infrastructure.

## Proposed Flow
1. User calls `lockplane shadow prepare --environment local` (CLI) or `lockplanectl shadow prep` API.
   - Lockplane cleans the shadow DB for that environment and returns a temporary connection URL + credentials.
2. User runs their tool (e.g., SQLAlchemy `create_all()`) against the returned shadow URL, generating the desired schema.
3. User invokes `lockplane plan --from shadow --target-environment local` (new flag) or `lockplane shadow diff --target local`. Lockplane treats the prepared shadow DB as the "from" side and the real target as "to".
4. Lockplane produces a plan JSON (or direct apply) and resets the shadow DB afterward.

## API Surface Ideas
- `lockplane shadow prepare --target-environment local` → prints JSON `{ "shadow_url": "postgres://..." }` and stores a lock so other commands know the shadow DB is in use.
- `lockplane shadow diff --target-environment local` → equivalent to `plan` but assumes `--from=<prepared-shadow>`.
- Optional `lockplane shadow release` to manually give up the reservation.

## Safety Considerations
- Shadow DB is inherently destructive; documenting that Lockplane may wipe the database if you don't diff/release promptly.
- For developer tools we accept that users already have credentials to shadow infrastructure (they run it). We'll log warnings and provide a `--force` flag if the DB is still reserved when another command needs it.

## Open Questions
- How to handle concurrent reservations (multiple developers per repo)?
- Do we store reservation metadata in `lockplane.toml` or a local lock file?
- Should preparation emit a `lockplane shadow token` to authenticate future actions until release/timeout?

## Milestones
1. Spec CLI commands and reservation model.
2. Implement cleaning + metadata (local state file) + JSON output.
3. Update `plan` to accept `--from shadow` shorthand.
4. Document workflow in README/docs/sqlalchemy.md, including `create_all()` example.
