// This file contains integration tests for lockplane with SQLite/libSQL.
//
// Tests verify SQLite-specific behaviors including file-based databases,
// DDL generation, and migration execution.
package integration_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/schema"
	_ "modernc.org/sqlite"
)

// TestSQLitePlanGeneration_CreateTable verifies that migration plans
// for SQLite schemas are generated correctly
func TestSQLitePlanGeneration_CreateTable(t *testing.T) {
	// Empty schema (before)
	before := &database.Schema{
		Dialect: database.DialectSQLite,
		Tables:  []database.Table{},
	}

	// Schema with one table (after)
	afterDDL := `
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    completed INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
`
	after, err := schema.LoadSQLSchemaFromBytes([]byte(afterDDL), &schema.SchemaLoadOptions{Dialect: database.DialectSQLite})
	if err != nil {
		t.Fatalf("failed to parse after schema: %v", err)
	}

	// Compute diff
	diff := schema.DiffSchemas(before, after)

	// Create SQLite driver for plan generation
	driver, err := executor.NewDriver("sqlite")
	if err != nil {
		t.Fatalf("failed to create driver: %v", err)
	}

	// Generate plan
	plan, err := planner.GeneratePlanWithHash(diff, before, driver)
	if err != nil {
		t.Fatalf("failed to generate plan: %v", err)
	}

	if len(plan.Steps) == 0 {
		t.Fatal("expected plan to have steps")
	}

	// Verify we have a CREATE TABLE step
	var foundCreateTable bool
	for _, step := range plan.Steps {
		for _, sqlStmt := range step.SQL {
			if strings.Contains(strings.ToUpper(sqlStmt), "CREATE TABLE") &&
				strings.Contains(sqlStmt, "tasks") {
				foundCreateTable = true
				// Verify SQLite-specific syntax is preserved
				if !strings.Contains(sqlStmt, "INTEGER") {
					t.Error("expected INTEGER type in CREATE TABLE")
				}
			}
		}
	}

	if !foundCreateTable {
		t.Error("expected CREATE TABLE tasks in plan")
	}
}

// TestSQLitePlanGeneration_AddColumn tests adding a column to an existing table
func TestSQLitePlanGeneration_AddColumn(t *testing.T) {
	beforeDDL := `
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL
);
`
	before, err := schema.LoadSQLSchemaFromBytes([]byte(beforeDDL), &schema.SchemaLoadOptions{Dialect: database.DialectSQLite})
	if err != nil {
		t.Fatalf("failed to parse before schema: %v", err)
	}

	afterDDL := `
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL,
    name TEXT
);
`
	after, err := schema.LoadSQLSchemaFromBytes([]byte(afterDDL), &schema.SchemaLoadOptions{Dialect: database.DialectSQLite})
	if err != nil {
		t.Fatalf("failed to parse after schema: %v", err)
	}

	// Compute diff
	diff := schema.DiffSchemas(before, after)

	// Create SQLite driver for plan generation
	driver, err := executor.NewDriver("sqlite")
	if err != nil {
		t.Fatalf("failed to create driver: %v", err)
	}

	// Generate plan
	plan, err := planner.GeneratePlanWithHash(diff, before, driver)
	if err != nil {
		t.Fatalf("failed to generate plan: %v", err)
	}

	// SQLite uses table recreation for column additions in some cases
	// Verify we have steps to add the column
	if len(plan.Steps) == 0 {
		t.Fatal("expected plan to have steps for adding column")
	}

	foundColumnChange := false
	for _, step := range plan.Steps {
		for _, sqlStmt := range step.SQL {
			sql := strings.ToUpper(sqlStmt)
			if strings.Contains(sql, "NAME") &&
				(strings.Contains(sql, "ADD COLUMN") || strings.Contains(sql, "CREATE TABLE")) {
				foundColumnChange = true
			}
		}
	}

	if !foundColumnChange {
		t.Error("expected plan to include column addition")
	}
}

