# Test Suite Improvement Plan

**Status**: Proposed
**Created**: 2025-11-05
**Goal**: Improve test coverage, simplify test organization, and ensure reliable CI testing

---

## Executive Summary

This document outlines a comprehensive plan to improve Lockplane's test suite by:
- Adding SQLite/libSQL integration tests (currently missing)
- Making integration tests database-agnostic
- Ensuring tests run reliably in CI instead of silently skipping
- Improving test organization and maintainability
- Adding test coverage metrics and reporting

---

## Current State Analysis

### Test Organization

Tests are currently split into two categories:

#### Unit Tests (no database required)
- ✅ `database/interface_test.go` - Data structure marshaling
- ✅ `database/postgres/driver_test.go` - PostgreSQL driver interface
- ✅ `database/sqlite/driver_test.go` - SQLite driver interface
- ✅ `database/postgres/generator_test.go` - PostgreSQL SQL generation
- ✅ `database/sqlite/generator_test.go` - SQLite SQL generation
- ✅ `planner_test.go`, `rollback_test.go`, `diff_test.go` - Business logic

#### Integration Tests (require live database)
- ⚠️ `main_test.go` - End-to-end workflows (**PostgreSQL only**)
- ⚠️ `database/postgres/introspector_test.go` - Schema introspection (**PostgreSQL only**)
- ❌ **No SQLite/libSQL integration tests exist**

### Key Issues

1. **PostgreSQL-Only Integration Tests**
   - `main_test.go` hardcodes PostgreSQL connections
   - Uses only `postgres.NewDriver()`
   - SQLite/libSQL drivers untested in integration scenarios

2. **Silent Test Skipping in CI**
   - Tests use `t.Skipf("Database not available (this is okay in CI)")`
   - No visibility when tests are skipped
   - Can hide real issues

3. **Duplicate Test Setup Code**
   - Database connection logic repeated across files
   - No shared test utilities
   - Maintenance burden

4. **Permanently Skipped Tests**
   - 6 tests in `diff_test.go` skipped: "JSON test fixtures not yet created"
   - 3 tests in `main_test.go` skipped: "JSON test fixtures not yet created"
   - Tech debt accumulating

5. **No Coverage Metrics**
   - Can't track test quality over time
   - No coverage requirements in CI
   - Unknown gaps in test coverage

---

## Improvement Plan

### Priority 1: Database-Agnostic Integration Tests

**Goal**: Make integration tests work with PostgreSQL, SQLite, and libSQL.

#### Implementation

Create `testing_utils.go`:

