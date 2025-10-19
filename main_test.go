package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
	"github.com/lockplane/lockplane/database/postgres"
)

// goldenTest runs a test case using fixture files
func goldenTest(t *testing.T, fixtureName string) {
	t.Helper()

	// Connect to test database
	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Database not available (this is okay in CI): %v", err)
	}

	// Read SQL fixture
	fixtureDir := filepath.Join("testdata", "fixtures", fixtureName)
	sqlPath := filepath.Join(fixtureDir, "schema.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatalf("Failed to read SQL fixture: %v", err)
	}

	// Read expected output from JSON file
	expectedPath := filepath.Join(fixtureDir, "expected.json")
	expectedSchema, err := LoadJSONSchema(expectedPath)
	if err != nil {
		t.Fatalf("Failed to load expected JSON schema: %v", err)
	}
	expected := *expectedSchema

	// Clean up tables first (extract table names from expected output)
	for _, table := range expected.Tables {
		_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table.Name+" CASCADE")
	}

	// Execute SQL to create schema
	_, err = db.ExecContext(ctx, string(sqlBytes))
	if err != nil {
		t.Fatalf("Failed to execute SQL fixture: %v", err)
	}

	// Clean up after test
	defer func() {
		for _, table := range expected.Tables {
			_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table.Name+" CASCADE")
		}
	}()

	// Run introspection using postgres driver
	driver := postgres.NewDriver()
	actual, err := driver.IntrospectSchema(ctx, db)
	if err != nil {
		t.Fatalf("Failed to introspect schema: %v", err)
	}

	// Compare results
	compareSchemas(t, &expected, actual)
}

// compareSchemas compares two Schema objects
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

// Test cases using golden files
func TestBasicSchema(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	goldenTest(t, "basic")
}

func TestComprehensiveSchema(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	goldenTest(t, "comprehensive")
}

func TestIndexesSchema(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	goldenTest(t, "indexes")
}

// Executor tests

func TestApplyPlan_CreateTable(t *testing.T) {
	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Database not available (this is okay in CI): %v", err)
	}

	// Clean up
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS posts CASCADE")
	defer func() {
		_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS posts CASCADE")
	}()

	// Load plan from JSON
	planPtr, err := LoadJSONPlan("testdata/plans-json/create_table.json")
	if err != nil {
		t.Fatalf("Failed to load plan: %v", err)
	}
	plan := *planPtr

	// Execute plan
	result, err := applyPlan(ctx, db, &plan, nil)
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
	err = db.QueryRowContext(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'posts')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check table existence: %v", err)
	}

	if !exists {
		t.Error("Expected posts table to exist")
	}
}

func TestApplyPlan_WithShadowDB(t *testing.T) {
	mainConnStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")
	shadowConnStr := getEnv("SHADOW_DATABASE_URL", "postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable")

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
	planPtr, err := LoadJSONPlan("testdata/plans-json/create_table.json")
	if err != nil {
		t.Fatalf("Failed to load plan: %v", err)
	}
	plan := *planPtr

	// Execute plan with shadow DB validation
	result, err := applyPlan(ctx, mainDB, &plan, shadowDB)
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
	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Database not available (this is okay in CI): %v", err)
	}

	// Create an invalid plan inline (no JSON fixture for this)
	plan := Plan{
		Steps: []PlanStep{
			{Description: "Invalid SQL", SQL: "INVALID SQL STATEMENT"},
		},
	}

	// Execute plan - should fail
	result, err := applyPlan(ctx, db, &plan, nil)
	if err == nil {
		t.Error("Expected error for invalid SQL, got nil")
	}

	if result.Success {
		t.Error("Expected success=false for invalid SQL")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors to be recorded")
	}
}

func TestApplyPlan_AddColumn(t *testing.T) {
	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Database not available (this is okay in CI): %v", err)
	}

	// Set up - create users table first
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS users CASCADE")
	_, err = db.ExecContext(ctx, "CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT NOT NULL)")
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS users CASCADE")
	}()

	// Load plan to add age column from JSON
	planPtr, err := LoadJSONPlan("testdata/plans-json/add_column.json")
	if err != nil {
		t.Fatalf("Failed to load plan: %v", err)
	}
	plan := *planPtr

	// Execute plan
	result, err := applyPlan(ctx, db, &plan, nil)
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
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'age'
		)
	`).Scan(&hasAge)
	if err != nil {
		t.Fatalf("Failed to check column existence: %v", err)
	}

	if !hasAge {
		t.Error("Expected age column to exist")
	}
}
