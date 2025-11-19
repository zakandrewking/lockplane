# Pending Tasks

**Status**: Active backlog tracking
**Last Updated**: 2025-11-14
**Maintainer**: Lockplane Core (keep in sync with sprint planning)

## Current Sprint

### 1. ✅ Init UX Improvements
- [x] Escape exits on first page
- [x] Show clean "lockplane init cancelled" message
- [x] Committed: 652df1a

### 2. ✅ RLS (Row Level Security) Support
**Status**: ✅ **Completed** (2025-11-19)

**What Was Implemented**:

✅ **RLS Toggle Support** (ENABLE/DISABLE ROW LEVEL SECURITY):
- Schema diff detection respects RLS changes (`internal/schema/diff.go:221`)
- Validation with `AlterRLSValidator` (safe + reversible)
- Planner emits RLS enable/disable steps
- Rollback generates reverse operations
- Comprehensive test coverage (8 test functions)

✅ **RLS Policy Management** (NEW - completed with multi-schema support):
- Policy introspection from `pg_policy` system catalog
- Policy DDL generation (`CREATE POLICY`, `DROP POLICY`)
- Schema-qualified policy support (`storage.objects`)
- Policy metadata: name, command, roles, USING, WITH CHECK
- Full integration with multi-schema introspection

**Test Coverage**:
- `TestDiffSchemas_DetectsRLSChange` - Diff detection
- `TestAlterRLSValidator_Enable/Disable` - Validation
- `TestValidateSchemaDiff_RLSChange` - Integration
- `TestGeneratePlan_RLSEnable/Disable` - Planner
- `TestGenerateRollback_EnableRLS/DisableRLS` - Rollback

**Deferred**:
- [ ] End-to-end CLI test - Requires database test harness (not yet implemented)

**Documentation**:
- README.md section 7: Multi-Schema Support & RLS
- docs/supabase.md: RLS policy examples
- Design doc: `devdocs/projects/completed/multi-schema-and-policies.md`

### 3. ✅ Add Tests for cmd/ Files
**Status**: Completed

**Goal**: Add test coverage for command files in `cmd/` directory

**What was done**:
- Created comprehensive test coverage for all cmd/ files
- Tests cover command registration, flag parsing, flag types, and command structure
- All tests pass (37 test cases covering 9 test files)
- Tests verify command metadata (Use, Short, Long, Example fields)
- Tests verify flag existence and types (string vs bool)
- Tests verify shorthand flags where applicable

**Test files created**:
- `cmd/root_test.go` - Root command, version, and command registration tests
- `cmd/init_test.go` - Init wizard command, config checking, bootstrap reporting tests
- `cmd/plan_test.go` - Plan generation command and flag tests
- `cmd/apply_test.go` - Apply migration command and flag tests
- `cmd/rollback_test.go` - Rollback command and plan-rollback flag tests
- `cmd/validate_test.go` - Validation command, subcommands, and flag tests
- `cmd/introspect_test.go` - Introspection command and flag tests
- `cmd/convert_test.go` - Conversion command tests
- `cmd/multiphase_test.go` - Multi-phase migration command tests (plan-multiphase, apply-phase, rollback-phase, phase-status)

**Future improvements** (nice to have, not blocking):
- [ ] Add behavioral tests for `RunE` code paths using in-memory FS + fake executors
- [ ] Exercise error handling (missing flags, invalid schema paths, driver resolution failures)
- [ ] Verify flag default wiring with integration tests
- [ ] Use lightweight test helpers to stub `config.LoadConfig`, `executor.LoadSchema`, etc.

### 4. 📁 Supabase Schema Directory Support
**Status**: ✅ Completed

**Goal**: Support Supabase workflow where schema lives in `supabase/schema/`

**User Workflow**:
1. User puts schema in `supabase/schema/` directory
2. Supabase starts and runs the schema
3. User runs `lockplane apply` pointing to `supabase/schema/` to keep it updated

**Tasks**:
- [x] Update `lockplane init` to offer Supabase preset (`lockplane init --supabase --yes`)
- [x] Support `--schema supabase/schema` by wiring `--schema-path` through and preferring `supabase/schema/`
- [x] Add Supabase workflow documentation (README + docs/supabase*.md)
- [ ] Test with actual Supabase project
- [x] Ensure `config.ResolveEnvironment` understands Supabase defaults (shadow DB, database URL, schema path)
- [x] Verify `cmd/apply.go` auto-detection checks `supabase/schema/` before falling back to `schema/`

**Implementation Outline**:
- Extend `cmd/init.go` with the `--supabase` preset flag (non-interactive flow) and scaffold `.env.supabase` plus `schema_path = "supabase/schema"`.
- Update docs to explain the preset and new auto-detection behavior.
- Integration test: **still pending** (run `lockplane plan` against a sample Supabase repo in `examples/supabase/` to ensure schema loading works).

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
