// Package executor provides shared execution utilities for lockplane commands.
//
// This includes database driver detection, schema loading, plan execution,
// and shadow database validation.
package executor

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/database/postgres"
	"github.com/lockplane/lockplane/database/sqlite"
	"github.com/lockplane/lockplane/internal/introspect"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/schema"
	"github.com/lockplane/lockplane/internal/sqliteutil"
)

// DetectDriver detects the database driver type from a connection string.
func DetectDriver(connString string) string {
	connString = strings.ToLower(connString)

	if strings.HasPrefix(connString, "postgres://") || strings.HasPrefix(connString, "postgresql://") {
		return "postgres"
	}

	if strings.HasPrefix(connString, "libsql://") {
		return "libsql"
	}

	if strings.HasPrefix(connString, "sqlite://") ||
		strings.HasPrefix(connString, "file:") ||
		strings.HasSuffix(connString, ".db") ||
		strings.HasSuffix(connString, ".sqlite") ||
		strings.HasSuffix(connString, ".sqlite3") ||
		connString == ":memory:" {
		return "sqlite"
	}

	// Default to postgres for backward compatibility
	return "postgres"
}

// NewDriver creates a new database driver based on the driver name.
func NewDriver(driverName string) (database.Driver, error) {
	switch driverName {
	case "postgres", "postgresql":
		return postgres.NewDriver(), nil
	case "sqlite", "sqlite3":
		return sqlite.NewDriver(), nil
	case "libsql":
		// Turso/libSQL is SQLite-compatible, use SQLite driver for introspection and SQL generation
		return sqlite.NewDriver(), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driverName)
	}
}

// GetSQLDriverName returns the sql.Open driver name for a given database type.
func GetSQLDriverName(driverType string) string {
	switch driverType {
	case "postgres", "postgresql":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite"
	case "libsql":
		return "libsql"
	default:
		return driverType
	}
}

// BuildSchemaLoadOptions creates schema load options based on input and dialect.
func BuildSchemaLoadOptions(input string, dialect database.Dialect) *schema.SchemaLoadOptions {
	if introspect.IsConnectionString(input) || dialect == database.DialectUnknown {
		return nil
	}
	return &schema.SchemaLoadOptions{Dialect: dialect}
}

// LoadSchemaFromConnectionString introspects a database and returns its schema.
func LoadSchemaFromConnectionString(connStr string) (*database.Schema, error) {
	return LoadSchemaFromConnectionStringWithSchemas(connStr, nil)
}

// LoadSchemaFromConnectionStringWithSchemas introspects a database and returns its schema.
// If schemas is provided and non-empty, introspects those specific schemas (PostgreSQL only).
// If schemas is nil or empty, uses default behavior (current_schema() for PostgreSQL).
func LoadSchemaFromConnectionStringWithSchemas(connStr string, schemas []string) (*database.Schema, error) {
	// Detect database driver from connection string
	driverType := DetectDriver(connStr)

	// For SQLite, check if the database file exists and create it if needed
	// Also offer to create shadow database
	if driverType == "sqlite" || driverType == "sqlite3" {
		if err := sqliteutil.EnsureSQLiteDatabaseWithShadow(connStr, "target", false, true); err != nil {
			return nil, err
		}
	}

	driver, err := NewDriver(driverType)
	if err != nil {
		return nil, fmt.Errorf("failed to create database driver: %w", err)
	}

	// Get the SQL driver name (use detected type, not driver.Name())
	sqlDriverName := GetSQLDriverName(driverType)

	db, err := sql.Open(sqlDriverName, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Use multi-schema introspection if schemas are specified
	dbSchema, err := driver.IntrospectSchemas(ctx, db, schemas)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect schema: %w", err)
	}

	dbSchema.Dialect = schema.DriverNameToDialect(driverType)
	return dbSchema, nil
}

// LoadSchemaOrIntrospect loads a schema from a file/directory or introspects from a database connection string.
func LoadSchemaOrIntrospect(pathOrConnStr string) (*database.Schema, error) {
	return LoadSchemaOrIntrospectWithOptions(pathOrConnStr, nil)
}

