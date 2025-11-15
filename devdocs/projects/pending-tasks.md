# Pending Tasks

**Status**: Active backlog tracking
**Last Updated**: 2025-11-14
**Maintainer**: Lockplane Core (keep in sync with sprint planning)

## Current Sprint

### 1. ✅ Init UX Improvements
- [x] Escape exits on first page
- [x] Show clean "lockplane init cancelled" message
- [x] Committed: 652df1a

### 2. 🏗️ RLS (Row Level Security) Support
**Status**: In Progress (blocker: validation rejects statements)

**Problem**:
- `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` is blocked during `apply` (classified as dangerous)
- `validate sql` allows it (no error), so CLI behaviour is inconsistent
- Need to sync validation between `validate sql`, `plan`, and `apply` pipelines

**Recent Findings**:
- `internal/schema/diff.go` detects RLS changes, but `TableDiff.IsEmpty()` ignores `RLSChanged`, so pure RLS diffs are dropped before planning.
- `internal/planner/planner.go:214-230` and `internal/planner/rollback.go:63-95` already know how to emit/undo the SQL when the diff survives.
- `apply` safety reporting ultimately uses `internal/validation/validation.go` which currently has no validator for `ENABLE/DISABLE ROW LEVEL SECURITY`, so arbitrary statements default to `SafetyLevelDangerous`.

**Tasks**:
- [ ] Extend `TableDiff.IsEmpty()` to treat `RLSChanged` as a modification so RLS-only diffs reach the planner.
- [ ] Introduce a validator (e.g. `AlterRLSValidator`) that classifies enable/disable operations as safe and reversible (rolls back to opposite statement).
- [ ] Ensure `validation.ValidateSchemaDiff{,WithSchema}` surface the new safety info so `plan` output shows ✅/⚠️ for the RLS steps.
- [ ] Wire the validator into `apply` safety gates so RLS statements stop blocking deployments.
- [ ] Add regression tests:
  - Schema diff unit test proving RLS-only tables produce a modified table entry.
  - Planner/rollback tests ensuring ENABLE/DISABLE steps are generated and reversed.
  - Validation tests covering new validator + ensuring `SafetyLevelSafe` classification.
  - End-to-end CLI test (once DB harness exists) that runs `lockplane plan` and `lockplane apply` for an RLS toggle.

**Files to investigate** (with anchors):
- `internal/schema/diff.go:212-226` – `TableDiff.IsEmpty` update.
- `internal/planner/planner.go:214-230` – Step emission.
- `internal/planner/rollback.go:63-95`, `361-388` – Reverse operation helpers.
- `internal/validation/validation.go` – add validator + include in dispatcher.
- `cmd/apply.go` / `cmd/plan.go` – ensure CLI messaging reflects new safety level.

### 3. 📝 Add Tests for cmd/ Files
**Status**: In Progress (initial scaffolding merged locally)

**Goal**: Add test coverage for command files in `cmd/` directory

**Current Coverage**:
- Lightweight flag/metadata tests exist for `apply`, `init`, `plan`, `rollback`, and `root` (`cmd/*_test.go` stubs currently staged but not committed).
- No behavioural tests execute command logic or verify interactions with executor/config packages.

**Next Steps**:
- [ ] Flesh out existing tests to cover `RunE` code paths using in-memory FS + fake executors.
- [ ] Add coverage for remaining commands (`apply_phase.go`, `plan_multiphase.go`, `rollback_phase.go`, `convert.go`, `validate.go`, `introspect.go`, `phase_status.go`).
- [ ] Exercise error handling (missing flags, invalid schema paths, driver resolution failures).
- [ ] Verify flag default wiring (e.g. `--auto-approve`, `--skip-shadow`, `--schema`, Supabase preset once added).
- [ ] Use a lightweight test helper to stub `config.LoadConfig`, `executor.LoadSchema`, etc., so CLI logic can be executed without hitting disks or DBs.

**Implementation Ideas**:
- Introduce `cmd/testhelpers` with small interfaces for config/executor to allow dependency injection during tests.
- Consider reorganizing cobra command factories to `newApplyCmd(deps)` style to simplify stateful flag testing.
- Capture cobra output via `bytes.Buffer` to assert on user messaging (warnings, errors, success logs).

### 4. 📁 Supabase Schema Directory Support
**Status**: Pending

**Goal**: Support Supabase workflow where schema lives in `supabase/schema/`

**User Workflow**:
1. User puts schema in `supabase/schema/` directory
2. Supabase starts and runs the schema
3. User runs `lockplane apply` pointing to `supabase/schema/` to keep it updated

**Tasks**:
- [ ] Update `lockplane init` to offer Supabase preset
- [ ] Support `--schema supabase/schema` flag
- [ ] Add Supabase workflow documentation
- [ ] Test with actual Supabase project
- [ ] Ensure `config.ResolveEnvironment` understands Supabase defaults (shadow DB, database URL, schema path)
- [ ] Verify `cmd/apply.go` auto-detection checks `supabase/schema/` before falling back to `schema/`

**Implementation Outline**:
- Extend `cmd/init.go` prompt/flags to add "Supabase" template that scaffolds `supabase/schema/` and `.env.local` entries using Supabase CLI defaults.
- Update `config.LoadConfig` + docs to explain how `schema_path` resolves when Supabase preset is chosen.
- Teach `cmd/root.go` help text and `printHelp()` (in `main.go`) about the new preset/flag combination.
- Add Supabase-specific examples to `README.md`, `docs/getting_started.md`, and `docs/supabase.md` covering `lockplane apply --schema supabase/schema --target-environment supabase`.
- Integration test: run `lockplane plan` against a sample Supabase repo in `examples/supabase/` to ensure schema loading works.

## Completed Recently

- ✅ npm package support (`npx lockplane`) - v0.6.4
- ✅ Documentation updated to use npx as default
- ✅ Lock analysis infrastructure (Phases 1-4)
  - Lock type definitions
  - Lock-safe rewrites
  - Shadow DB measurement

## Notes

- All work should follow the checklist in CLAUDE.md
- Run tests, quality checks, commit, and push for each completed task
- Update this file as tasks progress
