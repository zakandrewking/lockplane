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

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Generate and apply a rollback migration",
	Long: `Generate a rollback plan from a forward migration plan and apply it to the target database.

This command generates a rollback plan and applies it in one step, with shadow DB validation.

The --from flag specifies the "before" schema state (before the forward migration was applied).
This is required to generate the rollback plan correctly.`,
	Example: `  # Rollback a migration
  lockplane rollback --plan migration.json --from before.json --target-environment local

  # Rollback using environment for before state
  lockplane rollback --plan migration.json --from-environment staging --target-environment production`,
	Run: runRollback,
}

var planRollbackCmd = &cobra.Command{
	Use:   "plan-rollback",
	Short: "Generate a rollback plan from a forward migration plan",
	Long: `Generate a rollback plan from a forward migration plan.

The plan-rollback command generates a reversible migration plan that undoes
a forward migration. It outputs a plan JSON file that can be reviewed,
saved, and applied later using 'lockplane apply'.`,
	Example: `  # Generate rollback plan
  lockplane plan-rollback --plan migration.json --from before.json > rollback.json

  # Use environment for before state
  lockplane plan-rollback --plan migration.json --from-environment staging > rollback.json`,
	Run: runPlanRollback,
}

var (
	rollbackPlan        string
	rollbackFrom        string
	rollbackFromEnv     string
	rollbackTarget      string
	rollbackTargetEnv   string
	rollbackAutoApprove bool
	rollbackSkipShadow  bool
	rollbackShadowDB    string
	rollbackShadowEnv   string
	rollbackVerbose     bool

	planRollbackPlan    string
	planRollbackFrom    string
	planRollbackFromEnv string
	planRollbackVerbose bool
)

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(planRollbackCmd)

	// rollback command flags
	rollbackCmd.Flags().StringVar(&rollbackPlan, "plan", "", "Forward migration plan file (required)")
	rollbackCmd.Flags().StringVar(&rollbackFrom, "from", "", "Before schema (file/directory/database URL)")
	rollbackCmd.Flags().StringVar(&rollbackFromEnv, "from-environment", "", "Environment providing the before schema")
	rollbackCmd.Flags().StringVar(&rollbackTarget, "target", "", "Target database URL")
	rollbackCmd.Flags().StringVar(&rollbackTargetEnv, "target-environment", "", "Target environment name")
	rollbackCmd.Flags().BoolVar(&rollbackAutoApprove, "auto-approve", false, "Skip interactive approval")
	rollbackCmd.Flags().BoolVar(&rollbackSkipShadow, "skip-shadow", false, "Skip shadow DB validation (not recommended)")
	rollbackCmd.Flags().StringVar(&rollbackShadowDB, "shadow-db", "", "Shadow database URL")
	rollbackCmd.Flags().StringVar(&rollbackShadowEnv, "shadow-environment", "", "Shadow environment")
	rollbackCmd.Flags().BoolVarP(&rollbackVerbose, "verbose", "v", false, "Verbose logging")
	_ = rollbackCmd.MarkFlagRequired("plan")

	// plan-rollback command flags
	planRollbackCmd.Flags().StringVar(&planRollbackPlan, "plan", "", "Forward migration plan file (required)")
	planRollbackCmd.Flags().StringVar(&planRollbackFrom, "from", "", "Before schema (file/directory/database URL)")
	planRollbackCmd.Flags().StringVar(&planRollbackFromEnv, "from-environment", "", "Environment providing the before schema")
	planRollbackCmd.Flags().BoolVarP(&planRollbackVerbose, "verbose", "v", false, "Verbose logging")
	_ = planRollbackCmd.MarkFlagRequired("plan")
}