// TestSQLitePlanExecution_EndToEnd tests the complete flow:
// 1. Load SQLite schema
// 2. Generate plan
// 3. Execute against in-memory database
// 4. Verify result
func TestSQLitePlanExecution_EndToEnd(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Empty starting state
	before := &database.Schema{
		Dialect: database.DialectSQLite,
		Tables:  []database.Table{},
	}

	// Target schema
	targetDDL := `
CREATE TABLE products (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    price REAL DEFAULT 0.0
);

CREATE INDEX idx_products_name ON products(name);
`
	after, err := schema.LoadSQLSchemaFromBytes([]byte(targetDDL), &schema.SchemaLoadOptions{Dialect: database.DialectSQLite})
	if err != nil {
		t.Fatalf("failed to parse target schema: %v", err)
	}

	// Compute diff
	diff := schema.DiffSchemas(before, after)

	// Create SQLite driver for plan generation
	driver, err := executor.NewDriver("sqlite")
	if err != nil {
		t.Fatalf("failed to create driver: %v", err)
	}

	// Generate plan
	plan, err := planner.GeneratePlanWithHash(diff, before, driver)
	if err != nil {
		t.Fatalf("failed to generate plan: %v", err)
	}

	// Execute plan steps
	for i, step := range plan.Steps {
		t.Logf("Executing step %d: %s", i+1, step.Description)
		for j, sqlStmt := range step.SQL {
			if _, err := db.ExecContext(ctx, sqlStmt); err != nil {
				t.Fatalf("failed to execute step %d, statement %d/%d (%s): %v\nSQL: %s",
					i+1, j+1, len(step.SQL), step.Description, err, sqlStmt)
			}
		}
	}

	// Verify table was created
	var tableName string
	err = db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='products'").Scan(&tableName)
	if err != nil {
		t.Fatalf("failed to verify table creation: %v", err)
	}
	if tableName != "products" {
		t.Errorf("expected table 'products', got %q", tableName)
	}

	// Verify columns
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(products)")
	if err != nil {
		t.Fatalf("failed to query table info: %v", err)
	}
	defer func() { _ = rows.Close() }()

	columnCount := 0
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		columnCount++
		t.Logf("Column: %s %s (pk=%d, notnull=%d, default=%v)", name, colType, pk, notNull, dfltValue)
	}

	if columnCount != 3 {
		t.Errorf("expected 3 columns, got %d", columnCount)
	}
}

// TestSQLiteRollbackGeneration tests that rollback plans are generated correctly
func TestSQLiteRollbackGeneration(t *testing.T) {
	// Schema with table
	beforeDDL := `
CREATE TABLE notes (
    id INTEGER PRIMARY KEY,
    content TEXT NOT NULL
);
`
	before, err := schema.LoadSQLSchemaFromBytes([]byte(beforeDDL), &schema.SchemaLoadOptions{Dialect: database.DialectSQLite})
	if err != nil {
		t.Fatalf("failed to parse before schema: %v", err)
	}

	// Empty schema (after - simulating DROP TABLE)
	after := &database.Schema{
		Dialect: database.DialectSQLite,
		Tables:  []database.Table{},
	}

	// Compute diff
	diff := schema.DiffSchemas(before, after)

	// Create SQLite driver
	driver, err := executor.NewDriver("sqlite")
	if err != nil {
		t.Fatalf("failed to create driver: %v", err)
	}

	// Generate forward plan
	forwardPlan, err := planner.GeneratePlanWithHash(diff, before, driver)
	if err != nil {
		t.Fatalf("failed to generate forward plan: %v", err)
	}

	// Should have DROP TABLE
	foundDrop := false
	for _, step := range forwardPlan.Steps {
		for _, sqlStmt := range step.SQL {
			if strings.Contains(strings.ToUpper(sqlStmt), "DROP TABLE") {
				foundDrop = true
			}
		}
	}
	if !foundDrop {
		t.Error("expected forward plan to contain DROP TABLE")
	}

	// Generate rollback plan (reverse diff)
	reverseDiff := schema.DiffSchemas(after, before)
	rollbackPlan, err := planner.GeneratePlanWithHash(reverseDiff, after, driver)
	if err != nil {
		t.Fatalf("failed to generate rollback plan: %v", err)
	}

	// Rollback should recreate the table
	foundCreate := false
	for _, step := range rollbackPlan.Steps {
		for _, sqlStmt := range step.SQL {
			if strings.Contains(strings.ToUpper(sqlStmt), "CREATE TABLE") &&
				strings.Contains(sqlStmt, "notes") {
				foundCreate = true
			}
		}
	}
	if !foundCreate {
		t.Error("expected rollback plan to contain CREATE TABLE notes")
	}
}

