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
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/schema"
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
	applyTarget      string
	applyTargetEnv   string
	applySchema      string
	applyAutoApprove bool
	applySkipShadow  bool
	applyShadowDB    string
	applyShadowEnv   string
	applyVerbose     bool
)

func init() {
	rootCmd.AddCommand(applyCmd)

	applyCmd.Flags().StringVar(&applyTarget, "target", "", "Target database URL")
	applyCmd.Flags().StringVar(&applyTargetEnv, "target-environment", "", "Target environment name")
	applyCmd.Flags().StringVar(&applySchema, "schema", "", "Schema file/directory")
	applyCmd.Flags().BoolVar(&applyAutoApprove, "auto-approve", false, "Skip interactive approval")
	applyCmd.Flags().BoolVar(&applySkipShadow, "skip-shadow", false, "Skip shadow DB validation (not recommended)")
	applyCmd.Flags().StringVar(&applyShadowDB, "shadow-db", "", "Shadow database URL")
	applyCmd.Flags().StringVar(&applyShadowEnv, "shadow-environment", "", "Shadow environment")
	applyCmd.Flags().BoolVarP(&applyVerbose, "verbose", "v", false, "Verbose logging")
}

func runApply(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var plan *planner.Plan

	// Mode 1: Apply pre-generated plan file
	if len(args) > 0 {
		planPath := args[0]
		if applyVerbose {
			fmt.Fprintf(os.Stderr, "📄 Loading plan from: %s\n", planPath)
		}
		plan, err = loadPlan(planPath)
		if err != nil {
			log.Fatalf("Failed to load plan: %v", err)
		}
		if applyVerbose {
			fmt.Fprintf(os.Stderr, "✓ Loaded plan with %d steps\n", len(plan.Steps))
		}
	} else {
		// Mode 2 or 3: Generate plan from schema
		// Determine schema path
		schemaPath := applySchema
		if schemaPath == "" {
			// Mode 3: Auto-detect schema directory
			if info, err := os.Stat("schema"); err == nil && info.IsDir() {
				schemaPath = "schema"
				if applyVerbose {
					fmt.Fprintf(os.Stderr, "🔍 Auto-detected schema directory: schema/\n")
				}
			} else {
				log.Fatalf("No plan file provided and no schema found. Use: lockplane apply <plan.json> or lockplane apply --schema <path>")
			}
		}

		// Resolve target database connection
		targetConnStr, err := resolveConnection(cfg, applyTarget, applyTargetEnv, "target")
		if err != nil {
			log.Fatalf("Failed to resolve target database: %v", err)
		}

		// Load current schema from database
		if applyVerbose {
			fmt.Fprintf(os.Stderr, "🔍 Introspecting current database schema...\n")
		}
		currentSchema, err := executor.LoadSchemaFromConnectionString(targetConnStr)
		if err != nil {
			log.Fatalf("Failed to introspect current schema: %v", err)
		}
		if applyVerbose {
			fmt.Fprintf(os.Stderr, "✓ Current schema has %d tables\n", len(currentSchema.Tables))
		}

		// Load desired schema
		driverType := executor.DetectDriver(targetConnStr)
		driver, err := executor.NewDriver(driverType)
		if err != nil {
			log.Fatalf("Failed to create driver: %v", err)
		}

		dialect := schema.DriverNameToDialect(driverType)
		opts := executor.BuildSchemaLoadOptions(schemaPath, dialect)
		if applyVerbose {
			fmt.Fprintf(os.Stderr, "🔍 Loading desired schema from: %s\n", schemaPath)
		}
		desiredSchema, err := executor.LoadSchemaOrIntrospectWithOptions(schemaPath, opts)
		if err != nil {
			log.Fatalf("Failed to load desired schema: %v", err)
		}
		if applyVerbose {
			fmt.Fprintf(os.Stderr, "✓ Desired schema has %d tables\n", len(desiredSchema.Tables))
		}

		// Generate plan
		diff := schema.DiffSchemas(currentSchema, desiredSchema)
		if applyVerbose {
			fmt.Fprintf(os.Stderr, "🔍 Generating migration plan...\n")
		}
		plan, err = planner.GeneratePlanWithHash(diff, currentSchema, driver)
		if err != nil {
			log.Fatalf("Failed to generate plan: %v", err)
		}

		if len(plan.Steps) == 0 {
			fmt.Println("✅ No changes needed - database schema matches desired schema")
			return
		}

		if applyVerbose {
			fmt.Fprintf(os.Stderr, "✓ Generated plan with %d steps\n", len(plan.Steps))
		}
	}

	// Resolve target database connection
	targetConnStr, err := resolveConnection(cfg, applyTarget, applyTargetEnv, "target")
	if err != nil {
		log.Fatalf("Failed to resolve target database: %v", err)
	}

	// Resolve shadow database (if not skipped)
	var shadowConnStr string
	if !applySkipShadow {
		shadowConnStr, err = resolveConnection(cfg, applyShadowDB, applyShadowEnv, "shadow_db")
		if err != nil {
			if applyVerbose {
				fmt.Fprintf(os.Stderr, "⚠️  Shadow DB not available: %v\n", err)
			}
			// Shadow DB is optional, continue without it
		}
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

	// Introspect current schema
	if applyVerbose {
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

	// Show plan summary
	fmt.Println("\n📋 Migration Plan Summary:")
	fmt.Printf("  Steps: %d\n", len(plan.Steps))
	for i, step := range plan.Steps {
		fmt.Printf("  %d. %s\n", i+1, step.Description)
	}
	fmt.Println()

	// Require approval unless auto-approved
	if !applyAutoApprove {
		fmt.Print("Proceed with migration? (yes/no): ")
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

	// Execute plan
	fmt.Println("\n🚀 Executing migration...")
	result, err := executor.ApplyPlan(ctx, targetDB, plan, shadowDB, currentSchema, driver, applyVerbose)
	if err != nil {
		fmt.Printf("\n❌ Migration failed: %v\n", err)
		if !result.Success {
			fmt.Println("\nErrors:")
			for _, errMsg := range result.Errors {
				fmt.Printf("  • %s\n", errMsg)
			}
		}
		os.Exit(1)
	}

	if !result.Success {
		fmt.Printf("\n❌ Migration failed:\n")
		for _, errMsg := range result.Errors {
			fmt.Printf("  • %s\n", errMsg)
		}
		os.Exit(1)
	}

	// Success
	_, _ = color.New(color.FgGreen).Printf("\n✅ Migration complete!\n")
	fmt.Printf("Applied %d steps successfully\n", result.StepsApplied)
}

func loadPlan(path string) (*planner.Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan planner.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// Basic validation
	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}

	return &plan, nil
}
