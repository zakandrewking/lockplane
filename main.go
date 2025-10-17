package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

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
	outputFormat := fs.String("format", "cue", "Output format: cue or json")
	fs.Parse(args)

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

	var output string
	switch *outputFormat {
	case "json":
		jsonBytes, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal schema to JSON: %v", err)
		}
		output = string(jsonBytes)
	case "cue":
		output = schemaToCUE(schema)
	default:
		log.Fatalf("Invalid output format: %s (use 'cue' or 'json')", *outputFormat)
	}

	fmt.Println(output)
}

func runDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	outputFormat := fs.String("format", "cue", "Output format: cue or json")
	fs.Parse(args)

	if fs.NArg() != 2 {
		log.Fatalf("Usage: lockplane diff <before.cue> <after.cue>")
	}

	beforePath := fs.Arg(0)
	afterPath := fs.Arg(1)

	// Load schemas
	before, err := LoadCUESchema(beforePath)
	if err != nil {
		log.Fatalf("Failed to load before schema: %v", err)
	}

	after, err := LoadCUESchema(afterPath)
	if err != nil {
		log.Fatalf("Failed to load after schema: %v", err)
	}

	// Generate diff
	diff := DiffSchemas(before, after)

	// Output diff
	var output string
	switch *outputFormat {
	case "json":
		jsonBytes, err := json.MarshalIndent(diff, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal diff to JSON: %v", err)
		}
		output = string(jsonBytes)
	case "cue":
		output = diffToCUE(diff)
	default:
		log.Fatalf("Invalid output format: %s (use 'cue' or 'json')", *outputFormat)
	}

	fmt.Println(output)
}

func runPlan(args []string) {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	outputFormat := fs.String("format", "cue", "Output format: cue or json")
	fromSchema := fs.String("from", "", "Source schema file (before)")
	toSchema := fs.String("to", "", "Target schema file (after)")
	fs.Parse(args)

	// Generate diff first
	var diff *SchemaDiff

	if *fromSchema != "" && *toSchema != "" {
		// Generate diff from two schemas
		before, err := LoadCUESchema(*fromSchema)
		if err != nil {
			log.Fatalf("Failed to load from schema: %v", err)
		}

		after, err := LoadCUESchema(*toSchema)
		if err != nil {
			log.Fatalf("Failed to load to schema: %v", err)
		}

		diff = DiffSchemas(before, after)
	} else {
		log.Fatalf("Usage: lockplane plan --from <before.cue> --to <after.cue>")
	}

	// Generate plan
	plan, err := GeneratePlan(diff)
	if err != nil {
		log.Fatalf("Failed to generate plan: %v", err)
	}

	// Output plan
	var output string
	switch *outputFormat {
	case "json":
		jsonBytes, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal plan to JSON: %v", err)
		}
		output = string(jsonBytes)
	case "cue":
		output = planToCUE(plan)
	default:
		log.Fatalf("Invalid output format: %s (use 'cue' or 'json')", *outputFormat)
	}

	fmt.Println(output)
}

func runRollback(args []string) {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	outputFormat := fs.String("format", "cue", "Output format: cue or json")
	planPath := fs.String("plan", "", "Forward migration plan file")
	fromSchema := fs.String("from", "", "Source schema file (before state)")
	fs.Parse(args)

	if *planPath == "" || *fromSchema == "" {
		log.Fatalf("Usage: lockplane rollback --plan <forward.cue> --from <before.cue>")
	}

	// Load the forward plan
	forwardPlan, err := LoadCUEPlan(*planPath)
	if err != nil {
		log.Fatalf("Failed to load forward plan: %v", err)
	}

	// Load the before schema
	beforeSchema, err := LoadCUESchema(*fromSchema)
	if err != nil {
		log.Fatalf("Failed to load before schema: %v", err)
	}

	// Generate rollback plan
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema)
	if err != nil {
		log.Fatalf("Failed to generate rollback: %v", err)
	}

	// Output rollback plan
	var output string
	switch *outputFormat {
	case "json":
		jsonBytes, err := json.MarshalIndent(rollbackPlan, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal rollback to JSON: %v", err)
		}
		output = string(jsonBytes)
	case "cue":
		output = planToCUE(rollbackPlan)
	default:
		log.Fatalf("Invalid output format: %s (use 'cue' or 'json')", *outputFormat)
	}

	fmt.Println(output)
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