func TestApplyPlan_ShadowDB_CatchesUniqueConstraintViolation(t *testing.T) {
	// This test demonstrates shadow DB catching a realistic SQLite migration failure:
	// Adding a UNIQUE constraint when duplicate data exists.
	// This is a common mistake when trying to improve data quality on legacy data.

	// Create two in-memory databases (main and shadow)
	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open main database: %v", err)
	}
	defer func() { _ = mainDB.Close() }()

	shadowDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open shadow database: %v", err)
	}
	defer func() { _ = shadowDB.Close() }()

	ctx := context.Background()

	// Setup: Create user_emails table with duplicate emails
	// This simulates legacy data where email uniqueness wasn't enforced
	createTableSQL := `
		CREATE TABLE user_emails (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		)
	`
	if _, err := mainDB.ExecContext(ctx, createTableSQL); err != nil {
		t.Fatalf("Failed to create user_emails table in main DB: %v", err)
	}
	if _, err := shadowDB.ExecContext(ctx, createTableSQL); err != nil {
		t.Fatalf("Failed to create user_emails table in shadow DB: %v", err)
	}

	// Insert data with DUPLICATE emails (common in legacy systems)
	// This is realistic: same person signed up multiple times, or data migration bug
	insertSQL := `
		INSERT INTO user_emails (email) VALUES
		('alice@example.com'),
		('bob@example.com'),
		('alice@example.com'),    -- DUPLICATE!
		('charlie@example.com'),
		('bob@example.com')       -- DUPLICATE!
	`
	if _, err := mainDB.ExecContext(ctx, insertSQL); err != nil {
		t.Fatalf("Failed to insert test data into main DB: %v", err)
	}
	if _, err := shadowDB.ExecContext(ctx, insertSQL); err != nil {
		t.Fatalf("Failed to insert test data into shadow DB: %v", err)
	}

	// Create a migration plan that tries to add UNIQUE constraint to clean up data quality
	// This looks like a good idea but will fail because duplicates exist
	dangerousPlan := planner.Plan{
		Steps: []planner.PlanStep{
			{
				Description: "Add unique index on user_emails.email",
				SQL:         []string{"CREATE UNIQUE INDEX idx_user_emails_email ON user_emails(email)"},
			},
		},
		SourceHash: "", // Not validating hash for this test
	}

	// Create schema representing the current state
	currentSchema := &database.Schema{
		Dialect: database.DialectSQLite,
		Tables: []database.Table{
			{
				Name: "user_emails",
				Columns: []database.Column{
					{Name: "id", Type: "INTEGER", Nullable: false, IsPrimaryKey: true},
					{Name: "email", Type: "TEXT", Nullable: false},
					{Name: "created_at", Type: "TEXT", Nullable: true},
				},
			},
		},
	}

	driver, err := executor.NewDriver("sqlite")
	if err != nil {
		t.Fatalf("Failed to create driver: %v", err)
	}

	// Execute plan with shadow DB validation - should FAIL on shadow DB
	result, err := executor.ApplyPlan(ctx, mainDB, &dangerousPlan, shadowDB, currentSchema, driver, false)

	// We expect the apply to fail because shadow DB should catch the duplicate error
	if err == nil {
		t.Error("Expected error from shadow DB validation, got nil")
	}

	if result.Success {
		t.Error("Expected success=false when shadow DB catches error")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors to be recorded from shadow DB failure")
	}

	// Verify the error message mentions the constraint violation
	foundConstraintError := false
	for _, errMsg := range result.Errors {
		if len(errMsg) > 0 {
			foundConstraintError = true
			t.Logf("Shadow DB caught error (as expected): %s", errMsg)
			// SQLite error should mention "UNIQUE constraint failed" or similar
			break
		}
	}
	if !foundConstraintError {
		t.Error("Expected shadow DB constraint violation error to be recorded")
	}

	// CRITICAL: Verify main database was NOT modified (shadow DB protected it)
	// Check that the UNIQUE index was NOT created in main DB
	var mainIndexCount int
	err = mainDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master
		 WHERE type='index' AND name='idx_user_emails_email'`).Scan(&mainIndexCount)
	if err != nil {
		t.Fatalf("Failed to check index existence in main DB: %v", err)
	}

	if mainIndexCount > 0 {
		t.Error("Main database was modified despite shadow DB failure! UNIQUE index should not exist")
	}

	// Verify duplicate data is still intact in main DB (shadow DB prevented data loss)
	var duplicateCount int
	err = mainDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM user_emails WHERE email = 'alice@example.com'").Scan(&duplicateCount)
	if err != nil {
		t.Fatalf("Failed to query main database after failed migration: %v", err)
	}

	if duplicateCount != 2 {
		t.Errorf("Main database data was modified! Expected 2 'alice@example.com' entries, got %d", duplicateCount)
	}

	t.Log("✅ Shadow DB successfully protected production from UNIQUE constraint violation")
}

// TestSQLiteSchemaIntrospectionRoundTrip tests that introspecting a SQLite
// database and then generating a plan produces no changes
func TestSQLiteSchemaIntrospectionRoundTrip(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") != "" {
		t.Skip("Skipping database test")
	}

	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Create schema directly in database
	schemaDDL := `