```go
// testing_utils.go
// +build !test

package main

import (
    "context"
    "database/sql"
    "os"
    "testing"

    _ "github.com/lib/pq"
    _ "modernc.org/sqlite"
    _ "github.com/tursodatabase/libsql-client-go/libsql"

    "github.com/lockplane/lockplane/database"
    "github.com/lockplane/lockplane/database/postgres"
    "github.com/lockplane/lockplane/database/sqlite"
)

// TestDB encapsulates a test database connection and driver
type TestDB struct {
    DB     *sql.DB
    Driver database.Driver
    Type   string
    ctx    context.Context
}

// Close closes the database connection
func (tdb *TestDB) Close() {
    if tdb.DB != nil {
        _ = tdb.DB.Close()
    }
}

// SetupTestDB creates a test database connection for the specified driver type
// Skips the test if the database is unavailable (unless REQUIRE_TEST_DB=true)
func SetupTestDB(t *testing.T, driverType string) *TestDB {
    t.Helper()

    requireDB := os.Getenv("REQUIRE_TEST_DB") == "true"

    var db *sql.DB
    var driver database.Driver
    var err error

    switch driverType {
    case "postgres", "postgresql":
        connStr := os.Getenv("POSTGRES_TEST_URL")
        if connStr == "" {
            connStr = "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
        }

        db, err = sql.Open("postgres", connStr)
        if err != nil {
            if requireDB {
                t.Fatalf("PostgreSQL required but unavailable: %v", err)
            }
            t.Skipf("PostgreSQL not available: %v", err)
        }

        if err := db.Ping(); err != nil {
            _ = db.Close()
            if requireDB {
                t.Fatalf("PostgreSQL required but unreachable: %v", err)
            }
            t.Skipf("PostgreSQL not reachable: %v", err)
        }

        driver = postgres.NewDriver()

    case "sqlite", "sqlite3":
        // Use in-memory database for fast tests
        db, err = sql.Open("sqlite", ":memory:")
        if err != nil {
            if requireDB {
                t.Fatalf("SQLite required but unavailable: %v", err)
            }
            t.Skipf("SQLite not available: %v", err)
        }

        // Enable foreign keys for SQLite
        _, err = db.Exec("PRAGMA foreign_keys = ON")
        if err != nil {
            _ = db.Close()
            t.Fatalf("Failed to enable foreign keys: %v", err)
        }

        driver = sqlite.NewDriver()

    case "libsql":
        // Use in-memory database for libSQL tests
        connStr := os.Getenv("LIBSQL_TEST_URL")
        if connStr == "" {
            connStr = "file::memory:?cache=shared"
        }

        db, err = sql.Open("libsql", connStr)
        if err != nil {
            if requireDB {
                t.Fatalf("libSQL required but unavailable: %v", err)
            }
            t.Skipf("libSQL not available: %v", err)
        }

        driver = sqlite.NewDriver() // libSQL uses SQLite driver

    default:
        t.Fatalf("Unknown database type: %s", driverType)
    }

    return &TestDB{
        DB:     db,
        Driver: driver,
        Type:   driverType,
        ctx:    context.Background(),
    }
}

// CleanupTables drops the specified tables (safe cleanup for tests)
func (tdb *TestDB) CleanupTables(t *testing.T, tables ...string) {
    t.Helper()

    for _, table := range tables {
        var sql string
        if tdb.Type == "postgres" || tdb.Type == "postgresql" {
            sql = "DROP TABLE IF EXISTS " + table + " CASCADE"
        } else {
            sql = "DROP TABLE IF EXISTS " + table
        }
        _, _ = tdb.DB.ExecContext(tdb.ctx, sql)
    }
}

// GetAllDrivers returns list of all supported drivers for parameterized tests
func GetAllDrivers() []string {
    drivers := []string{"postgres"}

    // Only test SQLite/libSQL if explicitly enabled
    // (to keep default test runs fast)
    if os.Getenv("TEST_ALL_DRIVERS") == "true" {
        drivers = append(drivers, "sqlite")
        if os.Getenv("TEST_LIBSQL") == "true" {
            drivers = append(drivers, "libsql")
        }
    }

    return drivers
}
```

#### Update `main_test.go`

Refactor integration tests to use new helpers:

```go
// Before: PostgreSQL-only
func TestApplyPlan_CreateTable(t *testing.T) {
    env := resolveTestEnvironment(t)
    connStr := env.DatabaseURL
    db, err := sql.Open("postgres", connStr)
    // ...
}

// After: Multi-database
func TestApplyPlan_CreateTable(t *testing.T) {
    for _, driverType := range GetAllDrivers() {
        t.Run(driverType, func(t *testing.T) {
            tdb := SetupTestDB(t, driverType)
            defer tdb.Close()
            defer tdb.CleanupTables(t, "posts")

            // Test logic here...
            // Use tdb.DB and tdb.Driver instead of hardcoded values
        })
    }
}
```

### Priority 2: Add SQLite Integration Tests

**Goal**: Create comprehensive integration tests for SQLite/libSQL.

Create `database/sqlite/introspector_test.go`:

