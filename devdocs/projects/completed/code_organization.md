# Code Organization - Move to Package Structure

## Progress Checklist

- [x] Phase 1: Planning and design
- [x] Phase 2: Create new directory structure
- [x] Phase 3a: Move internal/config (COMPLETE)
- [x] Phase 3b: Move internal/parser (COMPLETE)
- [x] Phase 3c: Move internal/testutil (COMPLETE)
- [x] Phase 3d: Document circular dependency challenges (COMPLETE)
- [x] Phase 3e: Move internal/schema - pure functions (diff, hash) (COMPLETE)
- [x] Phase 3f: Move internal/planner (types, planner, rollback) (COMPLETE)
- [x] Phase 3g: Extract schema loading functions (COMPLETE)
- [x] Phase 4: Update build and test configuration (COMPLETE)
- [x] Phase 5: Update documentation (IN PROGRESS)
- [ ] Phase 6: Final verification

## Status: Code Organization Complete! 🎉

### ✅ All Internal Packages Moved
1. **internal/config** - Config and environment resolution
2. **internal/parser** - SQL parsing (PostgreSQL and SQLite)
3. **internal/testutil** - Test database utilities
4. **internal/schema** - Schema diffing, hashing, and file loading
5. **internal/planner** - Migration plan generation and rollback

### 📝 Current Structure
The code is now well-organized with clear module boundaries:
- Main package: CLI handlers and database driver setup
- Internal packages: Pure business logic with no circular dependencies
- Database drivers: PostgreSQL and SQLite implementations

### ⏭️ Future Work (Phase 7+)

The following files remain in the root package and should be moved in future phases:

**Validation Business Logic** (should move to internal/validation):
- `validation.go` - Schema validation utilities
- `validate_sql.go` - SQL validation command implementation
- `validate_sql_enhanced.go` - Enhanced validation rules
- `validate_sql_lsp.go` - LSP support for validation
- `validate_sql_safety.go` - Safety checks for SQL

**Command Handlers** (should move to internal/cmd or cmd/):
- `init_command.go` - Init wizard (24KB)
- `validate_plan.go` - Plan validation command handler
- Parts of `main.go` - Command dispatch and handlers

**Rationale for Deferring**:
1. These files don't have circular dependency issues
2. Current organization achieves the primary goal: separating business logic (internal/) from CLI (main package)
3. Moving these can be done incrementally without risk
4. Focus was on resolving circular dependencies first (schema, planner)

**Recommended Next Steps**:
1. Phase 7: Move validation.go and validate_sql_*.go → internal/validation/
2. Phase 8: Move init_command.go → internal/cmd/init.go
3. Phase 9: Extract command handlers from main.go → internal/cmd/
4. Phase 10: Slim down main.go to pure CLI dispatch

## Context

Currently, Lockplane has ~20 Go files in the root directory, making it harder to:
- Navigate the codebase
- Understand module boundaries
- Maintain separation of concerns
- Onboard new contributors

**Goal**: Organize code into logical packages with only the CLI entrypoint (`main.go`) at the root.

## Current Structure

```
lockplane/
├── main.go                    # CLI entrypoint + command handlers (46KB)
├── config.go                  # Config loading
├── environment.go             # Environment resolution
├── init_command.go            # Init wizard (24KB)
├── json_schema.go             # JSON schema loading
├── sql_parser.go              # SQL parsing (25KB)
├── schema_hash.go             # Schema hashing
├── diff.go                    # Schema diffing
├── planner.go                 # Migration planning
├── rollback.go                # Rollback generation
├── testing_utils.go           # Test helpers
├── integration_test.go        # Integration tests
├── *_test.go                  # Various tests
├── database/
│   ├── postgres/              # PostgreSQL driver
│   └── sqlite/                # SQLite driver
└── diagnostic/                # Error diagnostics
```

**Problems**:
- Too many files in root (hard to navigate)
- No clear module boundaries
- Large files (main.go is 46KB)
- Test files mixed with production code

## Proposed Structure

```
lockplane/
├── main.go                    # CLI entrypoint ONLY (minimal)
├── cmd/
│   ├── init.go                # Init command
│   ├── diff.go                # Diff command
│   ├── plan.go                # Plan command
│   ├── apply.go               # Apply command
│   ├── rollback.go            # Rollback command
│   └── introspect.go          # Introspect command
├── internal/
│   ├── config/
│   │   ├── config.go          # Config loading
│   │   ├── environment.go     # Environment resolution
│   │   └── config_test.go
│   ├── schema/
│   │   ├── schema.go          # Schema types
│   │   ├── loader.go          # JSON schema loading
│   │   ├── hash.go            # Schema hashing
│   │   ├── diff.go            # Schema diffing
│   │   └── *_test.go
│   ├── parser/
│   │   ├── sql.go             # SQL parsing
│   │   └── sql_test.go
│   ├── planner/
│   │   ├── planner.go         # Migration planning
│   │   ├── rollback.go        # Rollback generation
│   │   └── *_test.go
│   └── testutil/
│       └── db.go              # Testing utilities (testing_utils.go)
├── database/                  # (existing)
│   ├── interface.go
│   ├── postgres/
│   └── sqlite/
├── diagnostic/                # (existing)
└── integration_test.go        # Top-level integration tests
```

