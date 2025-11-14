package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/introspect"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/schema"
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

	// Load forward plan
	if rollbackVerbose {
		fmt.Fprintf(os.Stderr, "📄 Loading forward plan from: %s\n", rollbackPlan)
	}
	forwardPlan, err := loadPlan(rollbackPlan)
	if err != nil {
		log.Fatalf("Failed to load forward plan: %v", err)
	}
	if rollbackVerbose {
		fmt.Fprintf(os.Stderr, "✓ Loaded forward plan with %d steps\n", len(forwardPlan.Steps))
	}

	// Resolve target database connection
	targetConnStr, err := resolveConnection(cfg, rollbackTarget, rollbackTargetEnv, "target")
	if err != nil {
		log.Fatalf("Failed to resolve target database: %v", err)
	}

	// Load before schema (required for rollback generation)
	fromInput := rollbackFrom
	if fromInput == "" {
		if rollbackFromEnv != "" {
			resolvedFrom, err := config.ResolveEnvironment(cfg, rollbackFromEnv)
			if err != nil {
				log.Fatalf("Failed to resolve from environment: %v", err)
			}
			fromInput = resolvedFrom.DatabaseURL
		} else {
			log.Fatal("--from or --from-environment is required")
		}
	}

	driverType := executor.DetectDriver(targetConnStr)
	driver, err := executor.NewDriver(driverType)
	if err != nil {
		log.Fatalf("Failed to create driver: %v", err)
	}

	dialect := schema.DriverNameToDialect(driverType)
	opts := executor.BuildSchemaLoadOptions(fromInput, dialect)
	if rollbackVerbose {
		fmt.Fprintf(os.Stderr, "🔍 Loading before schema from: %s\n", fromInput)
	}
	beforeSchema, err := executor.LoadSchemaOrIntrospectWithOptions(fromInput, opts)
	if err != nil {
		log.Fatalf("Failed to load before schema: %v", err)
	}
	if rollbackVerbose {
		fmt.Fprintf(os.Stderr, "✓ Before schema has %d tables\n", len(beforeSchema.Tables))
	}

	// Generate rollback plan
	if rollbackVerbose {
		fmt.Fprintf(os.Stderr, "🔍 Generating rollback plan...\n")
	}
	rollbackPlan, err := planner.GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		log.Fatalf("Failed to generate rollback plan: %v", err)
	}

	if len(rollbackPlan.Steps) == 0 {
		fmt.Println("✅ No rollback needed - forward plan has no reversible operations")
		return
	}

	if rollbackVerbose {
		fmt.Fprintf(os.Stderr, "✓ Generated rollback plan with %d steps\n", len(rollbackPlan.Steps))
	}

	// Resolve shadow database (if not skipped)
	var shadowConnStr string
	if !rollbackSkipShadow {
		shadowConnStr, err = resolveConnection(cfg, rollbackShadowDB, rollbackShadowEnv, "shadow_db")
		if err != nil {
			if rollbackVerbose {
				fmt.Fprintf(os.Stderr, "⚠️  Shadow DB not available: %v\n", err)
			}
			// Shadow DB is optional, continue without it
		}
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

	// Introspect current schema
	if rollbackVerbose {
		fmt.Fprintf(os.Stderr, "🔍 Introspecting current database schema...\n")
	}
	currentSchema, err := executor.LoadSchemaFromConnectionString(targetConnStr)
	if err != nil {
		log.Fatalf("Failed to introspect current schema: %v", err)
	}

	// Open shadow database connection (if available)
	var shadowDB *sql.DB
	if shadowConnStr != "" {
		shadowDB, err = sql.Open(sqlDriverName, shadowConnStr)
		if err != nil {
			log.Fatalf("Failed to connect to shadow database: %v", err)
		}
		defer func() { _ = shadowDB.Close() }()

		if err := shadowDB.PingContext(ctx); err != nil {
			log.Fatalf("Failed to ping shadow database: %v", err)
		}
	}

	// Show rollback plan summary
	fmt.Println("\n📋 Rollback Plan Summary:")
	fmt.Printf("  Steps: %d\n", len(rollbackPlan.Steps))
	for i, step := range rollbackPlan.Steps {
		fmt.Printf("  %d. %s\n", i+1, step.Description)
	}
	fmt.Println()

	// Require approval unless auto-approved
	if !rollbackAutoApprove {
		fmt.Print("Proceed with rollback? (yes/no): ")
		var response string
		_, err = fmt.Scanln(&response)
		if err != nil {
			log.Fatalf("Failed to read input: %v", err)
		}
		if response != "yes" && response != "y" {
			fmt.Println("Cancelled")
			return
		}
	}

	// Execute rollback plan
	fmt.Println("\n🚀 Executing rollback...")
	result, err := executor.ApplyPlan(ctx, targetDB, rollbackPlan, shadowDB, currentSchema, driver, rollbackVerbose)
	if err != nil {
		fmt.Printf("\n❌ Rollback failed: %v\n", err)
		if !result.Success {
			fmt.Println("\nErrors:")
			for _, errMsg := range result.Errors {
				fmt.Printf("  • %s\n", errMsg)
			}
		}
		os.Exit(1)
	}

	if !result.Success {
		fmt.Printf("\n❌ Rollback failed:\n")
		for _, errMsg := range result.Errors {
			fmt.Printf("  • %s\n", errMsg)
		}
		os.Exit(1)
	}

	// Success
	_, _ = color.New(color.FgGreen).Printf("\n✅ Rollback complete!\n")
	fmt.Printf("Applied %d steps successfully\n", result.StepsApplied)
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
	forwardPlan, err := loadPlan(planRollbackPlan)
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
	driverType := "postgres" // default
	if introspect.IsConnectionString(fromInput) {
		driverType = executor.DetectDriver(fromInput)
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
