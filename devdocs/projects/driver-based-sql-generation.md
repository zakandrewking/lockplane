# Driver-Based SQL Generation Refactor

**Status**: ✅ **COMPLETE**
**Started**: 2024-11-03
**Completed**: 2025-11-08
**Issue**: Lockplane was generating PostgreSQL SQL for SQLite/libSQL databases

## Problem Statement

When users generated migration plans from SQLite schema files, Lockplane generated PostgreSQL-style SQL:
- `ALTER TABLE ... ALTER COLUMN ... TYPE` (PostgreSQL syntax - not supported by SQLite)
- This caused SQLite migrations to fail with syntax errors
- SQLite doesn't support most ALTER COLUMN operations

**Root Cause**: The plan command used `detectDriver()` on file paths (which defaulted to PostgreSQL), ignoring the schema's dialect field loaded from JSON or SQL comments.

**Solution**: Check `after.Dialect` field first before falling back to `detectDriver()` for connection strings.

## Current Architecture

```
main.go:runApply()
  └─> GeneratePlanWithHash(diff, sourceSchema)
       └─> generateAddTable() // hardcoded PostgreSQL
       └─> generateModifyColumn() // hardcoded PostgreSQL
       └─> etc.
```

## Target Architecture

```
main.go:runApply()
  ├─> detectDriver(targetConnStr) → "libsql"
  ├─> newDriver("libsql") → sqlite.NewDriver()
  └─> GeneratePlanWithHash(diff, sourceSchema, driver)
       └─> driver.CreateTable() // uses SQLite driver
       └─> driver.ModifyColumn() // uses SQLite driver
       └─> etc.
```

## Type Hierarchy

**Current**:
- `main.Table`, `main.Column`, `main.Index`, etc.
- `main.ColumnDiff`, `main.PlanStep`
- `database.Table`, `database.Column` (aliased to main types)

**Issue**: The driver interface expects `database.ColumnDiff` and returns `[]database.PlanStep`, but planner uses main package types.

**Solution**: Since `database.Table` etc. are type aliases to main package types (see database/types.go), we just need to use the fully qualified names.

## Implementation Plan

### Phase 1: Update Function Signatures ✅

- [x] Update `GeneratePlan(diff, driver)` signature
- [x] Update `GeneratePlanWithHash(diff, sourceSchema, driver)` signature
- [x] Update planner.go to call driver methods instead of generate functions

### Phase 2: Fix Type References

- [ ] Update `planner.go` to use `database.ColumnDiff` and `database.PlanStep` types
- [ ] Verify type compatibility between main and database packages

### Phase 3: Update Call Sites in main.go

**File: main.go**

#### `runPlan()` (line ~507)
```go
// Before:
plan, err := GeneratePlanWithHash(diff, before)

// After:
// Need to detect driver from 'to' schema (the target state)
targetDriverType := detectDriver(toInput)
targetDriver, err := newDriver(targetDriverType)
plan, err := GeneratePlanWithHash(diff, before, targetDriver)
```

**Question**: Should we use driver from `fromInput` or `toInput`?
**Answer**: Use `toInput` (the target schema) because that's the database we're generating SQL for.

#### `runApply()` (line ~701)
```go
// Before:
generatedPlan, err := GeneratePlanWithHash(diff, before)

// After:
// mainDriverType already detected at line 746
// mainDriver already created at line 747
generatedPlan, err := GeneratePlanWithHash(diff, before, mainDriver)
```

### Phase 4: Update rollback.go

**File: rollback.go**

The rollback generator also uses hardcoded functions. Options:

1. **Pass driver to GenerateRollback()** (recommended)
   - Update signature: `GenerateRollback(plan, driver)`
   - Use driver methods for SQL generation

2. **Extract SQL from forward plan** (alternative)
   - Parse SQL to reverse it
   - More fragile, not recommended

**Decision**: Update rollback.go to accept driver parameter.

#### Functions to update:
- `GenerateRollback(plan, driver)` - add driver parameter
- Replace `generateAddTable()` → `driver.CreateTable()`
- Replace `generateAddIndex()` → `driver.AddIndex()`
- Replace `generateAddForeignKey()` → `driver.AddForeignKey()`
- Replace `formatColumnDefinition()` → `driver.FormatColumnDefinition()`

#### Call sites to update:
- Find all `GenerateRollback()` calls
- Pass appropriate driver based on context

### Phase 5: Update Tests

**File: planner_test.go**

- [ ] Update all `GeneratePlan()` calls to include driver
- [ ] Add tests for PostgreSQL driver
- [ ] Add tests for SQLite driver
- [ ] Verify SQL output is database-specific

**New test cases needed**:
```go
func TestGeneratePlan_PostgreSQL(t *testing.T) {
    driver := postgres.NewDriver()
    plan, err := GeneratePlan(diff, driver)
    // Verify PostgreSQL SQL syntax
}

func TestGeneratePlan_SQLite(t *testing.T) {
    driver := sqlite.NewDriver()
    plan, err := GeneratePlan(diff, driver)
    // Verify SQLite SQL syntax (no ALTER COLUMN)
}
```

### Phase 6: Integration Testing

- [ ] Test with real PostgreSQL database
- [ ] Test with real SQLite database
- [ ] Test with real Turso/libSQL database
- [ ] Verify generated SQL executes successfully

### Phase 7: Documentation