```go
package sqlite

import (
    "context"
    "database/sql"
    "testing"

    _ "modernc.org/sqlite"
    "github.com/lockplane/lockplane/database"
)

func getTestDB(t *testing.T) *sql.DB {
    t.Helper()

    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("Failed to open SQLite: %v", err)
    }

    // Enable foreign keys
    _, err = db.Exec("PRAGMA foreign_keys = ON")
    if err != nil {
        _ = db.Close()
        t.Fatalf("Failed to enable foreign keys: %v", err)
    }

    return db
}

func TestIntrospector_GetTables(t *testing.T) {
    db := getTestDB(t)
    defer db.Close()

    ctx := context.Background()
    introspector := NewIntrospector()

    // Create test tables
    _, err := db.ExecContext(ctx, `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            email TEXT NOT NULL
        )
    `)
    if err != nil {
        t.Fatalf("Failed to create table: %v", err)
    }

    // Test introspection
    tables, err := introspector.GetTables(ctx, db)
    if err != nil {
        t.Fatalf("GetTables failed: %v", err)
    }

    // Verify
    found := false
    for _, table := range tables {
        if table == "users" {
            found = true
            break
        }
    }

    if !found {
        t.Errorf("Expected to find 'users' table, got: %v", tables)
    }
}

func TestIntrospector_GetColumns(t *testing.T) {
    db := getTestDB(t)
    defer db.Close()

    ctx := context.Background()
    introspector := NewIntrospector()

    // Create test table with various column types
    _, err := db.ExecContext(ctx, `
        CREATE TABLE test_columns (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            age INTEGER,
            score REAL DEFAULT 0.0
        )
    `)
    if err != nil {
        t.Fatalf("Failed to create table: %v", err)
    }

    // Get columns
    columns, err := introspector.GetColumns(ctx, db, "test_columns")
    if err != nil {
        t.Fatalf("GetColumns failed: %v", err)
    }

    if len(columns) != 4 {
        t.Errorf("Expected 4 columns, got %d", len(columns))
    }

    // Verify column details
    idCol := findColumn(columns, "id")
    if idCol == nil {
        t.Fatal("Expected to find 'id' column")
    }
    if !idCol.IsPrimaryKey {
        t.Error("Expected id to be primary key")
    }
    if idCol.Nullable {
        t.Error("Expected id to be NOT NULL")
    }
}

func TestIntrospector_GetIndexes(t *testing.T) {
    db := getTestDB(t)
    defer db.Close()

    ctx := context.Background()
    introspector := NewIntrospector()

    // Create table with index
    _, err := db.ExecContext(ctx, `
        CREATE TABLE test_indexes (
            id INTEGER PRIMARY KEY,
            email TEXT
        )
    `)
    if err != nil {
        t.Fatalf("Failed to create table: %v", err)
    }

    _, err = db.ExecContext(ctx, "CREATE UNIQUE INDEX idx_email ON test_indexes (email)")
    if err != nil {
        t.Fatalf("Failed to create index: %v", err)
    }

    // Get indexes
    indexes, err := introspector.GetIndexes(ctx, db, "test_indexes")
    if err != nil {
        t.Fatalf("GetIndexes failed: %v", err)
    }

    // Verify
    found := false
    for _, idx := range indexes {
        if idx.Name == "idx_email" {
            found = true
            if !idx.Unique {
                t.Error("Expected idx_email to be unique")
            }
        }
    }

    if !found {
        t.Error("Expected to find idx_email index")
    }
}

func TestIntrospector_IntrospectSchema(t *testing.T) {
    db := getTestDB(t)
    defer db.Close()

    ctx := context.Background()
    introspector := NewIntrospector()

    // Create comprehensive test schema
    _, err := db.ExecContext(ctx, `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            email TEXT NOT NULL UNIQUE,
            created_at TEXT DEFAULT CURRENT_TIMESTAMP
        );

        CREATE TABLE posts (
            id INTEGER PRIMARY KEY,
            user_id INTEGER NOT NULL,
            title TEXT NOT NULL,
            FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
        );

        CREATE INDEX idx_posts_user_id ON posts (user_id);
    `)
    if err != nil {
        t.Fatalf("Failed to create schema: %v", err)
    }

    // Introspect full schema
    schema, err := introspector.IntrospectSchema(ctx, db)
    if err != nil {
        t.Fatalf("IntrospectSchema failed: %v", err)
    }

    if schema == nil {
        t.Fatal("Expected non-nil schema")
    }

    if len(schema.Tables) != 2 {
        t.Errorf("Expected 2 tables, got %d", len(schema.Tables))
    }

    // Verify users table
    usersTable := findTable(schema.Tables, "users")
    if usersTable == nil {
        t.Fatal("Expected to find users table")
    }
    if len(usersTable.Columns) != 3 {
        t.Errorf("Expected 3 columns in users, got %d", len(usersTable.Columns))
    }

    // Verify posts table and foreign key
    postsTable := findTable(schema.Tables, "posts")
    if postsTable == nil {
        t.Fatal("Expected to find posts table")
    }
    if len(postsTable.ForeignKeys) != 1 {
        t.Errorf("Expected 1 foreign key in posts, got %d", len(postsTable.ForeignKeys))
    }
}

// Helper functions
func findColumn(columns []database.Column, name string) *database.Column {
    for i := range columns {
        if columns[i].Name == name {
            return &columns[i]
        }
    }
    return nil
}

func findTable(tables []database.Table, name string) *database.Table {
    for i := range tables {
        if tables[i].Name == name {
            return &tables[i]
        }
    }
    return nil
}
```

