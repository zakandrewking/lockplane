# Code Organization - Move to Package Structure

## Progress Checklist

- [ ] Phase 1: Planning and design
- [ ] Phase 2: Create new directory structure
- [ ] Phase 3: Move files and update imports
- [ ] Phase 4: Update build and test configuration
- [ ] Phase 5: Update documentation
- [ ] Phase 6: Verify and test

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

### Phase 1: Planning (this document)

- [x] Analyze current structure
- [x] Design new structure
- [x] Document design decisions
- [ ] Review with team (if applicable)

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