func runRollback(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Resolve target environment
	resolvedTarget, err := config.ResolveEnvironment(cfg, rollbackTargetEnv)
	if err != nil {
		log.Fatalf("Failed to resolve target environment: %v", err)
	}

	// Load the forward plan
	if rollbackVerbose {
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "📖 Loading forward plan: %s\n", rollbackPlan)
	}
	forwardPlan, err := planner.LoadJSONPlan(rollbackPlan)
	if err != nil {
		log.Fatalf("Failed to load forward plan: %v", err)
	}

	// Resolve target database connection
	targetConnStr := strings.TrimSpace(rollbackTarget)
	if targetConnStr == "" {
		targetConnStr = resolvedTarget.DatabaseURL
	}
	if targetConnStr == "" {
		fmt.Fprintf(os.Stderr, "Error: no target database configured.\n\n")
		fmt.Fprintf(os.Stderr, "Provide --target or configure environment %q via lockplane.toml/.env.%s.\n", resolvedTarget.Name, resolvedTarget.Name)
		os.Exit(1)
	}

	// Determine source ("before") schema for rollback generation
	sourceInput := strings.TrimSpace(rollbackFrom)
	if sourceInput == "" && rollbackFromEnv != "" {
		resolvedFrom, err := config.ResolveEnvironment(cfg, rollbackFromEnv)
		if err != nil {
			log.Fatalf("Failed to resolve from environment: %v", err)
		}
		sourceInput = resolvedFrom.DatabaseURL
	}

	var beforeSchema *database.Schema
	if sourceInput != "" {
		// Load schema from file or database
		if rollbackVerbose {
			_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Loading 'before' schema from: %s\n", sourceInput)
		}
		rollbackFallback := schema.DriverNameToDialect(executor.DetectDriver(sourceInput))
		beforeSchema, err = executor.LoadSchemaOrIntrospectWithOptions(sourceInput, executor.BuildSchemaLoadOptions(sourceInput, rollbackFallback))
		if err != nil {
			log.Fatalf("Failed to load before schema: %v", err)
		}
	} else {
		// No --from provided, show helpful error
		fmt.Fprintf(os.Stderr, "Error: --from or --from-environment is required for rollback generation.\n\n")
		fmt.Fprintf(os.Stderr, "The rollback command needs the schema that existed BEFORE the forward migration.\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  1. Provide --from with a schema file/directory saved before the migration\n")
		fmt.Fprintf(os.Stderr, "  2. Provide --from-environment pointing to a database with the original state\n")
		fmt.Fprintf(os.Stderr, "  3. Use plan-rollback to generate rollback plan first:\n")
		fmt.Fprintf(os.Stderr, "     lockplane plan-rollback --plan %s --from <before.json> > rollback.json\n", rollbackPlan)
		fmt.Fprintf(os.Stderr, "     lockplane apply rollback.json --target-environment %s\n\n", resolvedTarget.Name)
		os.Exit(1)
	}

	// Detect database driver from target connection string
	mainDriverType := executor.DetectDriver(targetConnStr)
	mainDriver, err := executor.NewDriver(mainDriverType)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	// Generate rollback plan
	if rollbackVerbose {
		_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "⚙️  Generating rollback plan...\n")
	}
	rollbackPlan, err := planner.GenerateRollback(forwardPlan, beforeSchema, mainDriver)
	if err != nil {
		log.Fatalf("Failed to generate rollback: %v", err)
	}

	// Display rollback plan with colors
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	gray := color.New(color.FgHiBlack)
	red := color.New(color.FgRed, color.Bold)

	_, _ = red.Fprintf(os.Stderr, "\n🔄 Rollback plan (%d steps):\n\n", len(rollbackPlan.Steps))
	fmt.Fprintf(os.Stderr, "This will UNDO the changes from the forward migration.\n\n")

	for i, step := range rollbackPlan.Steps {
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
	if !rollbackAutoApprove {
		bold := color.New(color.Bold)
		_, _ = bold.Fprintf(os.Stderr, "Do you want to perform this rollback?\n")
		fmt.Fprintf(os.Stderr, "  Lockplane will UNDO the forward migration described above.\n")
		_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "  Only 'yes' will be accepted to approve.\n\n")
		fmt.Fprintf(os.Stderr, "  Enter a value: ")

		var response string
		_, err := fmt.Scanln(&response)
		if err != nil {
			_, _ = red.Fprintf(os.Stderr, "\nRollback cancelled.\n")
			os.Exit(0)
		}

		if response != "yes" {
			_, _ = red.Fprintf(os.Stderr, "\nRollback cancelled.\n")
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Connect to main database
	mainDriverName := executor.GetSQLDriverName(mainDriverType)
	mainDB, err := sql.Open(mainDriverName, targetConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to target database: %v", err)
	}
	defer func() { _ = mainDB.Close() }()

	if err := mainDB.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping target database: %v", err)
	}

	// Connect to shadow database if not skipped
	var shadowDB *sql.DB
	var shadowSchema string
	if !rollbackSkipShadow {
		shadowConnStr := strings.TrimSpace(rollbackShadowDB)
		if shadowConnStr == "" {
			shadowEnvName := strings.TrimSpace(rollbackShadowEnv)
			if shadowEnvName == "" {
				shadowEnvName = resolvedTarget.Name
			}
			resolvedShadow, err := config.ResolveEnvironment(cfg, shadowEnvName)
			if err != nil {
				log.Fatalf("Failed to resolve shadow environment: %v", err)
			}
			shadowConnStr = resolvedShadow.ShadowDatabaseURL
			shadowSchema = resolvedShadow.ShadowSchema

			// For SQLite/libSQL, default to :memory: if no shadow DB configured
			if shadowConnStr == "" && (mainDriverType == "sqlite" || mainDriverType == "sqlite3" || mainDriverType == "libsql") {
				shadowConnStr = ":memory:"
				_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "ℹ️  Using in-memory shadow database (fast, zero config)\n")
			} else if shadowConnStr == "" {
				fmt.Fprintf(os.Stderr, "Error: no shadow database configured for environment %q.\n", resolvedShadow.Name)
				fmt.Fprintf(os.Stderr, "Options:\n")
				fmt.Fprintf(os.Stderr, "  - Add SHADOW_DATABASE_URL to .env.%s\n", resolvedShadow.Name)
				fmt.Fprintf(os.Stderr, "  - Add SHADOW_SCHEMA=lockplane_shadow (PostgreSQL only)\n")
				fmt.Fprintf(os.Stderr, "  - Provide --shadow-db flag\n")
				fmt.Fprintf(os.Stderr, "  - Use --skip-shadow to skip shadow DB validation (not recommended)\n")
				os.Exit(1)
			}
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
		if shadowSchema != "" && mainDriver.SupportsSchemas() {
			// Create shadow schema if it doesn't exist
			if err := mainDriver.CreateSchema(ctx, shadowDB, shadowSchema); err != nil {
				log.Fatalf("Failed to create shadow schema: %v", err)
			}

			// Set search path to shadow schema
			if err := mainDriver.SetSchema(ctx, shadowDB, shadowSchema); err != nil {
				log.Fatalf("Failed to set shadow schema: %v", err)
			}

			// Show clear message about what we're doing
			if shadowConnStr == targetConnStr {
				_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Testing rollback on shadow schema %q (same database)...\n", shadowSchema)
			} else {
				_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Testing rollback on shadow schema %q in separate database...\n", shadowSchema)
			}
		} else if shadowConnStr == ":memory:" {
			_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Testing rollback on in-memory shadow database...\n")
		} else {
			_, _ = color.New(color.FgCyan).Fprintf(os.Stderr, "🔍 Testing rollback on shadow database...\n")
		}
	} else {
		_, _ = color.New(color.FgYellow).Fprintf(os.Stderr, "⚠️  Skipping shadow DB validation (--skip-shadow)\n")
	}

	// Introspect current database state (needed for shadow DB validation)
	currentSchema, err := mainDriver.IntrospectSchema(ctx, mainDB)
	if err != nil {
		log.Fatalf("Failed to introspect current database schema: %v", err)
	}

	// Apply the rollback plan
	if rollbackVerbose {
		_, _ = color.New(color.FgCyan, color.Bold).Fprintf(os.Stderr, "\n🚀 Executing rollback...\n\n")
	}
	result, err := executor.ApplyPlan(ctx, mainDB, rollbackPlan, shadowDB, (*database.Schema)(currentSchema), mainDriver, rollbackVerbose)
	if err != nil {
		_, _ = red.Fprintf(os.Stderr, "\n❌ Rollback failed: %v\n\n", err)
		if len(result.Errors) > 0 {
			_, _ = red.Fprintf(os.Stderr, "Errors:\n")
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
		}
		os.Exit(1)
	}

	// Success!
	if result.Success {
		green := color.New(color.FgGreen, color.Bold)
		_, _ = green.Fprintf(os.Stderr, "\n✓ Rollback completed successfully!\n\n")
		fmt.Fprintf(os.Stderr, "  Steps completed: %d/%d\n", result.StepsApplied, len(rollbackPlan.Steps))
	}
}