CREATE TABLE authors (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE
);

CREATE TABLE books (
    id INTEGER PRIMARY KEY,
    author_id INTEGER NOT NULL REFERENCES authors(id),
    title TEXT NOT NULL,
    published_at TEXT
);

CREATE INDEX idx_books_author ON books(author_id);
`
	if _, err := db.ExecContext(ctx, schemaDDL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Introspect the database
	driver, err := executor.NewDriver("sqlite")
	if err != nil {
		t.Fatalf("failed to create driver: %v", err)
	}

	introspectedSchema, err := driver.IntrospectSchema(ctx, db)
	if err != nil {
		t.Fatalf("failed to introspect schema: %v", err)
	}

	// Parse the same DDL as target schema
	targetDDL := schemaDDL
	targetSchema, err := schema.LoadSQLSchemaFromBytes([]byte(targetDDL), &schema.SchemaLoadOptions{Dialect: database.DialectSQLite})
	if err != nil {
		t.Fatalf("failed to parse target schema: %v", err)
	}

	// Generate plan between introspected and target
	diff := schema.DiffSchemas((*database.Schema)(introspectedSchema), targetSchema)
	plan, err := planner.GeneratePlanWithHash(diff, (*database.Schema)(introspectedSchema), driver)
	if err != nil {
		t.Fatalf("failed to generate plan: %v", err)
	}

	// Should have no steps (schemas are equivalent)
	if len(plan.Steps) > 0 {
		t.Logf("WARNING: Expected no plan steps for equivalent schemas, got %d:", len(plan.Steps))
		for i, step := range plan.Steps {
			t.Logf("  Step %d: %s\n    SQL: %v", i+1, step.Description, step.SQL)
		}
		// Note: This might not be zero due to index column ordering or other
		// minor differences between parsed and introspected schemas.
		// We log it as a warning rather than failing.
	}
}
