package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/schema"
	"github.com/lockplane/lockplane/internal/sqliteutil"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply [plan.json]",
	Short: "Apply a migration plan to a database",
	Long: `Apply a migration plan to a target database with shadow DB validation.

Three modes of operation:
  1. Apply a pre-generated plan file: lockplane apply plan.json
  2. Generate and apply from schema: lockplane apply --schema schema/ --target-environment local
  3. Auto-detect and apply: lockplane apply --target-environment local (auto-detects schema/)`,
	Example: `  # Apply a pre-generated plan
  lockplane apply migration.json --target-environment local

  # Generate and apply from schema
  lockplane apply --schema schema/ --target-environment local --auto-approve

  # Auto-detect schema and apply
  lockplane apply --target-environment local`,
	Run: runApply,
}

var (
	applyTarget       string
	applyTargetEnv    string
	applySchema       string
	applyAutoApprove  bool
	applySkipShadow   bool
	applyShadowDB     string
	applyShadowSchema string
	applyVerbose      bool
)

func init() {
	rootCmd.AddCommand(applyCmd)

	applyCmd.Flags().StringVar(&applyTarget, "target", "", "Target database URL")
	applyCmd.Flags().StringVar(&applyTargetEnv, "target-environment", "", "Target environment name")
	applyCmd.Flags().StringVar(&applySchema, "schema", "", "Schema file/directory")
	applyCmd.Flags().BoolVar(&applyAutoApprove, "auto-approve", false, "Skip interactive approval")
	applyCmd.Flags().BoolVar(&applySkipShadow, "skip-shadow", false, "Skip shadow DB validation (not recommended)")
	applyCmd.Flags().StringVar(&applyShadowDB, "shadow-db", "", "Shadow database URL")
	applyCmd.Flags().StringVar(&applyShadowSchema, "shadow-schema", "", "Shadow schema name (PostgreSQL only)")
	applyCmd.Flags().BoolVarP(&applyVerbose, "verbose", "v", false, "Verbose logging")
}