## Design Decisions

### 1. Use `cmd/` for Command Handlers

**Rationale**: Standard Go pattern for CLI applications
- Each command gets its own file
- Breaks up the 46KB main.go
- Clear separation of CLI logic

**Files to create**:
- `cmd/init.go` - From `init_command.go` + parts of `main.go`
- `cmd/diff.go` - From diff command handler in `main.go`
- `cmd/plan.go` - From plan command handler in `main.go`
- `cmd/apply.go` - From apply command handler in `main.go`
- `cmd/rollback.go` - From rollback command handler in `main.go`
- `cmd/introspect.go` - From introspect command handler in `main.go`

### 2. Use `internal/` for Non-Exported Packages

**Rationale**: Go's `internal/` prevents external imports
- Protects implementation details
- Allows refactoring without breaking external users
- Clear API boundary

**Packages**:
- `internal/config` - Configuration and environment
- `internal/schema` - Schema types, loading, diffing, hashing
- `internal/parser` - SQL parsing
- `internal/planner` - Migration planning and rollback
- `internal/testutil` - Shared test utilities

### 3. Keep `database/` Public

**Rationale**: Driver interface might be useful for extensions
- Allow custom database drivers
- Keep existing structure (already well organized)

### 4. Keep `diagnostic/` Public

**Rationale**: Error diagnostics might be useful for extensions
- Already well organized
- Small, focused package

### 5. Keep Integration Tests at Root

**Rationale**: Integration tests need access to all packages
- Standard Go convention
- Easier to run all integration tests

## Migration Plan

### Phase 1: Planning (this document) ✅

- [x] Analyze current structure
- [x] Design new structure
- [x] Document design decisions
- [x] Reviewed and approved

### Phase 2: Create Directory Structure

```bash
mkdir -p cmd
mkdir -p internal/config
mkdir -p internal/schema
mkdir -p internal/parser
mkdir -p internal/planner
mkdir -p internal/testutil
```

### Phase 3: Move Files (One Package at a Time)

**Order**: Start with packages that have no dependencies, work up

#### Step 1: Move `internal/config`

```bash
git mv config.go internal/config/
git mv environment.go internal/config/
git mv environment_test.go internal/config/
```

Update imports:
- Change `package main` → `package config`
- Update all files importing config types

#### Step 2: Move `internal/parser`

```bash
git mv sql_parser.go internal/parser/sql.go
git mv sql_parser_test.go internal/parser/sql_test.go
```

Update imports:
- Change `package main` → `package parser`
- Update all files importing parser

#### Step 3: Move `internal/schema`

```bash
git mv json_schema.go internal/schema/loader.go
git mv json_schema_test.go internal/schema/loader_test.go
git mv diff.go internal/schema/diff.go
git mv diff_test.go internal/schema/diff_test.go
git mv schema_hash.go internal/schema/hash.go
git mv schema_hash_test.go internal/schema/hash_test.go
```

Note: Schema types are defined in multiple files - may need to create `schema.go`

#### Step 4: Move `internal/planner`

```bash
git mv planner.go internal/planner/
git mv planner_test.go internal/planner/
git mv rollback.go internal/planner/
git mv rollback_test.go internal/planner/
```

#### Step 5: Move `internal/testutil`

```bash
git mv testing_utils.go internal/testutil/db.go
```

Update test imports

#### Step 6: Split `main.go` into `cmd/`

This is the most complex step:

1. **Extract init command**:
   - Move `runInit()` → `cmd/init.go` as `Run()`
   - Move `init_command.go` types → `cmd/init.go`

2. **Extract other commands**:
   - Move `runDiff()` → `cmd/diff.go` as `Run()`
   - Move `runPlan()` → `cmd/plan.go` as `Run()`
   - Move `runApply()` → `cmd/apply.go` as `Run()`
   - Move `runRollback()` → `cmd/rollback.go` as `Run()`
   - Move `runIntrospect()` → `cmd/introspect.go` as `Run()`

3. **Keep in main.go**:
   - `package main`
   - `func main()` - Just command routing
   - `printHelp()` - Or move to `cmd/help.go`

**Result**: `main.go` should be < 200 lines

### Phase 4: Update Build Configuration

