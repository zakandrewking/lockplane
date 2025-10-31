package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
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

// getVersionInfo returns version information, preferring goreleaser values
// but falling back to VCS info from debug.BuildInfo (for go install builds)
func getVersionInfo() (v, c, d string) {
	v, c, d = version, commit, date

	// If goreleaser set the version, use those values
	if version != "dev" {
		return
	}

	// Otherwise, try to get VCS info from build metadata (go install / go build)
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	var revision string
	var modified bool
	var buildTime string

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		case "vcs.time":
			buildTime = setting.Value
		}
	}

	// Use short commit hash (first 7 chars like git)
	if len(revision) > 7 {
		revision = revision[:7]
	}

	if revision != "" {
		c = revision
		if modified {
			c += " (modified)"
		}
	}

	if buildTime != "" {
		d = buildTime
	}

	return
}

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
		v, c, d := getVersionInfo()
		fmt.Printf("lockplane %s\n", v)
		fmt.Printf("  commit: %s\n", c)
		fmt.Printf("  built:  %s\n", d)
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
	case "convert":
		runConvert(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown subcommand '%s'\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func runIntrospect(args []string) {
	// Load config file (if it exists)
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	fs := flag.NewFlagSet("introspect", flag.ExitOnError)
	dbURL := fs.String("db", "", "Database connection string (overrides env var and config file)")
	format := fs.String("format", "json", "Output format: json or sql")

	// Custom usage function
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane introspect [options]\n\n")
		fmt.Fprintf(os.Stderr, "Introspect a database and output its schema.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nConfiguration Priority:\n")
		fmt.Fprintf(os.Stderr, "  1. --db flag (highest)\n")
		fmt.Fprintf(os.Stderr, "  2. DATABASE_URL environment variable\n")
		fmt.Fprintf(os.Stderr, "  3. database_url in lockplane.toml\n")
		fmt.Fprintf(os.Stderr, "  4. Default value (lowest)\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  # Introspect to JSON (default)\n")
		fmt.Fprintf(os.Stderr, "  lockplane introspect > schema.json\n\n")
		fmt.Fprintf(os.Stderr, "  # Introspect to SQL DDL\n")
		fmt.Fprintf(os.Stderr, "  lockplane introspect --format sql > lockplane/schema.lp.sql\n\n")
		fmt.Fprintf(os.Stderr, "  # Specify database connection directly\n")
		fmt.Fprintf(os.Stderr, "  lockplane introspect --db postgresql://localhost:5432/myapp?sslmode=disable > schema.json\n\n")
		fmt.Fprintf(os.Stderr, "  # Introspect Supabase local database to SQL\n")
		fmt.Fprintf(os.Stderr, "  lockplane introspect --db postgresql://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable --format sql > schema.lp.sql\n\n")
	}

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	// Priority: --db flag > DATABASE_URL env var > config file > default
	connStr := GetDatabaseURL(*dbURL, config, "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")

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

	// Output in requested format
	switch *format {
	case "json":
		jsonBytes, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal schema to JSON: %v", err)
		}
		fmt.Println(string(jsonBytes))

	case "sql":
		// Generate SQL DDL from schema
		// Use PostgreSQL driver for SQL generation (works for most databases)
		sqlDriver := postgres.NewDriver()
		var sqlBuilder strings.Builder

		for _, table := range schema.Tables {
			sql, _ := sqlDriver.CreateTable(table)
			sqlBuilder.WriteString(sql)
			sqlBuilder.WriteString(";\n\n")

			// Add indexes
			for _, idx := range table.Indexes {
				sql, _ := sqlDriver.AddIndex(table.Name, idx)
				sqlBuilder.WriteString(sql)
				sqlBuilder.WriteString(";\n")
			}

			if len(table.Indexes) > 0 {
				sqlBuilder.WriteString("\n")
			}

			// Add foreign keys
			for _, fk := range table.ForeignKeys {
				sql, _ := sqlDriver.AddForeignKey(table.Name, fk)
				if !strings.HasPrefix(sql, "--") { // Skip comment-only SQL
					sqlBuilder.WriteString(sql)
					sqlBuilder.WriteString(";\n")
				}
			}

			if len(table.ForeignKeys) > 0 {
				sqlBuilder.WriteString("\n")
			}
		}

		fmt.Print(sqlBuilder.String())

	default:
		log.Fatalf("Unsupported format: %s (use 'json' or 'sql')", *format)
	}
}

func runDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if fs.NArg() != 2 {
		log.Fatalf("Usage: lockplane diff <before> <after>")
	}

	beforePath := fs.Arg(0)
	afterPath := fs.Arg(1)

	// Load schemas
	before, err := LoadSchema(beforePath)
	if err != nil {
		log.Fatalf("Failed to load before schema: %v", err)
	}

	after, err := LoadSchema(afterPath)
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
	fromSchema := fs.String("from", "", "Source schema path (file or directory)")
	toSchema := fs.String("to", "", "Target schema path (file or directory)")
	validate := fs.Bool("validate", false, "Validate migration safety and reversibility")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	// Generate diff first
	var diff *SchemaDiff
	var before *Schema
	var after *Schema

	if *fromSchema != "" && *toSchema != "" {
		// Generate diff from two schemas (supports files, directories, or database connection strings)
		var err error
		before, err = LoadSchemaOrIntrospect(*fromSchema)
		if err != nil {
			log.Fatalf("Failed to load from schema: %v", err)
		}

		after, err = LoadSchemaOrIntrospect(*toSchema)
		if err != nil {
			log.Fatalf("Failed to load to schema: %v", err)
		}

		diff = DiffSchemas(before, after)
	} else {
		log.Fatalf("Usage: lockplane plan --from <before.json|db-url> --to <after.json|db-url> [--validate]")
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

	// Generate plan with source hash
	plan, err := GeneratePlanWithHash(diff, before)
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
	fromSchema := fs.String("from", "", "Source schema path (before state)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *planPath == "" || *fromSchema == "" {
		log.Fatalf("Usage: lockplane rollback --plan <forward.json> --from <before.json|db-url>")
	}

	// Load the forward plan
	forwardPlan, err := LoadJSONPlan(*planPath)
	if err != nil {
		log.Fatalf("Failed to load forward plan: %v", err)
	}

	// Load the before schema (supports files, directories, or database connection strings)
	beforeSchema, err := LoadSchemaOrIntrospect(*fromSchema)
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
	// Load config file (if it exists)
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	planPath := fs.String("plan", "", "Migration plan file to apply")
	skipShadow := fs.Bool("skip-shadow", false, "Skip shadow DB validation (not recommended)")
	autoApprove := fs.Bool("auto-approve", false, "Automatically generate and apply plan from --from and --to schemas")
	fromSchema := fs.String("from", "", "Source schema path (before) - used with --auto-approve")
	toSchema := fs.String("to", "", "Target schema path (after) - used with --auto-approve")
	validate := fs.Bool("validate", false, "Validate migration safety and reversibility - used with --auto-approve")
	dbURL := fs.String("db", "", "Main database connection string (overrides env var and config file)")
	shadowDBURL := fs.String("shadow-db", "", "Shadow database connection string (overrides env var and config file)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	var plan *Plan

	// Auto-approve mode: generate plan in-memory from schemas
	if *autoApprove {
		if *fromSchema == "" || *toSchema == "" {
			fmt.Fprintf(os.Stderr, "Error: --auto-approve requires both --from and --to arguments\n\n")
			fmt.Fprintf(os.Stderr, "Usage:\n")
			fmt.Fprintf(os.Stderr, "  lockplane apply --auto-approve --from <before.json|db-url> --to <after.json|db-url> [--validate] [--skip-shadow]\n\n")
			fmt.Fprintf(os.Stderr, "Examples:\n")
			fmt.Fprintf(os.Stderr, "  lockplane apply --auto-approve --from $DATABASE_URL --to schema/\n")
			fmt.Fprintf(os.Stderr, "  lockplane apply --auto-approve --from current.json --to schema/ --validate\n")
			os.Exit(1)
		}

		// Load schemas (supports files, directories, or database connection strings)
		before, err := LoadSchemaOrIntrospect(*fromSchema)
		if err != nil {
			log.Fatalf("Failed to load from schema: %v", err)
		}

		after, err := LoadSchemaOrIntrospect(*toSchema)
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

		// Generate plan with source hash
		generatedPlan, err := GeneratePlanWithHash(diff, before)
		if err != nil {
			log.Fatalf("Failed to generate plan: %v", err)
		}
		plan = generatedPlan

		fmt.Fprintf(os.Stderr, "📋 Generated migration plan with %d steps\n", len(plan.Steps))

	} else {
		// Traditional mode: load plan from file
		if *planPath == "" {
			fmt.Fprintf(os.Stderr, "Error: Missing required arguments\n\n")
			fmt.Fprintf(os.Stderr, "Usage: lockplane apply [options]\n\n")
			fmt.Fprintf(os.Stderr, "Two main ways to apply migrations:\n\n")
			fmt.Fprintf(os.Stderr, "1. Apply from a migration plan file:\n")
			fmt.Fprintf(os.Stderr, "   lockplane apply --plan <migration.json> [--skip-shadow]\n\n")
			fmt.Fprintf(os.Stderr, "2. Auto-approve: generate and apply in one step:\n")
			fmt.Fprintf(os.Stderr, "   lockplane apply --auto-approve --from <before.json|db-url> --to <after.json|db-url> [--validate] [--skip-shadow]\n\n")
			fmt.Fprintf(os.Stderr, "Examples:\n")
			fmt.Fprintf(os.Stderr, "  lockplane apply --plan migration.json\n")
			fmt.Fprintf(os.Stderr, "  lockplane apply --auto-approve --from $DATABASE_URL --to schema/\n")
			os.Exit(1)
		}

		// Load the migration plan
		loadedPlan, err := LoadJSONPlan(*planPath)
		if err != nil {
			log.Fatalf("Failed to load migration plan: %v", err)
		}
		plan = loadedPlan
	}

	// Connect to main database
	// Priority: --db flag > DATABASE_URL env var > config file > default
	mainConnStr := GetDatabaseURL(*dbURL, config, "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")

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
		// Priority: --shadow-db flag > SHADOW_DATABASE_URL env var > config file > default
		shadowConnStr := GetShadowDatabaseURL(*shadowDBURL, config, "postgres://lockplane:lockplane@localhost:5433/lockplane?sslmode=disable")

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

	// Validate source hash if present in plan
	if plan.SourceHash != "" {
		fmt.Fprintf(os.Stderr, "🔐 Validating source schema hash...\n")

		// Introspect current database state
		currentSchema, err := mainDriver.IntrospectSchema(ctx, mainDB)
		if err != nil {
			log.Fatalf("Failed to introspect current database schema: %v", err)
		}

		// Compute hash of current state
		currentHash, err := ComputeSchemaHash((*Schema)(currentSchema))
		if err != nil {
			log.Fatalf("Failed to compute current schema hash: %v", err)
		}

		// Compare hashes
		if currentHash != plan.SourceHash {
			fmt.Fprintf(os.Stderr, "\n❌ Source schema mismatch!\n\n")
			fmt.Fprintf(os.Stderr, "The migration plan was generated for a different database state.\n")
			fmt.Fprintf(os.Stderr, "This usually happens when:\n")
			fmt.Fprintf(os.Stderr, "  - The plan is being applied to the wrong database\n")
			fmt.Fprintf(os.Stderr, "  - The database has been modified since the plan was generated\n")
			fmt.Fprintf(os.Stderr, "  - The plan is being applied out of order\n\n")
			fmt.Fprintf(os.Stderr, "Expected source hash: %s\n", plan.SourceHash)
			fmt.Fprintf(os.Stderr, "Current database hash: %s\n\n", currentHash)
			fmt.Fprintf(os.Stderr, "To fix this:\n")
			fmt.Fprintf(os.Stderr, "  1. Introspect the current database: lockplane introspect > current.json\n")
			fmt.Fprintf(os.Stderr, "  2. Generate a new plan: lockplane plan --from current.json --to desired.lp.sql\n")
			fmt.Fprintf(os.Stderr, "  3. Apply the new plan: lockplane apply --plan migration.json\n\n")
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "✓ Source schema hash matches (hash: %s...)\n", currentHash[:12])
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
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(os.Stderr, "Usage: lockplane validate <command> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Validate schema and plan files in different formats.\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  schema    Validate JSON schema file against JSON Schema\n")
		fmt.Fprintf(os.Stderr, "  sql       Validate SQL DDL file or directory of .lp.sql files\n")
		fmt.Fprintf(os.Stderr, "  plan      Validate migration plan JSON file\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  # Validate JSON schema\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate schema schema.json\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate SQL schema\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate sql schema.lp.sql\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate migration plan\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate plan migration.json\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate with JSON output (for IDE integration)\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate sql --format json schema.lp.sql\n\n")
		if len(args) == 0 {
			os.Exit(1)
		}
		return
	}

	switch args[0] {
	case "schema":
		runValidateSchema(args[1:])
	case "sql":
		runValidateSQL(args[1:])
	case "plan":
		runValidatePlan(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown validate subcommand %q\n\n", args[0])
		fmt.Fprintf(os.Stderr, "Usage: lockplane validate <command> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  schema    Validate JSON schema file against JSON Schema\n")
		fmt.Fprintf(os.Stderr, "  sql       Validate SQL DDL file or directory of .lp.sql files\n")
		fmt.Fprintf(os.Stderr, "  plan      Validate migration plan JSON file\n\n")
		os.Exit(1)
	}
}

func runValidateSchema(args []string) {
	fs := flag.NewFlagSet("validate schema", flag.ExitOnError)
	fileFlag := fs.String("file", "", "Path to schema JSON file")
	fileShort := fs.String("f", "", "Path to schema JSON file (shorthand)")

	// Custom usage function
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane validate schema [options] <file>\n\n")
		fmt.Fprintf(os.Stderr, "Validate a JSON schema file against the Lockplane JSON Schema.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Validate JSON schema file\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate schema schema.json\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate using --file flag\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate schema --file schema.json\n\n")
	}

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
		fs.Usage()
		os.Exit(1)
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
  convert          Convert schema between SQL DDL (.lp.sql) and JSON formats
  validate         Validate schema and plan files (schema, sql, plan subcommands)
  version          Show version information
  help             Show this help message

EXAMPLES:
  # Introspect current database
  lockplane introspect > current.json

  # Compare schemas
  lockplane diff current.json schema/

  # Generate and validate migration plan from files
  lockplane plan --from current.json --to schema/ --validate > migration.json

  # Generate plan using database connection strings (auto-introspect)
  lockplane plan --from postgres://user:pass@localhost/db1 --to schema/ --validate > migration.json

  # Generate plan comparing two live databases
  lockplane plan --from $DATABASE_URL --to postgres://user:pass@localhost/db2 > migration.json

  # Apply migration (tests on shadow DB first, then applies to main DB)
  lockplane apply --plan migration.json

  # Auto-approve: generate plan and apply in one command with files
  lockplane apply --auto-approve --from current.json --to schema/ --validate

  # Auto-approve: generate plan from database and apply to target schema
  lockplane apply --auto-approve --from $DATABASE_URL --to schema/ --validate

  # Generate rollback plan
  lockplane rollback --plan migration.json --from current.json > rollback.json

  # Generate rollback using database connection string
  lockplane rollback --plan migration.json --from $DATABASE_URL > rollback.json

  # Validate SQL schema file
  lockplane validate sql schema.lp.sql

  # Validate JSON schema file
  lockplane validate schema schema.json

  # Validate migration plan
  lockplane validate plan migration.json

  # Convert SQL DDL to JSON
  lockplane convert --input schema.lp.sql --output schema.json

  # Convert a directory of SQL files to JSON
  lockplane convert --input schema/ --output schema.json

  # Convert JSON to SQL DDL
  lockplane convert --input schema.json --output schema.lp.sql --to sql

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
	SourceHash string     `json:"source_hash"`
	Steps      []PlanStep `json:"steps"`
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

func runConvert(args []string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	input := fs.String("input", "", "Input schema (.lp.sql file, directory, or .json)")
	output := fs.String("output", "", "Output file (defaults to stdout)")
	_ = fs.String("from", "", "Input format: sql or json (auto-detected if not specified)")
	toFormat := fs.String("to", "json", "Output format: json or sql")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane convert --input <file> [--output <file>] [--from <format>] [--to <format>]\n\n")
		fmt.Fprintf(os.Stderr, "Convert schema between SQL DDL and JSON formats.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Convert SQL to JSON\n")
		fmt.Fprintf(os.Stderr, "  lockplane convert --input schema.lp.sql --output schema.json\n\n")
		fmt.Fprintf(os.Stderr, "  # Convert a directory of .lp.sql files to JSON\n")
		fmt.Fprintf(os.Stderr, "  lockplane convert --input schema/ --output schema.json\n\n")
		fmt.Fprintf(os.Stderr, "  # Convert JSON to SQL\n")
		fmt.Fprintf(os.Stderr, "  lockplane convert --input schema.json --output schema.lp.sql --to sql\n\n")
		fmt.Fprintf(os.Stderr, "  # Output to stdout\n")
		fmt.Fprintf(os.Stderr, "  lockplane convert --input schema.lp.sql\n")
	}

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *input == "" {
		fs.Usage()
		log.Fatal("--input is required")
	}

	// Load the schema
	schema, err := LoadSchema(*input)
	if err != nil {
		log.Fatalf("Failed to load schema: %v", err)
	}

	// Convert to target format
	var outputData []byte
	switch *toFormat {
	case "json":
		outputData, err = json.MarshalIndent(schema, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal JSON: %v", err)
		}

	case "sql":
		// Generate SQL DDL from schema
		driver := postgres.NewDriver() // Use PostgreSQL SQL generator
		var sqlBuilder strings.Builder

		for _, table := range schema.Tables {
			sql, _ := driver.CreateTable(table)
			sqlBuilder.WriteString(sql)
			sqlBuilder.WriteString(";\n\n")

			// Add indexes
			for _, idx := range table.Indexes {
				sql, _ := driver.AddIndex(table.Name, idx)
				sqlBuilder.WriteString(sql)
				sqlBuilder.WriteString(";\n")
			}

			if len(table.Indexes) > 0 {
				sqlBuilder.WriteString("\n")
			}

			// Add foreign keys
			for _, fk := range table.ForeignKeys {
				sql, _ := driver.AddForeignKey(table.Name, fk)
				if !strings.HasPrefix(sql, "--") { // Skip comment-only SQL
					sqlBuilder.WriteString(sql)
					sqlBuilder.WriteString(";\n")
				}
			}

			if len(table.ForeignKeys) > 0 {
				sqlBuilder.WriteString("\n")
			}
		}

		outputData = []byte(sqlBuilder.String())

	default:
		log.Fatalf("Unsupported output format: %s (use 'json' or 'sql')", *toFormat)
	}

	// Write output
	if *output == "" {
		// Write to stdout
		fmt.Print(string(outputData))
	} else {
		if err := os.WriteFile(*output, outputData, 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		fmt.Printf("Converted %s to %s: %s\n", *input, *toFormat, *output)
	}
}
