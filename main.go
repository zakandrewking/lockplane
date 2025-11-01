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
	dbURL := fs.String("db", "", "Database connection string (overrides environment selection)")
	format := fs.String("format", "json", "Output format: json or sql")
	sourceEnv := fs.String("source-environment", "", "Named environment to introspect (defaults to config default)")
	useShadow := fs.Bool("shadow", false, "Use the shadow database URL for the selected environment")

	// Custom usage function
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane introspect [options]\n\n")
		fmt.Fprintf(os.Stderr, "Introspect a database and output its schema.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nConfiguration Priority:\n")
		fmt.Fprintf(os.Stderr, "  1. --db flag (highest)\n")
		fmt.Fprintf(os.Stderr, "  2. --source-environment or default environment from lockplane.toml\n")
		fmt.Fprintf(os.Stderr, "  3. Built-in defaults (postgres on localhost)\n\n")
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

	connStr := strings.TrimSpace(*dbURL)
	var resolvedEnv *ResolvedEnvironment
	if connStr == "" {
		resolvedEnv, err = ResolveEnvironment(config, *sourceEnv)
		if err != nil {
			log.Fatalf("Failed to resolve source environment: %v", err)
		}
		connStr = resolvedEnv.DatabaseURL
		if *useShadow {
			connStr = resolvedEnv.ShadowDatabaseURL
			if connStr == "" {
				log.Fatalf("Environment %q does not define a shadow database URL", resolvedEnv.Name)
			}
		}
	}

	if connStr == "" {
		envName := defaultEnvironmentName
		if resolvedEnv != nil {
			envName = resolvedEnv.Name
		} else if config != nil && config.DefaultEnvironment != "" {
			envName = config.DefaultEnvironment
		}
		log.Fatalf("No database connection configured. Provide --db or configure environment %q in lockplane.toml / .env.%s.", envName, envName)
	}

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
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	beforeEnv := fs.String("before-environment", "", "Environment providing the before-state database connection")
	afterEnv := fs.String("after-environment", "", "Environment providing the after-state database connection")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	var beforeArg, afterArg string
	if fs.NArg() > 0 {
		beforeArg = fs.Arg(0)
	}
	if fs.NArg() > 1 {
		afterArg = fs.Arg(1)
	}

	if beforeArg == "" {
		env, err := ResolveEnvironment(config, *beforeEnv)
		if err != nil {
			log.Fatalf("Failed to resolve before environment: %v", err)
		}
		beforeArg = env.DatabaseURL
	}

	if afterArg == "" {
		env, err := ResolveEnvironment(config, *afterEnv)
		if err != nil {
			log.Fatalf("Failed to resolve after environment: %v", err)
		}
		afterArg = env.DatabaseURL
	}

	if beforeArg == "" || afterArg == "" {
		log.Fatalf("Usage: lockplane diff <before> <after>\n       lockplane diff --before-environment <name> --after-environment <name>")
	}

	// Load schemas
	before, err := LoadSchemaOrIntrospect(beforeArg)
	if err != nil {
		log.Fatalf("Failed to load before schema: %v", err)
	}

	after, err := LoadSchemaOrIntrospect(afterArg)
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
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	fromSchema := fs.String("from", "", "Source schema path (file or directory)")
	toSchema := fs.String("to", "", "Target schema path (file or directory)")
	fromEnvironment := fs.String("from-environment", "", "Environment providing the source database connection")
	toEnvironment := fs.String("to-environment", "", "Environment providing the target database connection")
	validate := fs.Bool("validate", false, "Validate migration safety and reversibility")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	fromInput := strings.TrimSpace(*fromSchema)
	toInput := strings.TrimSpace(*toSchema)

	if fromInput == "" {
		resolvedFrom, err := ResolveEnvironment(config, *fromEnvironment)
		if err != nil {
			log.Fatalf("Failed to resolve source environment: %v", err)
		}
		fromInput = resolvedFrom.DatabaseURL
		if fromInput == "" {
			fmt.Fprintf(os.Stderr, "Error: environment %q does not define a source database. Provide --from or configure .env.%s.\n", resolvedFrom.Name, resolvedFrom.Name)
			os.Exit(1)
		}
	}

	if toInput == "" {
		resolvedTo, err := ResolveEnvironment(config, *toEnvironment)
		if err != nil {
			log.Fatalf("Failed to resolve target environment: %v", err)
		}
		toInput = resolvedTo.DatabaseURL
		if toInput == "" {
			fmt.Fprintf(os.Stderr, "Error: environment %q does not define a target database. Provide --to or configure .env.%s.\n", resolvedTo.Name, resolvedTo.Name)
			os.Exit(1)
		}
	}

	if fromInput == "" || toInput == "" {
		log.Fatalf("Usage: lockplane plan --from <before.json|db> --to <after.json|db> [--validate]\n\n       lockplane plan --from-environment <name> --to <schema.json>\n       lockplane plan --from <schema.json> --to-environment <name>")
	}

	// Generate diff first
	var diff *SchemaDiff
	var before *Schema
	var after *Schema

	var loadErr error
	before, loadErr = LoadSchemaOrIntrospect(fromInput)
	if loadErr != nil {
		log.Fatalf("Failed to load from schema: %v", loadErr)
	}

	after, loadErr = LoadSchemaOrIntrospect(toInput)
	if loadErr != nil {
		log.Fatalf("Failed to load to schema: %v", loadErr)
	}

	diff = DiffSchemas(before, after)

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
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	planPath := fs.String("plan", "", "Forward migration plan file")
	fromSchema := fs.String("from", "", "Source schema path (before state)")
	fromEnvironment := fs.String("from-environment", "", "Environment providing the before-state database connection")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *planPath == "" {
		log.Fatalf("Usage: lockplane rollback --plan <forward.json> --from <before.json|db> [--from-environment <name>]")
	}

	sourceInput := strings.TrimSpace(*fromSchema)
	if sourceInput == "" {
		resolved, err := ResolveEnvironment(config, *fromEnvironment)
		if err != nil {
			log.Fatalf("Failed to resolve source environment: %v", err)
		}
		sourceInput = resolved.DatabaseURL
		if sourceInput == "" {
			fmt.Fprintf(os.Stderr, "Error: environment %q does not define a database connection. Provide --from or configure .env.%s.\n", resolved.Name, resolved.Name)
			os.Exit(1)
		}
	}

	if sourceInput == "" {
		log.Fatalf("Usage: lockplane rollback --plan <forward.json> --from <before.json|db> [--from-environment <name>]")
	}

	// Load the forward plan
	forwardPlan, err := LoadJSONPlan(*planPath)
	if err != nil {
		log.Fatalf("Failed to load forward plan: %v", err)
	}

	// Load the before schema (supports files, directories, or database connection strings)
	beforeSchema, err := LoadSchemaOrIntrospect(sourceInput)
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
	target := fs.String("target", "", "Target database URL to apply migration to (overrides environment settings)")
	targetEnvironment := fs.String("target-environment", "", "Named environment providing the target database connection (defaults to lockplane.toml)")
	schema := fs.String("schema", "", "Desired schema file/directory (required for plan generation)")
	autoApprove := fs.Bool("auto-approve", false, "Skip interactive approval of migration plan")
	skipShadow := fs.Bool("skip-shadow", false, "Skip shadow DB validation (not recommended)")
	shadowDBURL := fs.String("shadow-db", "", "Shadow database connection string (overrides environment settings)")
	shadowEnvironment := fs.String("shadow-environment", "", "Named environment providing the shadow database connection (defaults to target environment)")

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	// Get positional arguments (for plan file)
	positionalArgs := fs.Args()

	if value := strings.TrimSpace(*target); value != "" && strings.HasPrefix(value, "--") {
		fmt.Fprintf(os.Stderr, "Error: --target flag is missing its value. Provide a database URL or remove the flag to use --target-environment.\n\n")
		os.Exit(1)
	}

	if value := strings.TrimSpace(*schema); value != "" && strings.HasPrefix(value, "--") {
		fmt.Fprintf(os.Stderr, "Error: --schema flag has invalid value %q\n\n", value)
		fmt.Fprintf(os.Stderr, "Check that the preceding flag has its argument.\n\n")
		os.Exit(1)
	}

	resolvedTarget, err := ResolveEnvironment(config, *targetEnvironment)
	if err != nil {
		log.Fatalf("Failed to resolve target environment: %v", err)
	}

	targetConnStr := strings.TrimSpace(*target)
	if targetConnStr == "" {
		targetConnStr = resolvedTarget.DatabaseURL
	}
	if targetConnStr == "" {
		fmt.Fprintf(os.Stderr, "Error: no target database configured.\n\n")
		fmt.Fprintf(os.Stderr, "Provide --target or configure environment %q via lockplane.toml/.env.%s.\n", resolvedTarget.Name, resolvedTarget.Name)
		os.Exit(1)
	}

	var plan *Plan
	var planFile string

	// Check if first positional arg is a plan file
	if len(positionalArgs) > 0 {
		planFile = positionalArgs[0]

		// Check if user accidentally passed a schema file instead of a plan file
		if strings.HasSuffix(planFile, ".sql") || strings.HasSuffix(planFile, ".lp.sql") {
			fmt.Fprintf(os.Stderr, "Error: '%s' appears to be a schema file, not a migration plan.\n\n", planFile)
			fmt.Fprintf(os.Stderr, "Did you mean to use --schema?\n\n")
			fmt.Fprintf(os.Stderr, "  lockplane apply --target-environment %s --schema %s\n\n", resolvedTarget.Name, planFile)
			fmt.Fprintf(os.Stderr, "Or to generate and save a plan first:\n\n")
			fmt.Fprintf(os.Stderr, "  lockplane plan --from-environment %s --to %s --output plan.json\n", resolvedTarget.Name, planFile)
			fmt.Fprintf(os.Stderr, "  lockplane apply plan.json --target-environment %s\n\n", resolvedTarget.Name)
			os.Exit(1)
		}

		// Warn if --schema was also provided (might be confusing)
		if *schema != "" {
			fmt.Fprintf(os.Stderr, "Warning: Ignoring --schema flag when applying a pre-generated plan file\n")
			fmt.Fprintf(os.Stderr, "         The plan file (%s) already contains the migration steps\n\n", planFile)
		}

		// Load plan from file
		loadedPlan, err := LoadJSONPlan(planFile)
		if err != nil {
			log.Fatalf("Failed to load migration plan: %v", err)
		}
		plan = loadedPlan

		fmt.Fprintf(os.Stderr, "📋 Loaded migration plan with %d steps from %s\n", len(plan.Steps), planFile)
	} else {
		// Generate plan mode - require --schema
		schemaPath := strings.TrimSpace(*schema)
		if schemaPath == "" {
			schemaPath = GetSchemaPath("", config, resolvedTarget, "")
		}
		if schemaPath == "" {
			fmt.Fprintf(os.Stderr, "Error: --schema required when generating a plan.\n\n")
			fmt.Fprintf(os.Stderr, "Set schema_path in lockplane.toml or provide the flag explicitly.\n\n")
			printApplyUsage()
			os.Exit(1)
		}

		// Introspect target database (this is the "from" state)
		fmt.Fprintf(os.Stderr, "🔍 Introspecting target database (%s)...\n", resolvedTarget.Name)
		before, err := LoadSchemaOrIntrospect(targetConnStr)
		if err != nil {
			log.Fatalf("Failed to introspect target database: %v", err)
		}

		// Load desired schema (this is the "to" state)
		fmt.Fprintf(os.Stderr, "📖 Loading desired schema from %s...\n", schemaPath)
		after, err := LoadSchemaOrIntrospect(schemaPath)
		if err != nil {
			log.Fatalf("Failed to load schema: %v", err)
		}

		// Generate diff
		diff := DiffSchemas(before, after)

		// Check if there are any changes
		if diff.IsEmpty() {
			fmt.Fprintf(os.Stderr, "\n✓ No changes detected - database already matches desired schema\n")
			os.Exit(0)
		}

		// Generate plan with source hash
		generatedPlan, err := GeneratePlanWithHash(diff, before)
		if err != nil {
			log.Fatalf("Failed to generate plan: %v", err)
		}
		plan = generatedPlan

		// Print plan details
		fmt.Fprintf(os.Stderr, "\n📋 Migration plan (%d steps):\n\n", len(plan.Steps))
		for i, step := range plan.Steps {
			fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, step.Description)
			if step.SQL != "" {
				// Truncate SQL for display
				sql := step.SQL
				if len(sql) > 100 {
					sql = sql[:100] + "..."
				}
				fmt.Fprintf(os.Stderr, "     SQL: %s\n", sql)
			}
		}
		fmt.Fprintf(os.Stderr, "\n")

		// Ask for confirmation unless --auto-approve
		if !*autoApprove {
			fmt.Fprintf(os.Stderr, "Do you want to perform these actions?\n")
			fmt.Fprintf(os.Stderr, "  Lockplane will perform the actions described above.\n")
			fmt.Fprintf(os.Stderr, "  Only 'yes' will be accepted to approve.\n\n")
			fmt.Fprintf(os.Stderr, "  Enter a value: ")

			var response string
			_, err := fmt.Scanln(&response)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nApply cancelled.\n")
				os.Exit(0)
			}

			if response != "yes" {
				fmt.Fprintf(os.Stderr, "\nApply cancelled.\n")
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	// Connect to main database
	// Detect database driver from connection string
	mainDriver, err := newDriverFromConnString(targetConnStr)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	mainDriverName := getSQLDriverName(mainDriver.Name())
	mainDB, err := sql.Open(mainDriverName, targetConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to target database: %v", err)
	}
	defer func() { _ = mainDB.Close() }()

	ctx := context.Background()
	if err := mainDB.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping target database: %v", err)
	}

	// Connect to shadow database if not skipped
	var shadowDB *sql.DB
	if !*skipShadow {
		shadowConnStr := strings.TrimSpace(*shadowDBURL)
		if shadowConnStr == "" {
			shadowEnvName := strings.TrimSpace(*shadowEnvironment)
			if shadowEnvName == "" {
				shadowEnvName = resolvedTarget.Name
			}
			resolvedShadow, err := ResolveEnvironment(config, shadowEnvName)
			if err != nil {
				log.Fatalf("Failed to resolve shadow environment: %v", err)
			}
			shadowConnStr = resolvedShadow.ShadowDatabaseURL
			if shadowConnStr == "" {
				fmt.Fprintf(os.Stderr, "Error: no shadow database configured for environment %q.\n", resolvedShadow.Name)
				fmt.Fprintf(os.Stderr, "Add SHADOW_DATABASE_URL to .env.%s or provide --shadow-db.\n", resolvedShadow.Name)
				os.Exit(1)
			}
		}

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

func printApplyUsage() {
	fmt.Fprintf(os.Stderr, "Error: Missing required arguments\n\n")
	fmt.Fprintf(os.Stderr, "Usage: lockplane apply [plan.json] [options]\n\n")
	fmt.Fprintf(os.Stderr, "Three ways to use lockplane apply:\n\n")
	fmt.Fprintf(os.Stderr, "1. Plan, review, and apply (interactive):\n")
	fmt.Fprintf(os.Stderr, "   lockplane apply --target-environment local --schema schema/\n")
	fmt.Fprintf(os.Stderr, "   (Introspects target, generates plan, shows it, asks for confirmation)\n\n")
	fmt.Fprintf(os.Stderr, "2. Apply a pre-generated plan:\n")
	fmt.Fprintf(os.Stderr, "   lockplane apply plan.json --target-environment local\n")
	fmt.Fprintf(os.Stderr, "   (Loads plan from file and applies it)\n\n")
	fmt.Fprintf(os.Stderr, "3. Plan and apply without confirmation:\n")
	fmt.Fprintf(os.Stderr, "   lockplane apply --auto-approve --target-environment local --schema schema/\n")
	fmt.Fprintf(os.Stderr, "   (Introspects target, generates plan, applies immediately)\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  --target <url>         Target database URL (overrides environment settings)\n")
	fmt.Fprintf(os.Stderr, "  --target-environment <name>\n")
	fmt.Fprintf(os.Stderr, "                         Environment providing the target database connection\n")
	fmt.Fprintf(os.Stderr, "  --schema <path>        Desired schema file/directory (required for plan generation)\n")
	fmt.Fprintf(os.Stderr, "  --auto-approve         Skip interactive approval\n")
	fmt.Fprintf(os.Stderr, "  --skip-shadow          Skip shadow DB validation (not recommended)\n")
	fmt.Fprintf(os.Stderr, "  --shadow-db <url>      Shadow database URL (overrides environment settings)\n")
	fmt.Fprintf(os.Stderr, "  --shadow-environment <name>\n")
	fmt.Fprintf(os.Stderr, "                         Environment providing the shadow database connection\n\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  lockplane apply --target-environment local --schema schema/\n")
	fmt.Fprintf(os.Stderr, "  lockplane apply plan.json --target-environment local\n")
	fmt.Fprintf(os.Stderr, "  lockplane apply --target mydriver://user:pass@host/db --schema schema/\n")
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
  lockplane plan --from postgres://user:pass@localhost/db1 --to postgres://user:pass@localhost/db2 > migration.json

  # Generate plan using named environments
  lockplane plan --from-environment local --to-environment staging --validate > migration.json

  # Apply migration interactively (plan, review, confirm)
  lockplane apply --target-environment local --schema schema/

  # Apply a pre-generated plan
  lockplane apply plan.json --target-environment local

  # Auto-approve: plan and apply without confirmation
  lockplane apply --auto-approve --target-environment local --schema schema/

  # Generate rollback plan
  lockplane rollback --plan migration.json --from current.json > rollback.json

  # Generate rollback using database connection string
  lockplane rollback --plan migration.json --from-environment local > rollback.json

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

ENVIRONMENTS:
  Configure environments in lockplane.toml (default environment: "local").
  Each environment can define connection strings inline or load them from .env.<name> files
  containing DATABASE_URL and SHADOW_DATABASE_URL entries.

  Example:
    default_environment = "local"

    [environments.local]
      description = "Local development"

  Use --target-environment, --from-environment, and --source-environment flags to select the
  environment when running commands. Without configuration, Lockplane falls back to the bundled
  defaults:
    Database: postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable
    Shadow:   postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable

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