func runPlanRollback(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load forward plan
	if planRollbackVerbose {
		fmt.Fprintf(os.Stderr, "📄 Loading forward plan from: %s\n", planRollbackPlan)
	}
	forwardPlan, err := planner.LoadJSONPlan(planRollbackPlan)
	if err != nil {
		log.Fatalf("Failed to load forward plan: %v", err)
	}
	if planRollbackVerbose {
		fmt.Fprintf(os.Stderr, "✓ Loaded forward plan with %d steps\n", len(forwardPlan.Steps))
	}

	// Resolve before schema
	fromInput := planRollbackFrom
	if fromInput == "" {
		if planRollbackFromEnv != "" {
			resolvedFrom, err := config.ResolveEnvironment(cfg, planRollbackFromEnv)
			if err != nil {
				log.Fatalf("Failed to resolve from environment: %v", err)
			}
			fromInput = resolvedFrom.DatabaseURL
		} else {
			log.Fatal("--from or --from-environment is required")
		}
	}

	// Detect driver from before schema (if it's a connection string)
	driverType := executor.DetectDriver(fromInput)
	if driverType == "" {
		driverType = "postgres" // default
	}

	driver, err := executor.NewDriver(driverType)
	if err != nil {
		log.Fatalf("Failed to create driver: %v", err)
	}

	dialect := schema.DriverNameToDialect(driverType)
	opts := executor.BuildSchemaLoadOptions(fromInput, dialect)
	if planRollbackVerbose {
		fmt.Fprintf(os.Stderr, "🔍 Loading before schema from: %s\n", fromInput)
	}
	beforeSchema, err := executor.LoadSchemaOrIntrospectWithOptions(fromInput, opts)
	if err != nil {
		log.Fatalf("Failed to load before schema: %v", err)
	}
	if planRollbackVerbose {
		fmt.Fprintf(os.Stderr, "✓ Before schema has %d tables\n", len(beforeSchema.Tables))
	}

	// Generate rollback plan
	if planRollbackVerbose {
		fmt.Fprintf(os.Stderr, "🔍 Generating rollback plan...\n")
	}
	rollbackPlan, err := planner.GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		log.Fatalf("Failed to generate rollback plan: %v", err)
	}

	if len(rollbackPlan.Steps) == 0 {
		fmt.Fprintf(os.Stderr, "⚠️  No rollback steps generated - forward plan has no reversible operations\n")
		// Output empty plan
		rollbackPlan = &planner.Plan{Steps: []planner.PlanStep{}}
	}

	// Output rollback plan as JSON
	jsonBytes, err := json.MarshalIndent(rollbackPlan, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal rollback plan: %v", err)
	}
	fmt.Println(string(jsonBytes))
}