// LoadSchemaOrIntrospectWithOptions loads a schema with optional parsing options.
func LoadSchemaOrIntrospectWithOptions(pathOrConnStr string, opts *schema.SchemaLoadOptions) (*database.Schema, error) {
	// Check if it's a connection string
	if introspect.IsConnectionString(pathOrConnStr) {
		return LoadSchemaFromConnectionString(pathOrConnStr)
	}

	// Otherwise treat it as a file path
	return schema.LoadSchemaWithOptions(pathOrConnStr, opts)
}

// ApplyPlan executes a migration plan on the target database, with optional shadow DB validation.
func ApplyPlan(ctx context.Context, db *sql.DB, plan *planner.Plan, shadowDB *sql.DB, currentSchema *database.Schema, driver database.Driver, verbose bool) (*planner.ExecutionResult, error) {
	result := &planner.ExecutionResult{
		Success: false,
		Errors:  []string{},
	}

	// Validate source hash if present in plan
	if plan.SourceHash != "" {
		currentHash, err := schema.ComputeSchemaHash(currentSchema)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to compute current schema hash: %v", err))
			return result, fmt.Errorf("failed to compute current schema hash: %w", err)
		}

		if currentHash != plan.SourceHash {
			errMsg := fmt.Sprintf("source schema hash mismatch: expected %s, got %s", plan.SourceHash, currentHash)
			result.Errors = append(result.Errors, errMsg)
			return result, fmt.Errorf("source schema hash mismatch: plan was generated for a different database state")
		}
	}

	// If shadow DB provided, run dry-run first
	if shadowDB != nil {
		if err := DryRunPlan(ctx, shadowDB, plan, currentSchema, driver, verbose); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("dry-run failed: %v", err))
			return result, fmt.Errorf("dry-run validation failed: %w", err)
		}
	}

	// Execute plan in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to begin transaction: %v", err))
		return result, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if !result.Success {
			_ = tx.Rollback()
		}
	}()

	// Execute each step
	for i, step := range plan.Steps {
		if verbose {
			_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "  [Step %d/%d] %s\n", i+1, len(plan.Steps), step.Description)
		}
		// Execute all SQL statements in this step
		for j, sqlStmt := range step.SQL {
			trimmedSQL := strings.TrimSpace(sqlStmt)
			if trimmedSQL == "" || strings.HasPrefix(trimmedSQL, "--") {
				continue // Skip empty or comment-only statements
			}

			if verbose {
				// Show SQL being executed
				sqlPreview := sqlStmt
				if len(sqlPreview) > 200 {
					sqlPreview = sqlPreview[:200] + "..."
				}
				_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "    SQL: %s\n", sqlPreview)
			}

			_, err := tx.ExecContext(ctx, sqlStmt)
			if err != nil {
				errMsg := fmt.Sprintf("step %d, statement %d/%d (%s) failed: %v",
					i+1, j+1, len(step.SQL), step.Description, err)
				result.Errors = append(result.Errors, errMsg)
				return result, fmt.Errorf("step %d failed: %w", i+1, err)
			}

			if verbose {
				_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "    ✓ Executed successfully\n")
			}
		}
		result.StepsApplied++
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to commit: %v", err))
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}

	result.Success = true
	return result, nil
}

