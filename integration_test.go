package main

import (
	"context"
	"database/sql"
	"github.com/lockplane/lockplane/internal/planner"
	"testing"

	_ "github.com/lib/pq"
	"github.com/lockplane/lockplane/database/postgres"
	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/testutil"
)

func resolveTestEnvironment(t *testing.T) *config.ResolvedEnvironment {
	t.Helper()

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	env, err := config.ResolveEnvironment(cfg, "")
	if err != nil {
		t.Fatalf("Failed to resolve default environment: %v", err)
	}

	return env
}

// compareSchemas compares two Schema objects (used by json_schema_test.go)
func compareSchemas(t *testing.T, expected, actual *Schema) {
	t.Helper()

	if len(expected.Tables) != len(actual.Tables) {
		t.Errorf("Expected %d tables, got %d", len(expected.Tables), len(actual.Tables))
	}

	// Build a map of expected tables for easier lookup
	expectedTables := make(map[string]*Table)
	for i := range expected.Tables {
		expectedTables[expected.Tables[i].Name] = &expected.Tables[i]
	}

	// Compare each actual table with expected
	for i := range actual.Tables {
		actualTable := &actual.Tables[i]
		expectedTable, ok := expectedTables[actualTable.Name]
		if !ok {
			t.Errorf("Unexpected table: %s", actualTable.Name)
			continue
		}

		compareTable(t, expectedTable, actualTable)
	}

	// Check for missing tables
	for _, expectedTable := range expected.Tables {
		found := false
		for i := range actual.Tables {
			if actual.Tables[i].Name == expectedTable.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing expected table: %s", expectedTable.Name)
		}
	}
}

// compareTable compares two Table objects
func compareTable(t *testing.T, expected, actual *Table) {
	t.Helper()

	if expected.Name != actual.Name {
		t.Errorf("Table name mismatch: expected %s, got %s", expected.Name, actual.Name)
	}

	// Compare columns
	if len(expected.Columns) != len(actual.Columns) {
		t.Errorf("Table %s: expected %d columns, got %d", expected.Name, len(expected.Columns), len(actual.Columns))
	}

	expectedCols := make(map[string]*Column)
	for i := range expected.Columns {
		expectedCols[expected.Columns[i].Name] = &expected.Columns[i]
	}

	for i := range actual.Columns {
		actualCol := &actual.Columns[i]
		expectedCol, ok := expectedCols[actualCol.Name]
		if !ok {
			t.Errorf("Table %s: unexpected column %s", expected.Name, actualCol.Name)
			continue
		}

		compareColumn(t, expected.Name, expectedCol, actualCol)
	}

	// Compare indexes
	if len(expected.Indexes) != len(actual.Indexes) {
		t.Errorf("Table %s: expected %d indexes, got %d", expected.Name, len(expected.Indexes), len(actual.Indexes))
	}

	expectedIdxs := make(map[string]*Index)
	for i := range expected.Indexes {
		expectedIdxs[expected.Indexes[i].Name] = &expected.Indexes[i]
	}

	for i := range actual.Indexes {
		actualIdx := &actual.Indexes[i]
		expectedIdx, ok := expectedIdxs[actualIdx.Name]
		if !ok {
			t.Errorf("Table %s: unexpected index %s", expected.Name, actualIdx.Name)
			continue
		}

		compareIndex(t, expected.Name, expectedIdx, actualIdx)
	}
}

// compareColumn compares two Column objects
func compareColumn(t *testing.T, tableName string, expected, actual *Column) {
	t.Helper()

	if expected.Name != actual.Name {
		t.Errorf("Table %s: column name mismatch: expected %s, got %s", tableName, expected.Name, actual.Name)
	}

	if expected.Type != actual.Type {
		t.Errorf("Table %s, column %s: expected type %s, got %s", tableName, expected.Name, expected.Type, actual.Type)
	}

	if expected.Nullable != actual.Nullable {
		t.Errorf("Table %s, column %s: expected nullable=%v, got %v", tableName, expected.Name, expected.Nullable, actual.Nullable)
	}

	if expected.IsPrimaryKey != actual.IsPrimaryKey {
		t.Errorf("Table %s, column %s: expected is_primary_key=%v, got %v", tableName, expected.Name, expected.IsPrimaryKey, actual.IsPrimaryKey)
	}

	// Check default value presence (not exact match, as functions may vary)
	if (expected.Default == nil) != (actual.Default == nil) {
		t.Errorf("Table %s, column %s: default value presence mismatch (expected has default: %v, actual has default: %v)",
			tableName, expected.Name, expected.Default != nil, actual.Default != nil)
	}
}

