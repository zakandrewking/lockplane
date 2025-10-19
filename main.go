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
	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/database/postgres"
	"github.com/lockplane/lockplane/database/sqlite"
	_ "modernc.org/sqlite"
)

// Version information (set by goreleaser during build)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Type aliases for backward compatibility
type Schema = database.Schema
type Table = database.Table
type Column = database.Column
type Index = database.Index
type ForeignKey = database.ForeignKey

// detectDriver detects the database driver type from a connection string
func detectDriver(connString string) string {
	connString = strings.ToLower(connString)

	if strings.HasPrefix(connString, "postgres://") || strings.HasPrefix(connString, "postgresql://") {
		return "postgres"
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

// newDriver creates a new database driver based on the driver name
func newDriver(driverName string) (database.Driver, error) {
	switch driverName {
	case "postgres", "postgresql":
		return postgres.NewDriver(), nil
	case "sqlite", "sqlite3":
		return sqlite.NewDriver(), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driverName)
	}
}

// newDriverFromConnString creates a new database driver by detecting the type from connection string
func newDriverFromConnString(connString string) (database.Driver, error) {
	driverType := detectDriver(connString)
	return newDriver(driverType)
}

// getSQLDriverName returns the sql.Open driver name for a given database type
func getSQLDriverName(driverType string) string {
	switch driverType {
	case "postgres", "postgresql":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return driverType
	}
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
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
	case "apply":
		runApply(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "validate":
		runValidate(os.Args[2:])
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

	// Detect database driver from connection string
	driver, err := newDriverFromConnString(connStr)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	// Get the SQL driver name
	sqlDriverName := getSQLDriverName(driver.Name())

	db, err := sql.Open(sqlDriverName, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	schema, err := driver.IntrospectSchema(ctx, db)
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
	validate := fs.Bool("validate", false, "Validate migration safety and reversibility")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	// Generate diff first
	var diff *SchemaDiff
	var after *Schema

	if *fromSchema != "" && *toSchema != "" {
		// Generate diff from two schemas
		before, err := LoadJSONSchema(*fromSchema)
		if err != nil {
			log.Fatalf("Failed to load from schema: %v", err)
		}

		after, err = LoadJSONSchema(*toSchema)
		if err != nil {
			log.Fatalf("Failed to load to schema: %v", err)
		}

		diff = DiffSchemas(before, after)
	} else {
		log.Fatalf("Usage: lockplane plan --from <before.json> --to <after.json> [--validate]")
	}

	// Validate the diff if requested
	if *validate {
		validationResults := ValidateSchemaDiffWithSchema(diff, after)

		if len(validationResults) > 0 {
			fmt.Fprintf(os.Stderr, "\n=== Validation Results ===\n\n")

			for i, result := range validationResults {
				if result.Valid {
					fmt.Fprintf(os.Stderr, "✓ Validation %d: PASS\n", i+1)
				} else {
					fmt.Fprintf(os.Stderr, "✗ Validation %d: FAIL\n", i+1)
				}

				if !result.Reversible {
					fmt.Fprintf(os.Stderr, "  ⚠ NOT REVERSIBLE\n")
				}

				for _, err := range result.Errors {
					fmt.Fprintf(os.Stderr, "  Error: %s\n", err)
				}

				for _, warning := range result.Warnings {
					fmt.Fprintf(os.Stderr, "  Warning: %s\n", warning)
				}

				for _, reason := range result.Reasons {
					fmt.Fprintf(os.Stderr, "  - %s\n", reason)
				}

				fmt.Fprintf(os.Stderr, "\n")
			}

			if !AllValid(validationResults) {
				fmt.Fprintf(os.Stderr, "❌ Validation FAILED: Some operations are not safe\n\n")
				os.Exit(1)
			}

			if AllReversible(validationResults) {
				fmt.Fprintf(os.Stderr, "✓ All operations are reversible\n")
			} else {
				fmt.Fprintf(os.Stderr, "⚠ Warning: Some operations are not reversible\n")
			}

			fmt.Fprintf(os.Stderr, "✓ All validations passed\n\n")
		}
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

func runApply(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	planPath := fs.String("plan", "", "Migration plan file to apply")
	skipShadow := fs.Bool("skip-shadow", false, "Skip shadow DB validation (not recommended)")
	autoApprove := fs.Bool("auto-approve", false, "Automatically generate and apply plan from --from and --to schemas")
	fromSchema := fs.String("from", "", "Source schema file (before) - used with --auto-approve")
	toSchema := fs.String("to", "", "Target schema file (after) - used with --auto-approve")
	validate := fs.Bool("validate", false, "Validate migration safety and reversibility - used with --auto-approve")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	var plan *Plan

	// Auto-approve mode: generate plan in-memory from schemas
	if *autoApprove {
		if *fromSchema == "" || *toSchema == "" {
			log.Fatalf("Usage: lockplane apply --auto-approve --from <before.json> --to <after.json> [--validate] [--skip-shadow]")
		}

		// Load schemas
		before, err := LoadJSONSchema(*fromSchema)
		if err != nil {
			log.Fatalf("Failed to load from schema: %v", err)
		}

		after, err := LoadJSONSchema(*toSchema)
		if err != nil {
			log.Fatalf("Failed to load to schema: %v", err)
		}

		// Generate diff
		diff := DiffSchemas(before, after)

		// Validate the diff if requested
		if *validate {
			validationResults := ValidateSchemaDiffWithSchema(diff, after)

			if len(validationResults) > 0 {
				fmt.Fprintf(os.Stderr, "\n=== Validation Results ===\n\n")

				for i, result := range validationResults {
					if result.Valid {
						fmt.Fprintf(os.Stderr, "✓ Validation %d: PASS\n", i+1)
					} else {
						fmt.Fprintf(os.Stderr, "✗ Validation %d: FAIL\n", i+1)
					}

					if !result.Reversible {
						fmt.Fprintf(os.Stderr, "  ⚠ NOT REVERSIBLE\n")
					}

					for _, err := range result.Errors {
						fmt.Fprintf(os.Stderr, "  Error: %s\n", err)
					}

					for _, warning := range result.Warnings {
						fmt.Fprintf(os.Stderr, "  Warning: %s\n", warning)
					}

					for _, reason := range result.Reasons {
						fmt.Fprintf(os.Stderr, "  - %s\n", reason)
					}

					fmt.Fprintf(os.Stderr, "\n")
				}

				if !AllValid(validationResults) {
					fmt.Fprintf(os.Stderr, "❌ Validation FAILED: Some operations are not safe\n\n")
					os.Exit(1)
				}

				if AllReversible(validationResults) {
					fmt.Fprintf(os.Stderr, "✓ All operations are reversible\n")
				} else {
					fmt.Fprintf(os.Stderr, "⚠ Warning: Some operations are not reversible\n")
				}

				fmt.Fprintf(os.Stderr, "✓ All validations passed\n\n")
			}
		}

		// Generate plan
		generatedPlan, err := GeneratePlan(diff)
		if err != nil {
			log.Fatalf("Failed to generate plan: %v", err)
		}
		plan = generatedPlan

		fmt.Fprintf(os.Stderr, "📋 Generated migration plan with %d steps\n", len(plan.Steps))

	} else {
		// Traditional mode: load plan from file
		if *planPath == "" {
			log.Fatalf("Usage: lockplane apply --plan <migration.json> [--skip-shadow]\n       lockplane apply --auto-approve --from <before.json> --to <after.json> [--validate] [--skip-shadow]")
		}

		// Load the migration plan
		loadedPlan, err := LoadJSONPlan(*planPath)
		if err != nil {
			log.Fatalf("Failed to load migration plan: %v", err)
		}
		plan = loadedPlan
	}

	// Connect to main database
	mainConnStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")

	// Detect database driver from connection string
	mainDriver, err := newDriverFromConnString(mainConnStr)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	mainDriverName := getSQLDriverName(mainDriver.Name())
	mainDB, err := sql.Open(mainDriverName, mainConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to main database: %v", err)
	}
	defer func() { _ = mainDB.Close() }()

	ctx := context.Background()
	if err := mainDB.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping main database: %v", err)
	}

	// Connect to shadow database if not skipped
	var shadowDB *sql.DB
	if !*skipShadow {
		shadowConnStr := getEnv("SHADOW_DATABASE_URL", "postgres://lockplane:lockplane@localhost:5433/lockplane?sslmode=disable")

		// Detect shadow database driver
		shadowDriver, err := newDriverFromConnString(shadowConnStr)
		if err != nil {
			log.Fatalf("Failed to create shadow database driver: %v", err)
		}

		shadowDriverName := getSQLDriverName(shadowDriver.Name())
		shadowDB, err = sql.Open(shadowDriverName, shadowConnStr)
		if err != nil {
			log.Fatalf("Failed to connect to shadow database: %v", err)
		}
		defer func() { _ = shadowDB.Close() }()

		if err := shadowDB.PingContext(ctx); err != nil {
			log.Fatalf("Failed to ping shadow database: %v", err)
		}

		fmt.Fprintf(os.Stderr, "🔍 Testing migration on shadow database...\n")
	} else {
		fmt.Fprintf(os.Stderr, "⚠️  Skipping shadow DB validation (--skip-shadow)\n")
	}

	// Apply the plan
	result, err := applyPlan(ctx, mainDB, plan, shadowDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Migration failed: %v\n\n", err)
		if len(result.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "Errors:\n")
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
		}
		os.Exit(1)
	}

	// Success!
	fmt.Fprintf(os.Stderr, "\n✅ Migration applied successfully!\n")
	fmt.Fprintf(os.Stderr, "   Steps applied: %d\n", result.StepsApplied)

	// Output result as JSON
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal result to JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}

func runValidate(args []string) {
	if len(args) == 0 {
		log.Fatalf("Usage: lockplane validate <command> [options]")
	}

	switch args[0] {
	case "schema":
		runValidateSchema(args[1:])
	default:
		log.Fatalf("Unknown validate command %q", args[0])
	}
}

func runValidateSchema(args []string) {
	fs := flag.NewFlagSet("validate schema", flag.ExitOnError)
	fileFlag := fs.String("file", "", "Path to schema JSON file")
	fileShort := fs.String("f", "", "Path to schema JSON file (shorthand)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *fileFlag != "" && *fileShort != "" && *fileFlag != *fileShort {
		log.Fatalf("Specify schema file only once (use either --file or -f)")
	}

	path := *fileFlag
	if path == "" {
		path = *fileShort
	}
	if path == "" && fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	if path == "" {
		log.Fatalf("Usage: lockplane validate schema --file <schema.json>")
	}

	if err := ValidateJSONSchema(path); err != nil {
		log.Fatalf("Schema validation failed: %v", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Schema JSON is valid: %s\n", path)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func printHelp() {
	fmt.Print(`Lockplane - Safe, AI-friendly schema management for Postgres and SQLite

USAGE:
  lockplane <command> [options]

COMMANDS:
  introspect       Introspect database and output current schema as JSON
  diff             Compare two schemas and show differences
  plan             Generate migration plan from schema diff (with --validate flag)
  apply            Apply migration plan to database (validates on shadow DB first)
  rollback         Generate rollback plan from forward migration
  validate         Validate schema JSON files
  version          Show version information
  help             Show this help message

EXAMPLES:
  # Introspect current database
  lockplane introspect > current.json

  # Compare schemas
  lockplane diff before.json after.json

  # Generate and validate migration plan
  lockplane plan --from current.json --to desired.json --validate > migration.json

  # Apply migration (tests on shadow DB first, then applies to main DB)
  lockplane apply --plan migration.json

  # Auto-approve: generate plan and apply in one command
  lockplane apply --auto-approve --from current.json --to desired.json --validate

  # Generate rollback plan
  lockplane rollback --plan migration.json --from current.json > rollback.json

  # Validate a schema file against the JSON Schema
  lockplane validate schema desired.json

ENVIRONMENT:
  DATABASE_URL            Database connection string for main database
                          Postgres: postgres://user:pass@localhost:5432/dbname?sslmode=disable
                          SQLite: file:path/to/database.db or :memory:
                          (default: postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable)

  SHADOW_DATABASE_URL     Database connection string for shadow database
                          Same format as DATABASE_URL
                          (default: postgres://lockplane:lockplane@localhost:5433/lockplane?sslmode=disable)

SUPPORTED DATABASES:
  - PostgreSQL (full support)
  - SQLite (with some limitations on ALTER operations)

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
	defer func() {
		_ = tx.Rollback() // Always rollback shadow DB changes
	}()

	// Execute each step
	for i, step := range plan.Steps {
		_, err := tx.ExecContext(ctx, step.SQL)
		if err != nil {
			return fmt.Errorf("shadow DB step %d (%s) failed: %w", i, step.Description, err)
		}
	}

	return nil
}