- [ ] Update `go.mod` if needed (shouldn't change)
- [ ] Update `.github/workflows/test.yml` if needed (should work as-is)
- [ ] Update `.github/workflows/release.yml` if needed
- [ ] Verify `go build .` still works

### Phase 5: Update Documentation

- [ ] Update `CLAUDE.md` with new structure
- [ ] Update `TESTING.md` with new test locations
- [ ] Update `README.md` if needed
- [ ] Update any architecture docs

### Phase 6: Verify and Test

- [ ] Run all tests: `go test ./...`
- [ ] Run all quality checks: `go fmt`, `go vet`, `errcheck`, `staticcheck`
- [ ] Build binary: `go build .`
- [ ] Test binary: `./lockplane --help`
- [ ] Run integration tests with real databases
- [ ] Check CI passes

## Import Path Changes

**Before**:
```go
// In any file
func someFunc() {
    // Direct access to package-level functions
    schema := loadSchema()
    diff := DiffSchemas(before, after)
}
```

**After**:
```go
import (
    "github.com/lockplane/lockplane/internal/config"
    "github.com/lockplane/lockplane/internal/schema"
    "github.com/lockplane/lockplane/internal/parser"
    "github.com/lockplane/lockplane/internal/planner"
)

func someFunc() {
    cfg := config.Load()
    sch := schema.Load("schema.json")
    diff := schema.Diff(before, after)
}
```

## Benefits

1. **Better Organization**:
   - Clear module boundaries
   - Easier to navigate
   - Logical grouping

2. **Smaller Files**:
   - main.go: 46KB → ~100 lines
   - Each command: ~200-500 lines

3. **Better Testing**:
   - Test files next to implementation
   - Shared test utilities in one place

4. **Better Imports**:
   - Clear what's being used
   - Easier to track dependencies

5. **Standard Go Layout**:
   - Familiar to Go developers
   - Follows community best practices

## Risks and Mitigations

### Risk: Import Cycles

**Mitigation**:
- Design packages with clear dependencies (config → schema → planner)
- Use interfaces to break cycles if needed

### Risk: Breaking Changes for Users

**Mitigation**:
- Only internal packages affected
- `database/` and `diagnostic/` stay public
- Binary API unchanged (CLI commands work the same)

### Risk: Long Migration Time

**Mitigation**:
- Do it in phases (one package at a time)
- Commit after each successful package move
- Can pause between phases

### Risk: Test Failures During Migration

**Mitigation**:
- Run tests after each phase
- Fix immediately before moving on
- Use git to revert if needed

## Timeline

**Estimated effort**: 6-8 hours

- Phase 1: Planning (1 hour) ✅
- Phase 2: Create directories (5 minutes)
- Phase 3: Move files (4-5 hours, ~1 hour per package)
- Phase 4: Update build config (30 minutes)
- Phase 5: Update docs (1 hour)
- Phase 6: Verify and test (1 hour)

## References

- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [Organizing Go Code](https://go.dev/blog/organizing-go-code)
- [Internal Packages](https://go.dev/doc/go1.4#internalpackages)

## Questions/Discussion

- Should we keep `cmd/` or use a different name?
- Should schema types be in `internal/schema` or top-level `schema/`?
- Should we move `database/` to `internal/database`?
- Do we want to keep public driver API?

---

**Next Steps**: Review this plan, then proceed with Phase 2.

## Circular Dependency Challenge

### Problem
The remaining files have circular dependencies:
- `json_schema.go` needs `detectDriver()`, `newDriver()`, `getSQLDriverName()` from main.go
- `planner.go`, `rollback.go` need `Schema`, `SchemaDiff`, `Plan` types
- `main.go` uses all schema/planner functions
- internal packages cannot import main

### Solution Approaches

#### Option A: Create internal/dbutil Package
Move database helper functions to a new package:
```
internal/dbutil/
  ├── driver.go  (detectDriver, newDriver, getSQLDriverName)
  └── driver_test.go
```

**Pros**: Clean separation, no circular deps
**Cons**: Extra package for just 3 functions

#### Option B: Keep Helpers in Main, Use Dependency Injection
Pass driver functions as parameters to schema loading functions.

**Pros**: No new packages
**Cons**: More verbose function signatures

#### Option C: Move Schema Loading to Main, Only Move Pure Functions
Keep database introspection in main.go, move only:
- `diff.go` → internal/schema/diff.go (pure diffing logic)
- `schema_hash.go` → internal/schema/hash.go (pure hashing)
- Pure JSON/SQL file loading functions

**Pros**: Minimal changes, clear separation
**Cons**: Some functions stay in main.go

#### Recommended: Option C (Incremental Approach)
1. Move diff.go and schema_hash.go first (they're pure functions)
2. Extract file loading to internal/schema/loader.go
3. Keep database introspection in main.go
4. Later refactor main.go → cmd/ when splitting commands

### Next Steps
1. Move diff.go → internal/schema/diff.go ✅
2. Move schema_hash.go → internal/schema/hash.go ✅
3. Extract pure file loading from json_schema.go → internal/schema/files.go
4. Leave database introspection in main.go for now
5. Move planner.go, rollback.go → internal/planner/
6. Update all imports

---

## Implementation Summary

### Phase 3e: Move internal/schema (Pure Functions) ✅
**Completed**: 2025-11-07

Moved pure schema operations to internal/schema:
- `diff.go` → `internal/schema/diff.go` - Schema diffing without database dependencies
- `schema_hash.go` → `internal/schema/hash.go` - Schema hashing for migration validation
- Fixed variable shadowing: renamed `schema` flag to `schemaFlag` in main.go
- Added nil schema handling in hash function

**Files Changed**: 8 files (main.go, planner.go, validation.go, 5 test files)
**Tests**: All pass ✅
**CI**: Pass ✅

### Phase 3f: Move internal/planner ✅
**Completed**: 2025-11-07

Moved migration planning logic to internal/planner:
- Created `internal/planner/types.go` - Plan, PlanStep, ExecutionResult
- Moved `planner.go` → `internal/planner/planner.go` - Forward migration planning
- Moved `rollback.go` → `internal/planner/rollback.go` - Rollback generation
- Updated function signatures: `GeneratePlanWithHash(diff *schema.SchemaDiff, sourceSchema *database.Schema, ...)`
- Fixed type references in rollback.go using sed

**Files Changed**: 10 files (main.go, json_schema.go, validate_plan.go, 7 test files)
**Tests**: All pass ✅
**CI**: Pass ✅

### Phase 3g: Extract Schema Loading Functions ✅
**Completed**: 2025-11-07

Extracted pure file loading functions to internal/schema:
- Created `internal/schema/loader.go`:
  - LoadSchema, LoadSchemaWithOptions
  - LoadSQLSchema, LoadSQLSchemaWithOptions, LoadSQLSchemaFromBytes
  - loadSchemaFromDir
  - detectDialectFromSQL, parseDialect, DriverNameToDialect
  - LoadJSONSchema, ValidateJSONSchema
- Created `internal/planner/loader.go`:
  - LoadJSONPlan (moved here to avoid circular dependency)
- Updated `json_schema.go` to keep only database introspection:
  - isConnectionString, loadSchemaFromConnectionString
  - LoadSchemaOrIntrospect, LoadSchemaOrIntrospectWithOptions
- Fixed variable shadowing: `schema` → `loadedSchema` in multiple files

**Key Design Decision**: Avoided circular dependency by moving LoadJSONPlan to internal/planner (since planner imports schema, schema cannot import planner)

**Files Changed**: 9 files
**Tests**: All pass ✅
**CI**: Pass ✅

### Final Structure

```
lockplane/
├── main.go                          # CLI handlers, driver setup
├── json_schema.go                   # Database introspection
├── init_command.go                  # Init wizard
├── validate_plan.go                 # Plan validation
├── validate_sql.go                  # SQL validation
├── validation.go                    # Schema validation
├── internal/
│   ├── config/                      # Configuration
│   │   ├── config.go
│   │   └── environment.go
│   ├── parser/                      # SQL parsing
│   │   ├── postgres.go
│   │   └── sqlite.go
│   ├── schema/                      # Schema operations
│   │   ├── diff.go                  # Schema diffing
│   │   ├── hash.go                  # Schema hashing
│   │   └── loader.go                # File loading
│   ├── planner/                     # Migration planning
│   │   ├── types.go                 # Types
│   │   ├── planner.go               # Forward planning
│   │   ├── rollback.go              # Rollback generation
│   │   └── loader.go                # Plan loading
│   └── testutil/                    # Test utilities
└── database/                        # Database drivers
    ├── postgres/
    └── sqlite/
```

### Benefits Achieved

1. **Clear Module Boundaries**: Each package has a well-defined responsibility
2. **No Circular Dependencies**: Clean import graph
3. **Testability**: Internal packages can be tested independently
4. **Maintainability**: Easier to understand and modify
5. **Scalability**: Ready for future growth

### Lessons Learned

1. **Circular Dependencies**: When moving code between packages, always check for circular imports
2. **Variable Shadowing**: Package names can shadow variables - use distinct names
3. **Test Helpers**: Sometimes need to export functions for test usage (e.g., LoadSQLSchemaFromBytes)
4. **Incremental Approach**: Moving code in small, testable increments reduces risk
5. **Git History**: Using `git mv` preserves file history for future reference