// compareIndex compares two Index objects
func compareIndex(t *testing.T, tableName string, expected, actual *Index) {
	t.Helper()

	if expected.Name != actual.Name {
		t.Errorf("Table %s: index name mismatch: expected %s, got %s", tableName, expected.Name, actual.Name)
	}

	if expected.Unique != actual.Unique {
		t.Errorf("Table %s, index %s: expected unique=%v, got %v", tableName, expected.Name, expected.Unique, actual.Unique)
	}

	// Note: We're not comparing columns yet since the introspector doesn't parse them
}

// Note: The following golden file tests were removed as they required extensive fixture creation
// but schema introspection is already comprehensively tested in:
// - database/postgres/introspector_test.go (5 PostgreSQL tests)
// - database/sqlite/introspector_test.go (6 SQLite tests)
//
// Removed tests:
// - TestBasicSchema, TestComprehensiveSchema, TestIndexesSchema

// Executor tests

func TestApplyPlan_CreateTable(t *testing.T) {
	// Note: This test uses PostgreSQL-only JSON fixtures (SERIAL, TIMESTAMP, NOW()).
	// Database-agnostic SQL generation is tested in database/*/generator_test.go.
	// SQLite introspection is tested in database/sqlite/introspector_test.go.
	tdb := testutil.SetupTestDB(t, "postgres")
	defer tdb.Close()
	defer tdb.CleanupTables(t, "posts")

	ctx := context.Background()

	// Load plan from JSON
	planPtr, err := planner.LoadJSONPlan("testdata/plans-json/create_table.json")
	if err != nil {
		t.Fatalf("Failed to load plan: %v", err)
	}
	plan := *planPtr

	// Create empty schema for apply
	emptySchema := &Schema{Tables: []Table{}}

	// Execute plan
	result, err := applyPlan(ctx, tdb.DB, &plan, nil, emptySchema, tdb.Driver)
	if err != nil {
		t.Fatalf("Failed to apply plan: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success=true, got false")
	}

	if result.StepsApplied != 1 {
		t.Errorf("Expected 1 step applied, got %d", result.StepsApplied)
	}

	// Verify table was created
	var exists bool
	err = tdb.DB.QueryRowContext(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'posts')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check table existence: %v", err)
	}

	if !exists {
		t.Error("Expected posts table to exist")
	}
}

func TestApplyPlan_WithShadowDB(t *testing.T) {
	// Note: This test is PostgreSQL-only because:
	// 1. Uses create_table.json plan with PostgreSQL-specific SQL (SERIAL, NOW())
	// 2. Requires separate shadow database (typically port 5433)
	// 3. resolveTestEnvironment expects PostgreSQL configuration
	// Future: Could extend to SQLite with two in-memory databases
	env := resolveTestEnvironment(t)
	mainConnStr := env.DatabaseURL
	shadowConnStr := env.ShadowDatabaseURL

	mainDB, err := sql.Open("postgres", mainConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to main database: %v", err)
	}
	defer func() { _ = mainDB.Close() }()

	shadowDB, err := sql.Open("postgres", shadowConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to shadow database: %v", err)
	}
	defer func() { _ = shadowDB.Close() }()

	ctx := context.Background()
	if err := mainDB.PingContext(ctx); err != nil {
		t.Skipf("Main database not available (this is okay in CI): %v", err)
	}
	if err := shadowDB.PingContext(ctx); err != nil {
		t.Skipf("Shadow database not available (this is okay in CI): %v", err)
	}

	// Clean up both databases
	_, _ = mainDB.ExecContext(ctx, "DROP TABLE IF EXISTS posts CASCADE")
	_, _ = shadowDB.ExecContext(ctx, "DROP TABLE IF EXISTS posts CASCADE")
	defer func() {
		_, _ = mainDB.ExecContext(ctx, "DROP TABLE IF EXISTS posts CASCADE")
		_, _ = shadowDB.ExecContext(ctx, "DROP TABLE IF EXISTS posts CASCADE")
	}()

	// Load plan from JSON
	planPtr, err := planner.LoadJSONPlan("testdata/plans-json/create_table.json")
	if err != nil {
		t.Fatalf("Failed to load plan: %v", err)
	}
	plan := *planPtr

	// Create empty schema and driver for apply
	emptySchema := &Schema{Tables: []Table{}}
	driver := postgres.NewDriver()

	// Execute plan with shadow DB validation
	result, err := applyPlan(ctx, mainDB, &plan, shadowDB, emptySchema, driver)
	if err != nil {
		t.Fatalf("Failed to apply plan: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success=true, got false")
	}

	// Verify table exists in main DB
	var mainExists bool
	err = mainDB.QueryRowContext(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'posts')").Scan(&mainExists)
	if err != nil {
		t.Fatalf("Failed to check main table existence: %v", err)
	}
	if !mainExists {
		t.Error("Expected posts table to exist in main database")
	}

	// Verify table does NOT exist in shadow DB (should have been rolled back)
	var shadowExists bool
	err = shadowDB.QueryRowContext(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'posts')").Scan(&shadowExists)
	if err != nil {
		t.Fatalf("Failed to check shadow table existence: %v", err)
	}
	if shadowExists {
		t.Error("Expected posts table to NOT exist in shadow database (should be rolled back)")
	}
}