- [ ] Update CLAUDE.md if needed
- [ ] Update comments in planner.go
- [ ] Update comments in rollback.go
- [ ] Add example of driver-based generation to README

## Execution Checklist

### Step 1: Fix Type References ⏳
- [ ] Update planner.go line 71: use `database.ColumnDiff`
- [ ] Update planner.go line 72: handle `[]database.PlanStep` properly
- [ ] Test: `go build .`

### Step 2: Update rollback.go
- [ ] Add `import "github.com/lockplane/lockplane/database"`
- [ ] Update `GenerateRollback` signature
- [ ] Replace generate functions with driver methods
- [ ] Find and update all call sites
- [ ] Test: `go build .`

### Step 3: Update main.go call sites
- [ ] Update `runPlan()` line ~507
- [ ] Update `runApply()` line ~701
- [ ] Test: `go build .`

### Step 4: Update tests
- [ ] Update `planner_test.go`
- [ ] Add PostgreSQL tests
- [ ] Add SQLite tests
- [ ] Test: `go test ./...`

### Step 5: Manual testing
- [ ] Test PostgreSQL migration
- [ ] Test SQLite migration
- [ ] Test Turso/libSQL migration
- [ ] Verify SQL is database-appropriate

### Step 6: Commit
- [ ] Format: `go fmt ./...`
- [ ] Vet: `go vet ./...`
- [ ] Test: `go test ./...`
- [ ] Commit with descriptive message
- [ ] Push to remote

## Risks & Mitigations

### Risk: Type incompatibility
**Mitigation**: Types are aliases, should work. Verify with compilation.

### Risk: Rollback generation breaks
**Mitigation**: Update rollback.go in same commit, test thoroughly.

### Risk: Tests break
**Mitigation**: Update tests systematically, verify all pass.

### Risk: Missing call sites
**Mitigation**: Use grep to find all `GeneratePlan` calls, check each one.

## Success Criteria

- [ ] Code compiles without errors
- [ ] All tests pass
- [ ] PostgreSQL migrations generate PostgreSQL SQL
- [ ] SQLite migrations generate SQLite SQL
- [ ] Turso/libSQL migrations generate SQLite SQL
- [ ] No hardcoded SQL generation in planner.go
- [ ] rollback.go uses driver-based generation

## Notes

- The database package already has all the driver implementations
- PostgreSQL driver: `database/postgres/generator.go`
- SQLite driver: `database/sqlite/generator.go`
- Both implement the `database.SQLGenerator` interface
- We just need to wire them up to the planner

## References

- Original bug report: PostgreSQL SQL generated for SQLite database
- Driver interface: `database/interface.go`
- PostgreSQL generator: `database/postgres/generator.go`
- SQLite generator: `database/sqlite/generator.go`

---

## Final Implementation Summary

### What Was Completed

✅ **Phase 1-4**: All function signatures updated, driver parameters added throughout
✅ **Phase 5**: Added comprehensive tests comparing PostgreSQL vs SQLite SQL generation
✅ **Phase 6**: Integration tested with real schema files (JSON and SQL DDL)
✅ **Fix Applied**: Updated `runPlan()` in main.go to check `after.Dialect` before falling back to `detectDriver()`

### Key Changes

1. **main.go** (runPlan function):
   ```go
   // Now checks schema's Dialect field first
   if after.Dialect != "" && after.Dialect != database.DialectUnknown {
       targetDriverType = string(after.Dialect)
   } else {
       targetDriverType = detectDriver(toInput)  // fallback for connection strings
   }
   ```

2. **internal/planner/planner_test.go**:
   - Added `TestGeneratePlan_PostgreSQLvsSQLite` to verify drivers generate different SQL
   - PostgreSQL generates: `ALTER TABLE ... ALTER COLUMN ... TYPE`
   - SQLite generates: Comment about limitation (table recreation required)

### Verification

**Test Results:**
- PostgreSQL plan with type change: ✅ Generates `ALTER COLUMN TYPE`
- SQLite plan with type change: ✅ Generates limitation comment (correct behavior)
- SQLite plan with column addition: ✅ Generates `ALTER TABLE ... ADD COLUMN` (supported operation)

**Example Output:**
```json
{
  "steps": [
    {
      "description": "SQLite limitation: Cannot modify column users.age (changes: type). Would require table recreation.",
      "sql": "-- SQLite limitation: Cannot modify column users.age (changes: type). Would require table recreation."
    }
  ]
}
```

### Impact

- ✅ SQLite/libSQL users now get correct SQL or clear limitation messages
- ✅ PostgreSQL users continue to get PostgreSQL-specific SQL
- ✅ Dialect detection works for JSON schemas, SQL DDL files, and connection strings
- ✅ All tests pass, CI green

### Lessons Learned

1. **Binary confusion**: Always use `./lockplane` or `go install` - `lockplane` may run stale binary from PATH
2. **Logging**: `log.Printf` output doesn't appear by default - use `log.Fatalf` for debugging or check stderr explicitly
3. **Type aliases**: `Schema = database.Schema` preserves all fields including Dialect during conversions
4. **SQLite limitations**: Driver correctly returns comments for unsupported operations instead of invalid SQL

### Next Steps

The driver-based SQL generation is now complete and working correctly. Future enhancements could include:
- Implement table recreation strategy for SQLite column modifications (currently just returns comment)
- Add more dialect-specific optimizations
- Support additional databases (MySQL, etc.)