// DryRunPlan validates a plan by executing it on shadow DB and rolling back.
func DryRunPlan(ctx context.Context, shadowDB *sql.DB, plan *planner.Plan, currentSchema *database.Schema, driver database.Driver, verbose bool) error {
	// First, clean up any existing tables in the shadow DB
	if err := CleanupShadowDB(ctx, shadowDB, driver, verbose); err != nil {
		return fmt.Errorf("failed to clean shadow DB: %w", err)
	}

	// Apply the current schema to the shadow DB so it matches the target DB state
	// This is necessary because migration plans assume the DB is already in the "before" state
	if verbose {
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "  [Shadow DB] Preparing database to match current state...\n")
	}
	if err := ApplySchemaToDB(ctx, shadowDB, currentSchema, driver, verbose); err != nil {
		return fmt.Errorf("failed to prepare shadow DB: %w", err)
	}

	tx, err := shadowDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin shadow transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Always rollback shadow DB changes
	}()

	if verbose {
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "  [Shadow DB] Testing migration plan...\n")
	}

	// Execute each step
	for i, step := range plan.Steps {
		if verbose {
			_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "    [Step %d/%d] %s\n", i+1, len(plan.Steps), step.Description)
		}
		for j, sqlStmt := range step.SQL {
			trimmedSQL := strings.TrimSpace(sqlStmt)
			if trimmedSQL == "" || strings.HasPrefix(trimmedSQL, "--") {
				continue
			}

			if verbose {
				// Show SQL being executed
				sqlPreview := sqlStmt
				if len(sqlPreview) > 200 {
					sqlPreview = sqlPreview[:200] + "..."
				}
				_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "      SQL: %s\n", sqlPreview)
			}

			_, err := tx.ExecContext(ctx, sqlStmt)
			if err != nil {
				return fmt.Errorf("shadow DB step %d, statement %d/%d (%s) failed: %w",
					i+1, j+1, len(step.SQL), step.Description, err)
			}

			if verbose {
				_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "      ✓ Executed successfully\n")
			}
		}
	}

	if verbose {
		_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "  [Shadow DB] ✓ Migration test successful\n")
	}

	return nil
}

// CleanupShadowDB drops all existing tables from the shadow database.
func CleanupShadowDB(ctx context.Context, db *sql.DB, driver database.Driver, verbose bool) error {
	if verbose {
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "  [Shadow DB] Cleaning up existing tables...\n")
	}

	// Get list of existing tables
	tables, err := driver.GetTables(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get tables: %w", err)
	}

	if len(tables) == 0 {
		if verbose {
			_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "    ✓ Shadow database is clean (no tables)\n")
		}
		return nil
	}

	// Drop all tables in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// For each table, drop it
	for _, tableName := range tables {
		table := database.Table{Name: tableName}
		dropSQL, _ := driver.DropTable(table)

		if verbose {
			_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "    Dropping table %s\n", tableName)
		}

		if _, err := tx.ExecContext(ctx, dropSQL); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", tableName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	if verbose {
		_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "    ✓ Cleaned up %d table(s)\n", len(tables))
	}

	return nil
}

// ApplySchemaToDB applies a complete schema to a database (creates tables, indexes, foreign keys).
func ApplySchemaToDB(ctx context.Context, db *sql.DB, schema *database.Schema, driver database.Driver, verbose bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Create all tables
	for _, table := range schema.Tables {
		sql, _ := driver.CreateTable(table)
		if verbose {
			_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "    Creating table %s\n", table.Name)
		}
		if _, err := tx.ExecContext(ctx, sql); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table.Name, err)
		}

		// Create indexes for this table
		for _, idx := range table.Indexes {
			sql, _ := driver.AddIndex(table.Name, idx)
			if verbose {
				_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "    Creating index %s\n", idx.Name)
			}
			if _, err := tx.ExecContext(ctx, sql); err != nil {
				return fmt.Errorf("failed to create index %s: %w", idx.Name, err)
			}
		}
	}

	// Foreign keys must be added after all tables exist
	for _, table := range schema.Tables {
		for _, fk := range table.ForeignKeys {
			sql, _ := driver.AddForeignKey(table.Name, fk)
			// Skip comment-only SQL (e.g., SQLite foreign key limitations)
			trimmedSQL := strings.TrimSpace(sql)
			if trimmedSQL == "" || strings.HasPrefix(trimmedSQL, "--") {
				continue
			}
			if verbose {
				_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "    Creating foreign key %s\n", fk.Name)
			}
			if _, err := tx.ExecContext(ctx, sql); err != nil {
				return fmt.Errorf("failed to create foreign key %s: %w", fk.Name, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}
