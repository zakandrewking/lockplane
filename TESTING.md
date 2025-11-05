# Testing Guide

This document describes how to run and write tests for Lockplane.

## Quick Start

### Run All Tests

```bash
# Run unit tests only (no database required)
go test -short ./...

# Run all tests including SQLite integration tests
TEST_ALL_DRIVERS=true go test ./...

# For PostgreSQL integration tests, set up PostgreSQL manually or rely on CI
```

### Run Unit Tests Only (no database required)

```bash
go test -short ./...
```

## Test Organization

Lockplane uses two types of tests:

### Unit Tests

**Fast, no database required**

- Driver interface tests (`database/*/driver_test.go`)
- SQL generation tests (`database/*/generator_test.go`)
- Business logic tests (`planner_test.go`, `rollback_test.go`, `diff_test.go`)
- Schema parsing tests

Unit tests run in milliseconds and don't require external dependencies.

### Integration Tests

**Require live database connection**

- End-to-end workflows (`main_test.go`)
- Database introspection (`database/*/introspector_test.go`)
- Migration execution and rollback

Integration tests verify that Lockplane works correctly with real databases.

## Environment Variables

Control test behavior with these environment variables:

| Variable | Purpose | Default |
|----------|---------|---------|
| `REQUIRE_TEST_DB` | Fail tests if database unavailable (use in CI) | `false` |
| `TEST_ALL_DRIVERS` | Test PostgreSQL + SQLite + libSQL | `false` (PostgreSQL only) |
| `SKIP_INTEGRATION` | Skip all integration tests | `false` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable` |
| `SHADOW_DATABASE_URL` | Shadow database connection string | `postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable` |
| `POSTGRES_TEST_URL` | Override PostgreSQL test database | (uses `DATABASE_URL`) |
| `LIBSQL_TEST_URL` | libSQL/Turso connection string | `file::memory:?cache=shared` |

## Running Tests

### Local Development

```bash
# Quick test (unit tests only)
go test -short ./...

# Full test suite with SQLite integration tests
TEST_ALL_DRIVERS=true go test ./...

# Test specific package
go test ./database/postgres/...

# Test with verbose output
go test -v ./...

# Test with race detection
go test -race ./...
```

### With Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.txt -covermode=atomic ./...

# View coverage in terminal
go tool cover -func=coverage.txt

# View coverage in browser
go tool cover -html=coverage.txt
```

### Testing All Drivers

```bash
# Test PostgreSQL + SQLite + libSQL
TEST_ALL_DRIVERS=true go test ./...

# Test only SQLite
go test ./database/sqlite/...
```

### CI Environment

In GitHub Actions, tests run with:

```bash
REQUIRE_TEST_DB=true go test -race -coverprofile=coverage.txt ./...
```

This ensures all tests run and fail if databases are unavailable.

## Writing Tests

### Unit Tests

Unit tests should not require a database:

```go
func TestGeneratePlan_AddTable(t *testing.T) {
    // No database needed - pure logic testing
    oldSchema := &Schema{Tables: []Table{}}
    newSchema := &Schema{Tables: []Table{
        {Name: "users", Columns: []Column{{Name: "id", Type: "integer"}}},
    }}

    diff := DiffSchemas(oldSchema, newSchema)
    plan := GeneratePlan(diff)

    if len(plan.Steps) != 1 {
        t.Errorf("Expected 1 step, got %d", len(plan.Steps))
    }
}
```

### Integration Tests

Integration tests use the `SetupTestDB` helper:

```go
func TestApplyPlan_CreateTable(t *testing.T) {
    // Use helper to set up test database
    tdb := SetupTestDB(t, "postgres")
    defer tdb.Close()
    defer tdb.CleanupTables(t, "users")

    // Test with real database
    plan := &Plan{
        Steps: []Step{
            {
                Type: "create_table",
                SQL:  "CREATE TABLE users (id integer PRIMARY KEY)",
            },
        },
    }

    err := ApplyPlan(tdb.DB, plan)
    if err != nil {
        t.Fatalf("ApplyPlan failed: %v", err)
    }

    // Verify table was created
    var exists bool
    err = tdb.DB.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')").Scan(&exists)
    if err != nil {
        t.Fatalf("Query failed: %v", err)
    }
    if !exists {
        t.Error("Expected users table to exist")
    }
}
```

### Multi-Driver Tests

Test across all supported drivers:

```go
func TestIntrospection(t *testing.T) {
    for _, driverType := range GetAllDrivers() {
        t.Run(driverType, func(t *testing.T) {
            tdb := SetupTestDB(t, driverType)
            defer tdb.Close()

            // Test introspection with this driver
            // ...
        })
    }
}
```

## Test Database Setup

### PostgreSQL

For local PostgreSQL testing, you have several options:

**Option 1: Use CI for PostgreSQL tests**
- Push to GitHub and let CI run PostgreSQL integration tests
- Run SQLite tests locally with `TEST_ALL_DRIVERS=true go test ./...`

**Option 2: Manual PostgreSQL installation**
- Install PostgreSQL locally
- Create databases: `lockplane` (port 5432) and `lockplane_shadow` (port 5433)
- Set environment variables:
  ```bash
  export DATABASE_URL="postgres://user:pass@localhost:5432/lockplane?sslmode=disable"
  export SHADOW_DATABASE_URL="postgres://user:pass@localhost:5433/lockplane_shadow?sslmode=disable"
  ```

**Option 3: Docker**
```bash
docker run -d -p 5432:5432 -e POSTGRES_USER=lockplane -e POSTGRES_PASSWORD=lockplane -e POSTGRES_DB=lockplane postgres:15-alpine
docker run -d -p 5433:5432 -e POSTGRES_USER=lockplane -e POSTGRES_PASSWORD=lockplane -e POSTGRES_DB=lockplane_shadow postgres:15-alpine
```