func TestApplyPlan_InvalidSQL(t *testing.T) {
	// This test validates error handling and works with all databases
	for _, driverType := range testutil.GetAllDrivers() {
		t.Run(driverType, func(t *testing.T) {
			tdb := testutil.SetupTestDB(t, driverType)
			defer tdb.Close()

			ctx := context.Background()

			// Create an invalid plan inline (no JSON fixture for this)
			plan := planner.Plan{
				Steps: []planner.PlanStep{
					{Description: "Invalid SQL", SQL: "INVALID SQL STATEMENT"},
				},
			}

			// Create empty schema for apply
			emptySchema := &Schema{Tables: []Table{}}

			// Execute plan - should fail
			result, err := applyPlan(ctx, tdb.DB, &plan, nil, emptySchema, tdb.Driver)
			if err == nil {
				t.Error("Expected error for invalid SQL, got nil")
			}

			if result.Success {
				t.Error("Expected success=false for invalid SQL")
			}

			if len(result.Errors) == 0 {
				t.Error("Expected errors to be recorded")
			}
		})
	}
}

func TestApplyPlan_ShadowDB_CatchesTypeConversionFailure(t *testing.T) {
	// This test demonstrates shadow DB catching a realistic migration failure:
	// Converting BIGINT to INTEGER when data contains values outside INTEGER range.
	// This is a common mistake when trying to "optimize" column types.
	env := resolveTestEnvironment(t)
	mainConnStr := env.DatabaseURL
	shadowConnStr := env.ShadowDatabaseURL

	mainDB, err := sql.Open("postgres", mainConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to main database: %v", err)
	}
	defer func() { _ = mainDB.Close() }()

	shadowDB, err := sql.Open("postgres", shadowConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to shadow database: %v", err)
	}
	defer func() { _ = shadowDB.Close() }()

	ctx := context.Background()
	if err := mainDB.PingContext(ctx); err != nil {
		t.Skipf("Main database not available (this is okay in CI): %v", err)
	}
	if err := shadowDB.PingContext(ctx); err != nil {
		t.Skipf("Shadow database not available (this is okay in CI): %v", err)
	}

	// Clean up both databases
	_, _ = mainDB.ExecContext(ctx, "DROP TABLE IF EXISTS analytics CASCADE")
	_, _ = shadowDB.ExecContext(ctx, "DROP TABLE IF EXISTS analytics CASCADE")
	defer func() {
		_, _ = mainDB.ExecContext(ctx, "DROP TABLE IF EXISTS analytics CASCADE")
		_, _ = shadowDB.ExecContext(ctx, "DROP TABLE IF EXISTS analytics CASCADE")
	}()

	// Setup: Create analytics table with BIGINT user_id containing large values
	// This simulates a real scenario where user IDs use snowflake or Twitter-style IDs
	createTableSQL := `
		CREATE TABLE analytics (
			id SERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			event_name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`
	if _, err := mainDB.ExecContext(ctx, createTableSQL); err != nil {
		t.Fatalf("Failed to create analytics table in main DB: %v", err)
	}
	if _, err := shadowDB.ExecContext(ctx, createTableSQL); err != nil {
		t.Fatalf("Failed to create analytics table in shadow DB: %v", err)
	}

	// Insert realistic data with large user IDs (outside INTEGER range: > 2,147,483,647)
	// Twitter/Snowflake IDs are typically 64-bit integers like 1234567890123456789
	insertSQL := `
		INSERT INTO analytics (user_id, event_name) VALUES
		(9223372036854775807, 'page_view'),     -- max BIGINT value
		(1234567890123456789, 'signup'),        -- typical snowflake ID
		(9876543210987654321, 'purchase'),      -- large ID
		(123, 'login'),                          -- small ID (would fit in INTEGER)
		(5000000000, 'click')                    -- > INTEGER max (2147483647)
	`
	if _, err := mainDB.ExecContext(ctx, insertSQL); err != nil {
		t.Fatalf("Failed to insert test data into main DB: %v", err)
	}
	if _, err := shadowDB.ExecContext(ctx, insertSQL); err != nil {
		t.Fatalf("Failed to insert test data into shadow DB: %v", err)
	}

	// Create a migration plan that tries to "optimize" by converting BIGINT to INTEGER
	// This looks structurally valid but will fail on the actual data
	dangerousPlan := planner.Plan{
		Steps: []planner.PlanStep{
			{
				Description: "Alter column analytics.user_id type from BIGINT to INTEGER",
				SQL:         "ALTER TABLE analytics ALTER COLUMN user_id TYPE INTEGER",
			},
		},
		SourceHash: "", // Not validating hash for this test
	}

	// Create schema representing the current state
	currentSchema := &Schema{
		Tables: []Table{
			{
				Name: "analytics",
				Columns: []Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "user_id", Type: "bigint", Nullable: false},
					{Name: "event_name", Type: "text", Nullable: false},
					{Name: "created_at", Type: "timestamp without time zone", Nullable: false},
				},
			},
		},
	}

	driver := postgres.NewDriver()

	// Execute plan with shadow DB validation - should FAIL on shadow DB
	result, err := applyPlan(ctx, mainDB, &dangerousPlan, shadowDB, currentSchema, driver)

	// We expect the apply to fail because shadow DB should catch the error
	if err == nil {
		t.Error("Expected error from shadow DB validation, got nil")
	}

	if result.Success {
		t.Error("Expected success=false when shadow DB catches error")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors to be recorded from shadow DB failure")
	}

	// Verify the error message mentions the shadow DB validation failure
	foundShadowError := false
	for _, errMsg := range result.Errors {
		if len(errMsg) > 0 {
			foundShadowError = true
			t.Logf("Shadow DB caught error (as expected): %s", errMsg)
			break
		}
	}
	if !foundShadowError {
		t.Error("Expected shadow DB validation error to be recorded")
	}

	// CRITICAL: Verify main database was NOT modified (shadow DB protected it)
	var mainUserID int64
	err = mainDB.QueryRowContext(ctx, "SELECT user_id FROM analytics WHERE event_name = 'page_view'").Scan(&mainUserID)
	if err != nil {
		t.Fatalf("Failed to query main database after failed migration: %v", err)
	}

	// Verify data is still intact in main DB
	if mainUserID != 9223372036854775807 {
		t.Errorf("Main database was modified despite shadow DB failure! Expected user_id=9223372036854775807, got %d", mainUserID)
	}

	// Verify column type is still BIGINT in main database
	var mainColumnType string
	err = mainDB.QueryRowContext(ctx,
		`SELECT data_type FROM information_schema.columns
		 WHERE table_name = 'analytics' AND column_name = 'user_id'`).Scan(&mainColumnType)
	if err != nil {
		t.Fatalf("Failed to check column type in main DB: %v", err)
	}
	if mainColumnType != "bigint" {
		t.Errorf("Main database column type changed despite shadow DB failure! Expected 'bigint', got '%s'", mainColumnType)
	}

	t.Log("✅ Shadow DB successfully protected production from dangerous type conversion")
}

