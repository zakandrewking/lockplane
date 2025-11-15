# Pending Tasks

## Current Sprint

### 1. ✅ Init UX Improvements
- [x] Escape exits on first page
- [x] Show clean "lockplane init cancelled" message
- [x] Committed: 652df1a

### 2. 🏗️ RLS (Row Level Security) Support
**Status**: In Progress

**Problem**:
- `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` is blocked during `apply` (considered dangerous)
- `validate sql` allows it (no error)
- Need to sync validation between `validate sql` and `apply` pipelines

**Tasks**:
- [ ] Find where ALTER TABLE operations get safety classification in apply flow
- [ ] Identify why RLS is considered dangerous
- [ ] Add explicit support for ENABLE/DISABLE ROW LEVEL SECURITY as safe operations
- [ ] Add tests for RLS operations
- [ ] Update validation to recognize RLS statements

**Files to investigate**:
- `internal/validation/validation.go` - Safety level classifications
- `internal/planner/*.go` - Migration planning
- `internal/schema/diff.go` - Schema diffing

### 3. 📝 Add Tests for cmd/ Files
**Status**: Pending

**Goal**: Add test coverage for command files in `cmd/` directory

**Files needing tests**:
- `cmd/init.go`
- `cmd/apply.go`
- `cmd/plan.go`
- `cmd/rollback.go`
- Other command files

**Approach**:
- Create `cmd/*_test.go` files
- Test command initialization
- Test flag parsing
- Test error handling
- Mock database operations where needed

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