### Priority 3: GitHub Actions Integration

**Goal**: Run all tests including integration tests in CI reliably.

Create `.github/workflows/test.yml`:

```yaml
name: Test Suite

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  unit-tests:
    name: Unit Tests
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.21'
        cache: true

    - name: Install dependencies
      run: go mod download

    - name: Run unit tests
      run: |
        # Run only unit tests (fast, no database needed)
        go test -v -race -short ./...

    - name: Format check
      run: |
        if [ "$(gofmt -s -l . | wc -l)" -gt 0 ]; then
          echo "Code is not formatted:"
          gofmt -s -d .
          exit 1
        fi

    - name: Vet
      run: go vet ./...

    - name: Run errcheck
      run: |
        go install github.com/kisielk/errcheck@latest
        errcheck ./...

    - name: Run staticcheck
      run: |
        go install honnef.co/go/tools/cmd/staticcheck@latest
        staticcheck ./...

  integration-tests-postgres:
    name: Integration Tests (PostgreSQL)
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15-alpine
        env:
          POSTGRES_USER: lockplane
          POSTGRES_PASSWORD: lockplane
          POSTGRES_DB: lockplane
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

      postgres-shadow:
        image: postgres:15-alpine
        env:
          POSTGRES_USER: lockplane
          POSTGRES_PASSWORD: lockplane
          POSTGRES_DB: lockplane_shadow
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5433:5432

    steps:
    - uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.21'
        cache: true

    - name: Install dependencies
      run: go mod download

    - name: Wait for PostgreSQL
      run: |
        until pg_isready -h localhost -p 5432 -U lockplane; do
          echo "Waiting for PostgreSQL..."
          sleep 2
        done

    - name: Run PostgreSQL integration tests
      env:
        REQUIRE_TEST_DB: "true"
        POSTGRES_TEST_URL: "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
        DATABASE_URL: "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
        SHADOW_DATABASE_URL: "postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable"
      run: |
        go test -v -race -coverprofile=coverage-postgres.txt -covermode=atomic ./...

    - name: Upload PostgreSQL coverage
      uses: codecov/codecov-action@v4
      with:
        files: ./coverage-postgres.txt
        flags: postgres
        name: postgres-coverage

  integration-tests-sqlite:
    name: Integration Tests (SQLite)
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.21'
        cache: true

    - name: Install dependencies
      run: go mod download

    - name: Run SQLite integration tests
      env:
        REQUIRE_TEST_DB: "true"
        TEST_ALL_DRIVERS: "true"
      run: |
        go test -v -race -coverprofile=coverage-sqlite.txt -covermode=atomic ./database/sqlite/...

    - name: Upload SQLite coverage
      uses: codecov/codecov-action@v4
      with:
        files: ./coverage-sqlite.txt
        flags: sqlite
        name: sqlite-coverage

  test-summary:
    name: Test Summary
    runs-on: ubuntu-latest
    needs: [unit-tests, integration-tests-postgres, integration-tests-sqlite]

    steps:
    - name: Summary
      run: |
        echo "✅ All tests passed!"
        echo ""
        echo "Test coverage:"
        echo "- Unit tests: Complete"
        echo "- PostgreSQL integration: Complete"
        echo "- SQLite integration: Complete"
```