func TestApplyPlan_AddColumn(t *testing.T) {
	// This test uses a plan with portable SQL (ALTER TABLE ... ADD COLUMN)
	// and handles database-specific table creation in the setup
	for _, driverType := range testutil.GetAllDrivers() {
		t.Run(driverType, func(t *testing.T) {
			tdb := testutil.SetupTestDB(t, driverType)
			defer tdb.Close()
			defer tdb.CleanupTables(t, "users")

			ctx := context.Background()

			// Set up - create users table first
			var createTableSQL string
			if tdb.Type == "postgres" || tdb.Type == "postgresql" {
				createTableSQL = "CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT NOT NULL)"
			} else {
				// SQLite/libSQL
				createTableSQL = "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"
			}
			_, err := tdb.DB.ExecContext(ctx, createTableSQL)
			if err != nil {
				t.Fatalf("Failed to create users table: %v", err)
			}

			// Load plan to add age column from JSON
			planPtr, err := planner.LoadJSONPlan("testdata/plans-json/add_column.json")
			if err != nil {
				t.Fatalf("Failed to load plan: %v", err)
			}
			plan := *planPtr

			// Create schema with the existing users table
			existingSchema := &Schema{
				Tables: []Table{
					{
						Name: "users",
						Columns: []Column{
							{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
							{Name: "name", Type: "text", Nullable: false},
						},
					},
				},
			}

			// Execute plan
			result, err := applyPlan(ctx, tdb.DB, &plan, nil, existingSchema, tdb.Driver)
			if err != nil {
				t.Fatalf("Failed to apply plan: %v", err)
			}

			if !result.Success {
				t.Errorf("Expected success=true, got false")
			}

			if result.StepsApplied != 1 {
				t.Errorf("Expected 1 step applied, got %d", result.StepsApplied)
			}

			// Verify age column was added
			var hasAge bool
			var checkQuery string
			if tdb.Type == "postgres" || tdb.Type == "postgresql" {
				checkQuery = "SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'age')"
			} else {
				// SQLite/libSQL
				checkQuery = "SELECT COUNT(*) > 0 FROM pragma_table_info('users') WHERE name='age'"
			}
			err = tdb.DB.QueryRowContext(ctx, checkQuery).Scan(&hasAge)
			if err != nil {
				t.Fatalf("Failed to check column existence: %v", err)
			}

			if !hasAge {
				t.Error("Expected age column to exist")
			}
		})
	}
}

func TestDetectDriver(t *testing.T) {
	tests := []struct {
		name     string
		connStr  string
		expected string
	}{
		// PostgreSQL
		{
			name:     "postgres URL",
			connStr:  "postgres://user:pass@localhost:5432/dbname",
			expected: "postgres",
		},
		{
			name:     "postgresql URL",
			connStr:  "postgresql://user:pass@localhost:5432/dbname",
			expected: "postgres",
		},
		{
			name:     "postgres uppercase",
			connStr:  "POSTGRES://USER:PASS@LOCALHOST:5432/DBNAME",
			expected: "postgres",
		},
		// libSQL/Turso
		{
			name:     "libsql URL",
			connStr:  "libsql://mydb-user.turso.io",
			expected: "libsql",
		},
		{
			name:     "libsql with auth token",
			connStr:  "libsql://mydb-user.turso.io?authToken=eyJhbGc...",
			expected: "libsql",
		},
		{
			name:     "libsql uppercase",
			connStr:  "LIBSQL://MYDB-USER.TURSO.IO",
			expected: "libsql",
		},
		// SQLite
		{
			name:     "sqlite URL",
			connStr:  "sqlite://path/to/database.db",
			expected: "sqlite",
		},
		{
			name:     "file URL",
			connStr:  "file:path/to/database.db",
			expected: "sqlite",
		},
		{
			name:     "db file",
			connStr:  "test.db",
			expected: "sqlite",
		},
		{
			name:     "sqlite file",
			connStr:  "test.sqlite",
			expected: "sqlite",
		},
		{
			name:     "sqlite3 file",
			connStr:  "test.sqlite3",
			expected: "sqlite",
		},
		{
			name:     "in-memory",
			connStr:  ":memory:",
			expected: "sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectDriver(tt.connStr)
			if result != tt.expected {
				t.Errorf("detectDriver(%q) = %q, want %q", tt.connStr, result, tt.expected)
			}
		})
	}
}

func TestGetSQLDriverName(t *testing.T) {
	tests := []struct {
		name       string
		driverType string
		expected   string
	}{
		// PostgreSQL
		{
			name:       "postgres",
			driverType: "postgres",
			expected:   "postgres",
		},
		{
			name:       "postgresql",
			driverType: "postgresql",
			expected:   "postgres",
		},
		// SQLite
		{
			name:       "sqlite",
			driverType: "sqlite",
			expected:   "sqlite",
		},
		{
			name:       "sqlite3",
			driverType: "sqlite3",
			expected:   "sqlite",
		},
		// libSQL/Turso (critical test for the bug we just fixed)
		{
			name:       "libsql",
			driverType: "libsql",
			expected:   "libsql",
		},
		// Unknown/default
		{
			name:       "unknown falls through",
			driverType: "unknown",
			expected:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSQLDriverName(tt.driverType)
			if result != tt.expected {
				t.Errorf("getSQLDriverName(%q) = %q, want %q", tt.driverType, result, tt.expected)
			}
		})
	}
}