### SQLite

SQLite tests use in-memory databases (`:memory:`), so no setup required:

```go
tdb := SetupTestDB(t, "sqlite")
// Uses :memory: database automatically
```

### libSQL/Turso

For local testing, libSQL uses in-memory databases.

For testing against real Turso databases:

```bash
# Set up Turso database
turso db create lockplane-test

# Get connection URL
turso db show lockplane-test

# Create auth token
turso db tokens create lockplane-test

# Run tests with real Turso database
export LIBSQL_TEST_URL="libsql://lockplane-test-yourname.turso.io?authToken=..."
export TEST_LIBSQL=true
export TEST_ALL_DRIVERS=true
go test ./...
```

## Continuous Integration

Tests run automatically in GitHub Actions on:
- Every push to `main`
- Every pull request

### CI Test Matrix

The CI runs 4 parallel jobs on every push:

1. **Unit Tests** (~1m14s)
   - Fast tests, no database required
   - Includes format check and `go vet`
   - Must pass for PR merge

2. **PostgreSQL Integration Tests** (~48s)
   - Uses GitHub Actions PostgreSQL service containers
   - Tests main database (port 5432) + shadow database (port 5433)
   - Runs all integration tests with `REQUIRE_TEST_DB=true`
   - Uploads coverage to Codecov with `postgres` flag

3. **SQLite Integration Tests** (~30s)
   - Uses in-memory SQLite (no service needed)
   - Tests SQLite-specific functionality
   - Runs multi-database tests (TestApplyPlan_InvalidSQL, TestApplyPlan_AddColumn)
   - Uploads coverage to Codecov with `sqlite` flag

4. **Lint** (~1m33s)
   - Runs golangci-lint v2
   - Checks code quality and style

### Coverage Tracking

- **Coverage reports**: [![codecov](https://codecov.io/github/zakandrewking/lockplane/graph/badge.svg?token=JP0QINP1G1)](https://codecov.io/github/zakandrewking/lockplane)
- **Separate coverage** for PostgreSQL and SQLite test runs
- **Coverage trends** tracked over time on Codecov
- Coverage reports uploaded automatically from CI

## Troubleshooting

### Tests Skip with "Database not available"

**Problem**: Integration tests skip because PostgreSQL isn't running.

**Solution**:
- Run unit tests only: `go test -short ./...`
- Run SQLite integration tests: `TEST_ALL_DRIVERS=true go test ./...`
- For PostgreSQL tests: Set up PostgreSQL (see "Test Database Setup" section) or rely on CI

### Tests Fail with "connection refused"

**Problem**: Database hasn't finished starting.

**Solution**:
- Run tests without PostgreSQL: `go test -short ./...` or `TEST_ALL_DRIVERS=true go test ./...`
- If using local PostgreSQL, wait for it to be ready:
  ```bash
  until pg_isready -h localhost -p 5432; do
      echo "Waiting for PostgreSQL..."
      sleep 2
  done
  go test ./...
  ```

### Tests Fail with "database lockplane does not exist"

**Problem**: Database hasn't been created.

**Solution**:
- Create the database manually:
  ```bash
  createdb -U lockplane lockplane
  createdb -U lockplane lockplane_shadow
  ```
- Or run tests without PostgreSQL: `go test -short ./...`

### Coverage Report Not Generating

**Problem**: `coverage.txt` not created.

**Solution**:
```bash
# Ensure you're using -coverprofile flag
go test -coverprofile=coverage.txt -covermode=atomic ./...

# Check file was created
ls -la coverage.txt

# Generate HTML report
go tool cover -html=coverage.txt -o coverage.html
```

## Best Practices

### DO

- ✅ Use `SetupTestDB` helper for integration tests
- ✅ Clean up test data with `defer tdb.CleanupTables(t, "table1", "table2")`
- ✅ Use `t.Helper()` in test helper functions
- ✅ Test edge cases and error conditions
- ✅ Use table-driven tests for multiple similar cases
- ✅ Write descriptive test names: `TestApplyPlan_CreateTable_WithForeignKeys`

### DON'T

- ❌ Don't skip tests permanently - fix or remove them
- ❌ Don't use `time.Sleep()` - use proper synchronization
- ❌ Don't ignore errors in tests - check all error returns
- ❌ Don't leave test data in database - always clean up
- ❌ Don't hardcode connection strings - use environment variables
- ❌ Don't commit `coverage.txt` or `coverage.html` - add to `.gitignore`

## Example Test Workflow

1. **Write failing test**
   ```go
   func TestNewFeature(t *testing.T) {
       // Test the feature that doesn't exist yet
       result := NewFeature()
       if result != expected {
           t.Errorf("Expected %v, got %v", expected, result)
       }
   }
   ```

2. **Run test** (should fail)
   ```bash
   go test -v -run TestNewFeature
   ```

3. **Implement feature**
   ```go
   func NewFeature() Result {
       // Implementation
   }
   ```

4. **Run test** (should pass)
   ```bash
   go test -v -run TestNewFeature
   ```

5. **Run full suite**
   ```bash
   go test ./...
   ```

6. **Check coverage**
   ```bash
   go test -cover ./...
   ```

## Additional Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table-Driven Tests in Go](https://go.dev/wiki/TableDrivenTests)
- [Advanced Testing Patterns](https://www.youtube.com/watch?v=8hQG7QlcLBk)
- [Lockplane Test Suite Improvement Plan](devdocs/test-suite-improvement-plan.md)