Update existing `.github/workflows/` or create if needed.

### Priority 4: Test Coverage Reporting

**Goal**: Track and enforce test coverage standards.

#### Local Development

Add to `Makefile` or create `scripts/test.sh`:

```bash
#!/bin/bash
set -e

echo "Running test suite..."
echo ""

# Unit tests (fast)
echo "📦 Running unit tests..."
go test -short -v ./...
echo ""

# Integration tests (requires DB)
if [ "$SKIP_INTEGRATION" != "true" ]; then
    echo "🗄️  Running integration tests..."

    # Check if PostgreSQL is available
    if pg_isready -h localhost -p 5432 >/dev/null 2>&1; then
        echo "  ✓ PostgreSQL available"
        export DATABASE_URL="postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
        export REQUIRE_TEST_DB="true"
    else
        echo "  ⚠️  PostgreSQL not available (skipping DB tests)"
        echo "     Run 'docker compose up -d' to enable integration tests"
    fi

    # Run all tests with coverage
    go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

    # Generate coverage report
    go tool cover -func=coverage.txt | tail -1

    # Generate HTML coverage report
    go tool cover -html=coverage.txt -o coverage.html
    echo "  📊 Coverage report: coverage.html"
else
    echo "⏭️  Skipping integration tests (SKIP_INTEGRATION=true)"
fi

echo ""
echo "✅ Test suite complete!"
```

#### Coverage Badge

Add to `README.md`:

```markdown
[![Test Coverage](https://codecov.io/gh/yourusername/lockplane/branch/main/graph/badge.svg)](https://codecov.io/gh/yourusername/lockplane)
```

### Priority 5: Fix or Remove Skipped Tests

**Goal**: Eliminate permanently skipped tests.

#### Option A: Create Missing Fixtures

For tests like:
```go
func TestBasicSchema(t *testing.T) {
    t.Skip("JSON test fixtures not yet created")
    goldenTest(t, "basic")
}
```

Create the fixtures in `testdata/fixtures/basic/`:
- `schema.sql` - SQL to create the schema
- `expected.json` - Expected introspection output

#### Option B: Remove Tests

If fixtures aren't needed, remove the tests:

```bash
# Remove skipped tests from diff_test.go and main_test.go
# Document decision in commit message
```

### Priority 6: Test Organization Cleanup

**Goal**: Make test suite more maintainable and discoverable.

#### Rename and Reorganize Files

```
Before:
- main_test.go (mixed unit + integration tests)

After:
- integration_test.go (database-dependent integration tests)
- unit_test.go (pure unit tests, no database)
- testing_utils.go (shared test helpers)
```

#### Add Test Documentation

Create `TESTING.md`:

```markdown
# Testing Guide

## Running Tests

### Quick Test (unit tests only)
```bash
go test -short ./...
```

### Full Test Suite (requires PostgreSQL)
```bash
docker compose up -d
go test ./...
```

### With Coverage
```bash
go test -coverprofile=coverage.txt ./...
go tool cover -html=coverage.txt
```

## Test Organization

- **Unit tests**: Fast, no database required
  - `*_test.go` files with `t.Skip()` guards
  - Driver interface tests
  - SQL generation tests

- **Integration tests**: Require live database
  - `integration_test.go`
  - `database/*/introspector_test.go`

## Environment Variables

- `REQUIRE_TEST_DB=true` - Fail if database unavailable (CI)
- `TEST_ALL_DRIVERS=true` - Test PostgreSQL + SQLite
- `SKIP_INTEGRATION=true` - Skip integration tests
- `DATABASE_URL` - PostgreSQL connection string
- `POSTGRES_TEST_URL` - Override default test database

## CI Testing

All tests run automatically in GitHub Actions:
- Unit tests on every push
- PostgreSQL integration tests with real database
- SQLite integration tests with in-memory database
```

---

## Action Plan

### Week 1: Foundation (8-12 hours) ✅ COMPLETE

- [x] Document current state (this document)
- [x] Create `testing_utils.go` with shared helpers
- [x] Add `REQUIRE_TEST_DB` environment variable support
- [x] Set up local coverage reporting (documented in TESTING.md)
- [x] Create `TESTING.md` documentation
- [x] All CI checks passing (build, lint, tests)

**Deliverable**: ✅ Shared test infrastructure in place (commits: 9a54ecb, ca33863) - CI passing

### Week 2: SQLite Support (12-16 hours) ✅ COMPLETE

- [x] Create `database/sqlite/introspector_test.go` (6 tests, all passing)
- [x] Refactor `main_test.go` to use `SetupTestDB()`
- [x] Made 2 integration tests multi-database: TestApplyPlan_InvalidSQL, TestApplyPlan_AddColumn
- [x] Tests pass in CI with PostgreSQL
- [x] Tested locally with SQLite (TEST_ALL_DRIVERS=true) - 2 tests pass ✅
- [x] Documented PostgreSQL-only tests (plan fixtures are database-specific)
- [x] Documented goldenTest function status (unused, future work)

**Deliverable**: ✅ SQLite integration tests working (commits: 5105845, ac8246c) - CI passing

**Key findings**:
- Migration plan JSON fixtures are database-specific (PostgreSQL SQL like SERIAL, NOW())
- 2 tests now work with both PostgreSQL and SQLite
- SQLite introspection fully tested (6 integration tests)
- Test infrastructure supports multiple databases seamlessly

### Week 3: CI Integration (8-12 hours)

- [ ] Create `.github/workflows/test.yml`
- [ ] Set up PostgreSQL service in GitHub Actions
- [ ] Add SQLite test job
- [ ] Configure `REQUIRE_TEST_DB=true` in CI
- [ ] Add codecov integration

**Deliverable**: All tests running in CI

### Week 4: Cleanup (6-8 hours)

- [ ] Fix or remove 9 skipped tests
- [ ] Rename `main_test.go` → `integration_test.go`
- [ ] Add test coverage badge to README
- [ ] Set minimum coverage threshold (70%)
- [ ] Document testing strategy

**Deliverable**: Clean, maintainable test suite

### Total Estimated Effort
**34-48 hours** over 4 weeks (1-2 hours per day)

---

## Success Metrics

After implementation, we should achieve:

- ✅ **>70% test coverage** across all packages
- ✅ **Zero silently skipped tests in CI**
- ✅ **SQLite + PostgreSQL integration tests** both passing
- ✅ **<5 minute CI test runs** for fast feedback
- ✅ **Zero permanently skipped tests**
- ✅ **Coverage trending visible** in README

---

## Risks and Mitigations

### Risk: SQLite behavior differences
**Mitigation**: Mark SQLite-specific limitations in tests, document known differences

### Risk: Slower CI with more tests
**Mitigation**: Run unit tests in parallel, use matrix for database types

### Risk: Flaky database tests
**Mitigation**: Use proper cleanup, transactions, and health checks

### Risk: Developers skip tests locally
**Mitigation**: Make fast unit tests default, integration tests optional

---

## Future Enhancements

After completing this plan:

1. **libSQL/Turso Integration Tests**
   - Add remote libSQL testing
   - Test with real Turso database

2. **Mutation Testing**
   - Use `go-mutesting` to find weak tests
   - Improve test assertions

3. **Benchmark Tests**
   - Add performance benchmarks
   - Track performance over time

4. **Property-Based Testing**
   - Use `gopter` for fuzz testing
   - Generate random schemas

5. **Contract Tests**
   - Ensure drivers maintain contracts
   - Test dialect compatibility

---

## References

- Current test files: `main_test.go`, `database/*/driver_test.go`
- CI workflow: `.github/workflows/` (to be created)
- Test fixtures: `testdata/`
- Docker setup: `docker-compose.yml`

---

## Questions/Discussion

- Should we test against multiple PostgreSQL versions (13, 14, 15)?
- Do we want nightly builds that test against real Turso databases?
- Should we set up test database caching for faster CI?
- What's the minimum acceptable code coverage percentage?

---

**Next Steps**: Review this plan, prioritize items, and begin Week 1 implementation.
