package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// Version information (set by goreleaser during build)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type Schema struct {
	Tables []Table `json:"tables"`
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	Indexes []Index  `json:"indexes"`
}

type Column struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default,omitempty"`
	IsPrimaryKey bool    `json:"is_primary_key"`
}

type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

func main() {
	if len(os.Args) < 2 {
		// Default to introspect command for backward compatibility
		runIntrospect(os.Args[1:])
		return
	}

	command := os.Args[1]

	switch command {
	case "version", "-v", "--version":
		fmt.Printf("lockplane %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		return
	case "help", "-h", "--help":
		printHelp()
		return
	case "introspect":
		runIntrospect(os.Args[2:])
	case "diff":
		runDiff(os.Args[2:])
	case "plan":
		runPlan(os.Args[2:])
	case "rollback":
		runRollback(os.Args[2:])
	default:
		// If not a recognized command, assume it's a flag for introspect
		runIntrospect(os.Args[1:])
	}
}

func runIntrospect(args []string) {
	fs := flag.NewFlagSet("introspect", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	schema, err := introspectSchema(ctx, db)
	if err != nil {
		log.Fatalf("Failed to introspect schema: %v", err)
	}

	jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal schema to JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}

func runDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if fs.NArg() != 2 {
		log.Fatalf("Usage: lockplane diff <before.json> <after.json>")
	}

	beforePath := fs.Arg(0)
	afterPath := fs.Arg(1)

	// Load schemas
	before, err := LoadJSONSchema(beforePath)
	if err != nil {
		log.Fatalf("Failed to load before schema: %v", err)
	}

	after, err := LoadJSONSchema(afterPath)
	if err != nil {
		log.Fatalf("Failed to load after schema: %v", err)
	}

	// Generate diff
	diff := DiffSchemas(before, after)

	// Output diff as JSON
	jsonBytes, err := json.MarshalIndent(diff, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal diff to JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}

func runPlan(args []string) {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	fromSchema := fs.String("from", "", "Source schema file (before)")
	toSchema := fs.String("to", "", "Target schema file (after)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	// Generate diff first
	var diff *SchemaDiff

	if *fromSchema != "" && *toSchema != "" {
		// Generate diff from two schemas
		before, err := LoadJSONSchema(*fromSchema)
		if err != nil {
			log.Fatalf("Failed to load from schema: %v", err)
		}

		after, err := LoadJSONSchema(*toSchema)
		if err != nil {
			log.Fatalf("Failed to load to schema: %v", err)
		}

		diff = DiffSchemas(before, after)
	} else {
		log.Fatalf("Usage: lockplane plan --from <before.json> --to <after.json>")
	}

	// Generate plan
	plan, err := GeneratePlan(diff)
	if err != nil {
		log.Fatalf("Failed to generate plan: %v", err)
	}

	// Output plan as JSON
	jsonBytes, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal plan to JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}

func runRollback(args []string) {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	planPath := fs.String("plan", "", "Forward migration plan file")
	fromSchema := fs.String("from", "", "Source schema file (before state)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *planPath == "" || *fromSchema == "" {
		log.Fatalf("Usage: lockplane rollback --plan <forward.json> --from <before.json>")
	}

	// Load the forward plan
	forwardPlan, err := LoadJSONPlan(*planPath)
	if err != nil {
		log.Fatalf("Failed to load forward plan: %v", err)
	}

	// Load the before schema
	beforeSchema, err := LoadJSONSchema(*fromSchema)
	if err != nil {
		log.Fatalf("Failed to load before schema: %v", err)
	}

	// Generate rollback plan
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema)
	if err != nil {
		log.Fatalf("Failed to generate rollback: %v", err)
	}

	// Output rollback plan as JSON
	jsonBytes, err := json.MarshalIndent(rollbackPlan, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal rollback to JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}

func introspectSchema(ctx context.Context, db *sql.DB) (*Schema, error) {
	schema := &Schema{}

	// Get all tables in public schema
	rows, err := db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	// For each table, get columns and indexes
	for _, tableName := range tableNames {
		table := Table{Name: tableName}

		// Get columns
		columns, err := getColumns(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}
		table.Columns = columns

		// Get indexes
		indexes, err := getIndexes(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get indexes for table %s: %w", tableName, err)
		}
		table.Indexes = indexes

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func getColumns(ctx context.Context, db *sql.DB, tableName string) ([]Column, error) {
	query := `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			COALESCE(
				(SELECT true
				 FROM information_schema.table_constraints tc
				 JOIN information_schema.key_column_usage kcu
				   ON tc.constraint_name = kcu.constraint_name
				 WHERE tc.table_name = c.table_name
				   AND tc.constraint_type = 'PRIMARY KEY'
				   AND kcu.column_name = c.column_name),
				false
			) as is_primary_key
		FROM information_schema.columns c
		WHERE c.table_schema = 'public'
		  AND c.table_name = $1
		ORDER BY c.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var nullable string
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &col.IsPrimaryKey); err != nil {
			return nil, err
		}

		col.Nullable = nullable == "YES"
		if defaultVal.Valid {
			col.Default = &defaultVal.String
		}

		columns = append(columns, col)
	}

	return columns, nil
}

func getIndexes(ctx context.Context, db *sql.DB, tableName string) ([]Index, error) {
	query := `
		SELECT
			i.indexname,
			i.indexdef,
			ix.indisunique
		FROM pg_indexes i
		JOIN pg_class c ON c.relname = i.tablename
		JOIN pg_index ix ON ix.indexrelid = (
			SELECT oid FROM pg_class WHERE relname = i.indexname
		)
		WHERE i.schemaname = 'public'
		  AND i.tablename = $1
		ORDER BY i.indexname
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var idx Index
		var indexDef string

		if err := rows.Scan(&idx.Name, &indexDef, &idx.Unique); err != nil {
			return nil, err
		}

		// TODO: Parse indexDef to extract column names properly
		// For now, just storing the index name and unique flag
		idx.Columns = []string{} // Simplified for MVP

		indexes = append(indexes, idx)
	}

	return indexes, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func printHelp() {
	fmt.Print(`Lockplane - Postgres-first control plane for safe schema management

USAGE:
  lockplane <command> [options]

COMMANDS:
  introspect       Introspect database and output current schema as JSON
  diff             Compare two schemas and show differences
  plan             Generate migration plan from schema diff
  rollback         Generate rollback plan from forward migration
  version          Show version information
  help             Show this help message

EXAMPLES:
  # Introspect current database
  lockplane introspect > current.json

  # Compare schemas
  lockplane diff before.json after.json

  # Generate migration plan
  lockplane plan --from current.json --to desired.json > migration.json

  # Generate rollback plan
  lockplane rollback --plan migration.json --from current.json > rollback.json

ENVIRONMENT:
  DATABASE_URL            Postgres connection string
                          (default: postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable)

For more information: https://github.com/lockplane/lockplane
`)
}

// Plan represents a migration plan with a series of steps
type Plan struct {
	Steps []PlanStep `json:"steps"`
}

// PlanStep represents a single migration operation
type PlanStep struct {
	Description string `json:"description"`
	SQL         string `json:"sql"`
}

// ExecutionResult tracks the outcome of executing a plan
type ExecutionResult struct {
	Success      bool     `json:"success"`
	StepsApplied int      `json:"steps_applied"`
	Errors       []string `json:"errors,omitempty"`
}

// applyPlan executes a migration plan with optional shadow DB validation
func applyPlan(ctx context.Context, db *sql.DB, plan *Plan, shadowDB *sql.DB) (*ExecutionResult, error) {
	result := &ExecutionResult{
		Success: false,
		Errors:  []string{},
	}

	// If shadow DB provided, run dry-run first
	if shadowDB != nil {
		if err := dryRunPlan(ctx, shadowDB, plan); err != nil {
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
		_, err := tx.ExecContext(ctx, step.SQL)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("step %d (%s) failed: %v", i, step.Description, err))
			return result, fmt.Errorf("step %d failed: %w", i, err)
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

// dryRunPlan validates a plan by executing it on shadow DB and rolling back
func dryRunPlan(ctx context.Context, shadowDB *sql.DB, plan *Plan) error {
	tx, err := shadowDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin shadow transaction: %w", err)
	}
	defer tx.Rollback() // Always rollback shadow DB changes

	// Execute each step
	for i, step := range plan.Steps {
		_, err := tx.ExecContext(ctx, step.SQL)
		if err != nil {
			return fmt.Errorf("shadow DB step %d (%s) failed: %w", i, step.Description, err)
		}
	}

	return nil
}