func runApply(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Resolve target environment first (needed for error messages)
	resolvedTarget, err := config.ResolveEnvironment(cfg, applyTargetEnv)
	if err != nil {
		log.Fatalf("Failed to resolve target environment: %v", err)
	}

	// Validate target flag value
	if strings.TrimSpace(applyTarget) != "" && strings.HasPrefix(strings.TrimSpace(applyTarget), "--") {
		fmt.Fprintf(os.Stderr, "Error: --target flag is missing its value. Provide a database URL or remove the flag to use --target-environment.\n\n")
		os.Exit(1)
	}

	// Validate schema flag value
	if strings.TrimSpace(applySchema) != "" && strings.HasPrefix(strings.TrimSpace(applySchema), "--") {
		fmt.Fprintf(os.Stderr, "Error: --schema flag has invalid value %q\n\n", applySchema)
		fmt.Fprintf(os.Stderr, "Check that the preceding flag has its argument.\n\n")
		os.Exit(1)
	}

	var plan *planner.Plan

	// Mode 1: Apply pre-generated plan file
	if len(args) > 0 {
		planPath := args[0]

		// Check if user accidentally passed a schema file instead of a plan file
		if strings.HasSuffix(planPath, ".sql") || strings.HasSuffix(planPath, ".lp.sql") {
			fmt.Fprintf(os.Stderr, "Error: '%s' appears to be a schema file, not a migration plan.\n\n", planPath)
			fmt.Fprintf(os.Stderr, "Did you mean to use --schema?\n\n")
			fmt.Fprintf(os.Stderr, "  lockplane apply --target-environment %s --schema %s\n\n", resolvedTarget.Name, planPath)
			fmt.Fprintf(os.Stderr, "Or to generate and save a plan first:\n\n")
			fmt.Fprintf(os.Stderr, "  lockplane plan --from-environment %s --to %s > plan.json\n", resolvedTarget.Name, planPath)
			fmt.Fprintf(os.Stderr, "  lockplane apply plan.json --target-environment %s\n\n", resolvedTarget.Name)
			os.Exit(1)
		}

		// Warn if --schema was also provided
		if applySchema != "" {
			fmt.Fprintf(os.Stderr, "Warning: Ignoring --schema flag when applying a pre-generated plan file\n")
			fmt.Fprintf(os.Stderr, "         The plan file (%s) already contains the migration steps\n\n", planPath)
		}

		if applyVerbose {
			fmt.Fprintf(os.Stderr, "📄 Loading plan from: %s\n", planPath)
		}
		plan, err = planner.LoadJSONPlan(planPath)
		if err != nil {
			log.Fatalf("Failed to load migration plan: %v", err)
		}
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "📋 Loaded migration plan with %d steps from %s\n", len(plan.Steps), planPath)
	} else {
		// Mode 2 or 3: Generate plan from schema
		// Determine schema path
		schemaPath := strings.TrimSpace(applySchema)
		if schemaPath == "" {
			// Try config first
			if resolvedTarget.SchemaPath != "" {
				schemaPath = resolvedTarget.SchemaPath
			}
		}
		if schemaPath == "" {
			// Mode 3: Auto-detect schema directory
			if info, err := os.Stat("schema"); err == nil && info.IsDir() {
				schemaPath = "schema"
				fmt.Fprintf(os.Stderr, "ℹ️  Auto-detected schema directory: schema/\n")
			}
		}

		if schemaPath == "" {
			fmt.Fprintf(os.Stderr, "Error: --schema required when generating a plan.\n\n")
			fmt.Fprintf(os.Stderr, "Set schema_path in lockplane.toml or provide the flag explicitly.\n\n")
			os.Exit(1)
		}

		// Resolve target database connection
		targetConnStr := strings.TrimSpace(applyTarget)
		if targetConnStr == "" {
			targetConnStr = resolvedTarget.DatabaseURL
		}
		if targetConnStr == "" {
			fmt.Fprintf(os.Stderr, "Error: no target database configured.\n\n")
			fmt.Fprintf(os.Stderr, "Provide --target or configure environment %q via lockplane.toml/.env.%s.\n", resolvedTarget.Name, resolvedTarget.Name)
			os.Exit(1)
		}

		// Load current schema from database
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Introspecting target database (%s)...\n", resolvedTarget.Name)
		before, err := executor.LoadSchemaFromConnectionString(targetConnStr)
		if err != nil {
			log.Fatalf("Failed to introspect target database: %v", err)
		}

		// Load desired schema
		driverType := executor.DetectDriver(targetConnStr)
		driver, err := executor.NewDriver(driverType)
		if err != nil {
			log.Fatalf("Failed to create database driver: %v", err)
		}

		dialect := schema.DriverNameToDialect(driverType)
		opts := executor.BuildSchemaLoadOptions(schemaPath, dialect)
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "📖 Loading desired schema from %s...\n", schemaPath)
		after, err := executor.LoadSchemaOrIntrospectWithOptions(schemaPath, opts)
		if err != nil {
			log.Fatalf("Failed to load schema: %v", err)
		}

		// Generate diff
		diff := schema.DiffSchemas(before, after)

		// Check if there are any changes
		if diff.IsEmpty() {
			_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "\n✓ No changes detected - database already matches desired schema\n")
			os.Exit(0)
		}

		// Generate plan with source hash
		generatedPlan, err := planner.GeneratePlanWithHash(diff, before, driver)
		if err != nil {
			log.Fatalf("Failed to generate plan: %v", err)
		}

		plan = generatedPlan

		// Print plan details with colors
		cyan := color.New(color.FgCyan, color.Bold)
		green := color.New(color.FgGreen)
		yellow := color.New(color.FgYellow)
		gray := color.New(color.FgHiBlack)

		_, _ = cyan.Fprintf(os.Stderr, "\n📋 Migration plan (%d steps):\n\n", len(plan.Steps))

		for i, step := range plan.Steps {
			_, _ = green.Fprintf(os.Stderr, "  %d. ", i+1)
			fmt.Fprintf(os.Stderr, "%s\n", step.Description)
			if len(step.SQL) > 0 {
				if len(step.SQL) == 1 {
					sql := step.SQL[0]
					if len(sql) > 100 {
						sql = sql[:100] + "..."
					}
					_, _ = gray.Fprintf(os.Stderr, "     SQL: ")
					_, _ = yellow.Fprintf(os.Stderr, "%s\n", sql)
				} else {
					_, _ = gray.Fprintf(os.Stderr, "     SQL: ")
					_, _ = yellow.Fprintf(os.Stderr, "%d statements\n", len(step.SQL))
				}
			}
		}
		fmt.Fprintf(os.Stderr, "\n")

		// Ask for confirmation unless --auto-approve
		if !applyAutoApprove {
			bold := color.New(color.Bold)
			red := color.New(color.FgRed)
			_, _ = bold.Fprintf(os.Stderr, "Do you want to perform these actions?\n")
			fmt.Fprintf(os.Stderr, "  Lockplane will perform the actions described above.\n")
			_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "  Only 'yes' will be accepted to approve.\n\n")
			fmt.Fprintf(os.Stderr, "  Enter a value: ")

			var response string
			_, err := fmt.Scanln(&response)
			if err != nil {
				_, _ = red.Fprintf(os.Stderr, "\nApply cancelled.\n")
				os.Exit(0)
			}
			if response != "yes" {
				_, _ = red.Fprintf(os.Stderr, "\nApply cancelled.\n")
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	// Resolve target database connection
	targetConnStr := strings.TrimSpace(applyTarget)
	if targetConnStr == "" {
		targetConnStr = resolvedTarget.DatabaseURL
	}
	if targetConnStr == "" {
		fmt.Fprintf(os.Stderr, "Error: no target database configured.\n\n")
		fmt.Fprintf(os.Stderr, "Provide --target or configure environment %q via lockplane.toml/.env.%s.\n", resolvedTarget.Name, resolvedTarget.Name)
		os.Exit(1)
	}

	// Detect database driver
	driverType := executor.DetectDriver(targetConnStr)
	driver, err := executor.NewDriver(driverType)
	if err != nil {
		log.Fatalf("Failed to create driver: %v", err)
	}

	// Open target database connection
	sqlDriverName := executor.GetSQLDriverName(driverType)
	targetDB, err := sql.Open(sqlDriverName, targetConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to target database: %v", err)
	}
	defer func() { _ = targetDB.Close() }()

	// Ping to verify connection
	if err := targetDB.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping target database: %v", err)
	}

	// Connect to shadow database if not skipped
	var shadowDB *sql.DB
	var shadowSchema string
	if !applySkipShadow {
		shadowConnStr := strings.TrimSpace(applyShadowDB)
		shadowSchema = strings.TrimSpace(applyShadowSchema)
		resolvedShadow := resolvedTarget

		if shadowConnStr == "" {
			shadowConnStr = resolvedShadow.ShadowDatabaseURL
		}
		if shadowSchema == "" {
			shadowSchema = resolvedShadow.ShadowSchema
		}
		if shadowSchema != "" && shadowConnStr == "" {
			// Reuse the main database when only a schema override is provided.
			shadowConnStr = targetConnStr
		}

		// For SQLite/libSQL, default to :memory: if no shadow DB configured
		if shadowConnStr == "" && (driverType == "sqlite" || driverType == "sqlite3" || driverType == "libsql") {
			shadowConnStr = ":memory:"
			_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "ℹ️  Using in-memory shadow database (fast, zero config)\n")
		} else if shadowConnStr == "" {
			fmt.Fprintf(os.Stderr, "Error: no shadow database configured for environment %q.\n", resolvedShadow.Name)
			fmt.Fprintf(os.Stderr, "Options:\n")
			fmt.Fprintf(os.Stderr, "  - Add SHADOW_DATABASE_URL to .env.%s\n", resolvedShadow.Name)
			fmt.Fprintf(os.Stderr, "  - Add/override SHADOW_SCHEMA (or --shadow-schema) to reuse the primary database\n")
			fmt.Fprintf(os.Stderr, "  - Provide --shadow-db flag\n")
			os.Exit(1)
		}

		// Detect shadow database driver type
		shadowDriverType := executor.DetectDriver(shadowConnStr)

		// For SQLite shadow DB (not :memory:), check if the database file exists and create it if needed
		if (shadowDriverType == "sqlite" || shadowDriverType == "sqlite3") && shadowConnStr != ":memory:" {
			if err := sqliteutil.EnsureSQLiteDatabase(shadowConnStr, "shadow", false); err != nil {
				log.Fatalf("Failed to ensure shadow database: %v", err)
			}
		}

		shadowDriverName := executor.GetSQLDriverName(shadowDriverType)
		shadowDB, err = sql.Open(shadowDriverName, shadowConnStr)
		if err != nil {
			log.Fatalf("Failed to connect to shadow database: %v", err)
		}
		defer func() { _ = shadowDB.Close() }()

		if err := shadowDB.PingContext(ctx); err != nil {
			log.Fatalf("Failed to ping shadow database: %v", err)
		}

		// If shadow schema is configured and driver supports it, set up the schema
		if shadowSchema != "" && driver.SupportsSchemas() {
			// Create shadow schema if it doesn't exist
			if err := driver.CreateSchema(ctx, shadowDB, shadowSchema); err != nil {
				log.Fatalf("Failed to create shadow schema: %v", err)
			}

			// Set search path to shadow schema
			if err := driver.SetSchema(ctx, shadowDB, shadowSchema); err != nil {
				log.Fatalf("Failed to set shadow schema: %v", err)
			}

			// Show clear message about what we're doing
			if shadowConnStr == targetConnStr {
				_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Testing migration on shadow schema %q (same database)...\n", shadowSchema)
			} else {
				_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Testing migration on shadow schema %q in separate database...\n", shadowSchema)
			}
		} else if shadowConnStr == ":memory:" {
			_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Testing migration on in-memory shadow database...\n")
		} else {
			_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Testing migration on shadow database...\n")
		}
	} else {
		_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "⚠️  Skipping shadow DB validation (--skip-shadow)\n")
	}

	// Introspect current database state (needed for shadow DB validation and source hash check)
	currentSchema, err := driver.IntrospectSchema(ctx, targetDB)
	if err != nil {
		log.Fatalf("Failed to introspect current database schema: %v", err)
	}

	// Validate source hash if present in plan
	if plan.SourceHash != "" {
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔐 Validating source schema hash...\n")

		// Compute hash of current state
		currentHash, err := schema.ComputeSchemaHash((*database.Schema)(currentSchema))
		if err != nil {
			log.Fatalf("Failed to compute current schema hash: %v", err)
		}

		// Compare hashes
		if currentHash != plan.SourceHash {
			red := color.New(color.FgRed, color.Bold)
			yellow := color.New(color.FgYellow)

			_, _ = red.Fprintf(os.Stderr, "\n❌ Source schema mismatch!\n\n")
			fmt.Fprintf(os.Stderr, "The migration plan was generated for a different database state.\n")
			fmt.Fprintf(os.Stderr, "This usually happens when:\n")
			fmt.Fprintf(os.Stderr, "  - The plan is being applied to the wrong database\n")
			fmt.Fprintf(os.Stderr, "  - The database has been modified since the plan was generated\n")
			fmt.Fprintf(os.Stderr, "  - The plan is being applied out of order\n\n")
			_, _ = yellow.Fprintf(os.Stderr, "Expected source hash: %s\n", plan.SourceHash)
			_, _ = yellow.Fprintf(os.Stderr, "Current database hash: %s\n\n", currentHash)
			_, _ = color.New(color.FgCyan, color.Bold).Fprintf(os.Stderr, "To fix this:\n")
			fmt.Fprintf(os.Stderr, "  1. Introspect the current database: lockplane introspect > current.json\n")
			fmt.Fprintf(os.Stderr, "  2. Generate a new plan: lockplane plan --from current.json --to desired.lp.sql\n")
			fmt.Fprintf(os.Stderr, "  3. Apply the new plan: lockplane apply --plan migration.json\n\n")
			os.Exit(1)
		}

		_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "✓ Source schema hash matches (hash: %s...)\n", currentHash[:12])
	}

	// Apply the plan
	if applyVerbose {
		_, _ = color.New(color.FgCyan, color.Bold).Fprintf(os.Stderr, "\n🚀 Applying migration...\n\n")
	}

	result, err := executor.ApplyPlan(ctx, targetDB, plan, shadowDB, (*database.Schema)(currentSchema), driver, applyVerbose)
	if err != nil {
		red := color.New(color.FgRed, color.Bold)
		_, _ = red.Fprintf(os.Stderr, "\n❌ Migration failed: %v\n\n", err)
		if len(result.Errors) > 0 {
			_, _ = red.Fprintf(os.Stderr, "Errors:\n")
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
		}
		os.Exit(1)
	}

	// Success!
	green := color.New(color.FgGreen, color.Bold)
	_, _ = green.Fprintf(os.Stderr, "\n✅ Migration applied successfully!\n")
	_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "   Steps applied: %d\n", result.StepsApplied)

	// Output result as JSON
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal result to JSON: %v", err)
	}
	fmt.Println(string(jsonBytes))
}