// schemaToCUE converts a Schema to CUE format
func schemaToCUE(schema *Schema) string {
	var sb strings.Builder

	sb.WriteString("package schema\n\n")
	sb.WriteString("import \"github.com/lockplane/lockplane/schema\"\n\n")
	sb.WriteString("schema.#Schema & {\n")
	sb.WriteString("\ttables: [\n")

	for i, table := range schema.Tables {
		sb.WriteString("\t\t{\n")
		sb.WriteString(fmt.Sprintf("\t\t\tname: %q\n", table.Name))
		sb.WriteString("\t\t\tcolumns: [\n")

		for _, col := range table.Columns {
			sb.WriteString("\t\t\t\t{\n")
			sb.WriteString(fmt.Sprintf("\t\t\t\t\tname:           %q\n", col.Name))
			sb.WriteString(fmt.Sprintf("\t\t\t\t\ttype:           %q\n", col.Type))
			sb.WriteString(fmt.Sprintf("\t\t\t\t\tnullable:       %t\n", col.Nullable))
			if col.Default != nil {
				sb.WriteString(fmt.Sprintf("\t\t\t\t\tdefault:        %q\n", *col.Default))
			}
			sb.WriteString(fmt.Sprintf("\t\t\t\t\tis_primary_key: %t\n", col.IsPrimaryKey))
			sb.WriteString("\t\t\t\t},\n")
		}

		sb.WriteString("\t\t\t]\n")

		if len(table.Indexes) > 0 {
			sb.WriteString("\t\t\tindexes: [\n")
			for _, idx := range table.Indexes {
				sb.WriteString("\t\t\t\t{\n")
				sb.WriteString(fmt.Sprintf("\t\t\t\t\tname:    %q\n", idx.Name))

				// Format columns array
				if len(idx.Columns) == 0 {
					sb.WriteString("\t\t\t\t\tcolumns: []\n")
				} else {
					sb.WriteString("\t\t\t\t\tcolumns: [")
					for j, colName := range idx.Columns {
						if j > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString(fmt.Sprintf("%q", colName))
					}
					sb.WriteString("]\n")
				}

				sb.WriteString(fmt.Sprintf("\t\t\t\t\tunique:  %t\n", idx.Unique))
				sb.WriteString("\t\t\t\t},\n")
			}
			sb.WriteString("\t\t\t]\n")
		}

		sb.WriteString("\t\t}")
		if i < len(schema.Tables)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\t]\n")
	sb.WriteString("}\n")

	return sb.String()
}

func printHelp() {
	fmt.Print(`Lockplane - Postgres-first control plane for safe schema management

USAGE:
  lockplane <command> [options]

COMMANDS:
  introspect       Introspect database and output current schema
  diff             Compare two schemas and show differences
  plan             Generate migration plan from schema diff
  rollback         Generate rollback plan from forward migration
  version          Show version information
  help             Show this help message

EXAMPLES:
  # Introspect current database
  lockplane introspect > current.cue

  # Compare schemas
  lockplane diff before.cue after.cue

  # Generate migration plan
  lockplane plan --from current.cue --to desired.cue > migration.cue

  # Generate rollback plan
  lockplane rollback --plan migration.cue --from current.cue > rollback.cue

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

// diffToCUE converts a SchemaDiff to CUE format (simplified human-readable output)
func diffToCUE(diff *SchemaDiff) string {
	var sb strings.Builder

	sb.WriteString("// Schema Diff\n\n")

	if len(diff.AddedTables) > 0 {
		sb.WriteString("added_tables: [\n")
		for _, table := range diff.AddedTables {
			sb.WriteString(fmt.Sprintf("\t%q,\n", table.Name))
		}
		sb.WriteString("]\n\n")
	}

	if len(diff.RemovedTables) > 0 {
		sb.WriteString("removed_tables: [\n")
		for _, table := range diff.RemovedTables {
			sb.WriteString(fmt.Sprintf("\t%q,\n", table.Name))
		}
		sb.WriteString("]\n\n")
	}

	if len(diff.ModifiedTables) > 0 {
		sb.WriteString("modified_tables: [\n")
		for _, tableDiff := range diff.ModifiedTables {
			sb.WriteString(fmt.Sprintf("\t{\n"))
			sb.WriteString(fmt.Sprintf("\t\tname: %q\n", tableDiff.TableName))

			if len(tableDiff.AddedColumns) > 0 {
				sb.WriteString("\t\tadded_columns: [")
				for i, col := range tableDiff.AddedColumns {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%q", col.Name))
				}
				sb.WriteString("]\n")
			}

			if len(tableDiff.RemovedColumns) > 0 {
				sb.WriteString("\t\tremoved_columns: [")
				for i, col := range tableDiff.RemovedColumns {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%q", col.Name))
				}
				sb.WriteString("]\n")
			}

			if len(tableDiff.ModifiedColumns) > 0 {
				sb.WriteString("\t\tmodified_columns: [")
				for i, col := range tableDiff.ModifiedColumns {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%q", col.ColumnName))
				}
				sb.WriteString("]\n")
			}

			sb.WriteString("\t},\n")
		}
		sb.WriteString("]\n")
	}

	if diff.IsEmpty() {
		sb.WriteString("// No changes\n")
	}

	return sb.String()
}

// planToCUE converts a Plan to CUE format
func planToCUE(plan *Plan) string {
	var sb strings.Builder

	sb.WriteString("package plan\n\n")
	sb.WriteString("import \"github.com/lockplane/lockplane/schema\"\n\n")
	sb.WriteString("schema.#Plan & {\n")
	sb.WriteString("\tsteps: [\n")

	for _, step := range plan.Steps {
		sb.WriteString("\t\t{\n")
		sb.WriteString(fmt.Sprintf("\t\t\tdescription: %q\n", step.Description))
		sb.WriteString(fmt.Sprintf("\t\t\tsql:         %q\n", step.SQL))
		sb.WriteString("\t\t},\n")
	}

	sb.WriteString("\t]\n")
	sb.WriteString("}\n")

	return sb.String()
}
