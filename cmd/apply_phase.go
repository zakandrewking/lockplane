package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/state"
	"github.com/spf13/cobra"
)

var applyPhaseCmd = &cobra.Command{
	Use:   "apply-phase <plan-file>",
	Short: "Execute a specific phase of a multi-phase migration",
	Long: `Execute a specific phase of a multi-phase migration plan.

Multi-phase migrations require coordination between database changes and
code deployments. Each phase must be executed sequentially, with code
deployments happening between phases as needed.

The command tracks state in .lockplane-state.json to ensure phases are
executed in the correct order and prevent skipping phases.`,
	Example: `  # Execute phase 1
  lockplane apply-phase rename-email.json --phase 1

  # Execute next phase (auto-detects from state)
  lockplane apply-phase rename-email.json --next

  # Force execution of a specific phase (dangerous)
  lockplane apply-phase rename-email.json --phase 2 --force`,
	Args: cobra.ExactArgs(1),
	Run:  runApplyPhase,
}

var (
	apPhase           int
	apNext            bool
	apForce           bool
	apTargetEnv       string
	apTarget          string
	apShadowDB        string
	apShadowDBEnv     string
	apSkipShadowDB    bool
	apDryRun          bool
	apVerbose         bool
	apAutoApprove     bool
	apRequireApproval bool
)

func init() {
	rootCmd.AddCommand(applyPhaseCmd)

	applyPhaseCmd.Flags().IntVar(&apPhase, "phase", 0, "Phase number to execute (1-based)")
	applyPhaseCmd.Flags().BoolVar(&apNext, "next", false, "Execute the next phase (auto-detects from state)")
	applyPhaseCmd.Flags().BoolVar(&apForce, "force", false, "Force execution, skip safety checks (dangerous)")
	applyPhaseCmd.Flags().StringVar(&apTargetEnv, "target-environment", "", "Target database environment from lockplane.toml")
	applyPhaseCmd.Flags().StringVar(&apTarget, "target", "", "Target database connection string")
	applyPhaseCmd.Flags().StringVar(&apShadowDBEnv, "shadow-db-environment", "", "Shadow database environment")
	applyPhaseCmd.Flags().StringVar(&apShadowDB, "shadow-db", "", "Shadow database connection string")
	applyPhaseCmd.Flags().BoolVar(&apSkipShadowDB, "skip-shadow-db", false, "Skip shadow database validation (not recommended)")
	applyPhaseCmd.Flags().BoolVar(&apDryRun, "dry-run", false, "Show what would be executed without applying changes")
	applyPhaseCmd.Flags().BoolVarP(&apVerbose, "verbose", "v", false, "Enable verbose logging")
	applyPhaseCmd.Flags().BoolVar(&apAutoApprove, "auto-approve", false, "Automatically approve execution without prompting")
	applyPhaseCmd.Flags().BoolVar(&apRequireApproval, "require-approval", true, "Require manual approval before executing")
}

func runApplyPhase(cmd *cobra.Command, args []string) {
	planPath := args[0]

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load multi-phase plan
	multiPhasePlan, err := loadMultiPhasePlan(planPath)
	if err != nil {
		log.Fatalf("Failed to load multi-phase plan: %v", err)
	}

	// Load state
	st, err := state.Load()
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	// Determine which phase to execute
	phaseNumber := apPhase
	if apNext {
		if apPhase != 0 {
			log.Fatal("Cannot use both --phase and --next")
		}
		phaseNumber = st.GetNextPhase()
		if phaseNumber == 0 {
			fmt.Println("✅ All phases complete!")
			return
		}
		fmt.Printf("Next phase to execute: %d\n", phaseNumber)
	}

	if phaseNumber == 0 {
		log.Fatal("Must specify --phase <number> or --next")
	}

	if phaseNumber < 1 || phaseNumber > multiPhasePlan.TotalPhases {
		log.Fatalf("Invalid phase number %d (plan has %d phases)", phaseNumber, multiPhasePlan.TotalPhases)
	}

	// Check if we can execute this phase (unless --force)
	if !apForce {
		if err := st.CanExecutePhase(phaseNumber); err != nil {
			log.Fatalf("Cannot execute phase %d: %v\nUse --force to override (dangerous)", phaseNumber, err)
		}
	}

	// Get the phase
	phase := multiPhasePlan.Phases[phaseNumber-1] // 0-indexed

	// Initialize state if this is phase 1
	if phaseNumber == 1 && st.ActiveMigration == nil {
		migrationID := fmt.Sprintf("%s_%s_%d", multiPhasePlan.Operation, multiPhasePlan.Pattern, time.Now().Unix())
		// Extract table/column from first phase if available
		table := ""
		column := ""
		if len(multiPhasePlan.Phases) > 0 {
			// Try to infer from description or operation
			table = multiPhasePlan.Operation // Placeholder
		}

		if err := st.StartMigration(migrationID, multiPhasePlan.Operation, multiPhasePlan.Pattern, table, column, multiPhasePlan.TotalPhases, planPath); err != nil {
			log.Fatalf("Failed to start migration: %v", err)
		}
		fmt.Printf("Started multi-phase migration: %s\n", migrationID)
	}

	// Display phase information
	fmt.Printf("\n")
	fmt.Printf("📋 Multi-Phase Migration: %s\n", multiPhasePlan.Description)
	fmt.Printf("Pattern: %s\n", multiPhasePlan.Pattern)
	fmt.Printf("Total Phases: %d\n", multiPhasePlan.TotalPhases)
	fmt.Printf("\n")
	fmt.Printf("🎯 Executing Phase %d/%d: %s\n", phaseNumber, multiPhasePlan.TotalPhases, phase.Name)
	fmt.Printf("Description: %s\n", phase.Description)
	fmt.Printf("\n")

	// Show code changes required
	if phase.RequiresCodeDeploy && len(phase.CodeChangesRequired) > 0 {
		fmt.Printf("⚠️  Code Changes Required:\n")
		for _, change := range phase.CodeChangesRequired {
			fmt.Printf("  • %s\n", change)
		}
		fmt.Printf("\n")
	}

	// Show SQL steps
	if phase.Plan != nil && len(phase.Plan.Steps) > 0 {
		fmt.Printf("SQL Steps:\n")
		for i, step := range phase.Plan.Steps {
			fmt.Printf("  %d. %s\n", i+1, step.Description)
			for _, sql := range step.SQL {
				fmt.Printf("     %s\n", sql)
			}
		}
		fmt.Printf("\n")
	} else {
		fmt.Printf("No SQL changes in this phase (code deployment only)\n\n")
	}

	// Show verification steps
	if len(phase.Verification) > 0 {
		fmt.Printf("Verification Steps:\n")
		for _, v := range phase.Verification {
			fmt.Printf("  • %s\n", v)
		}
		fmt.Printf("\n")
	}

	// Dry run mode
	if apDryRun {
		fmt.Println("🔍 DRY RUN: No changes will be applied")
		return
	}

	// Require approval unless auto-approved
	if apRequireApproval && !apAutoApprove {
		fmt.Printf("Proceed with phase %d execution? (yes/no): ", phaseNumber)
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

	// Skip execution if no SQL steps
	if phase.Plan == nil || len(phase.Plan.Steps) == 0 {
		fmt.Println("No SQL to execute. Mark this phase complete after code deployment.")

		if err := st.CompletePhase(phaseNumber); err != nil {
			log.Fatalf("Failed to update state: %v", err)
		}

		fmt.Printf("✅ Phase %d marked as complete\n", phaseNumber)
		showNextSteps(st, multiPhasePlan, phaseNumber)
		return
	}

	// Resolve target database
	targetConnStr, err := resolveConnection(cfg, apTarget, apTargetEnv, "target")
	if err != nil {
		log.Fatalf("Failed to resolve target database: %v", err)
	}

	// Resolve shadow database (if not skipped)
	var shadowConnStr string
	if !apSkipShadowDB {
		shadowConnStr, err = resolveConnection(cfg, apShadowDB, apShadowDBEnv, "shadow_db")
		if err != nil {
			log.Fatalf("Failed to resolve shadow database: %v", err)
		}
	}

	// Detect database driver
	driverName := executor.DetectDriver(targetConnStr)
	driver, err := executor.NewDriver(driverName)
	if err != nil {
		log.Fatalf("Failed to create driver: %v", err)
	}

	// Open target database connection
	ctx := context.Background()
	sqlDriverName := executor.GetSQLDriverName(driverName)
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
	currentSchema, err := executor.LoadSchemaFromConnectionString(targetConnStr)
	if err != nil {
		log.Fatalf("Failed to introspect current schema: %v", err)
	}

	// Open shadow database connection (if not skipped)
	var shadowDB *sql.DB
	if !apSkipShadowDB && shadowConnStr != "" {
		shadowDB, err = sql.Open(sqlDriverName, shadowConnStr)
		if err != nil {
			log.Fatalf("Failed to connect to shadow database: %v", err)
		}
		defer func() { _ = shadowDB.Close() }()

		if err := shadowDB.PingContext(ctx); err != nil {
			log.Fatalf("Failed to ping shadow database: %v", err)
		}
	}

	// Execute the phase plan
	fmt.Printf("Executing phase %d...\n", phaseNumber)
	result, err := executor.ApplyPlan(ctx, targetDB, phase.Plan, shadowDB, currentSchema, driver, apVerbose)
	if err != nil {
		log.Fatalf("Failed to execute phase: %v", err)
	}

	if !result.Success {
		fmt.Printf("❌ Phase %d execution failed:\n", phaseNumber)
		for _, errMsg := range result.Errors {
			fmt.Printf("  - %s\n", errMsg)
		}
		log.Fatal("Phase execution failed")
	}

	// Update state
	if err := st.CompletePhase(phaseNumber); err != nil {
		log.Fatalf("Failed to update state: %v", err)
	}

	// Success
	fmt.Printf("\n✅ Phase %d complete!\n", phaseNumber)
	fmt.Printf("Executed %d steps successfully\n", result.StepsApplied)

	showNextSteps(st, multiPhasePlan, phaseNumber)
}

func loadMultiPhasePlan(path string) (*planner.MultiPhasePlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan planner.MultiPhasePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// Validate it's a multi-phase plan
	if !plan.MultiPhase {
		return nil, fmt.Errorf("not a multi-phase plan (use 'lockplane apply' for single-phase plans)")
	}

	return &plan, nil
}

func resolveConnection(cfg *config.Config, explicit string, envName string, defaultEnvKey string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	if envName != "" {
		env, err := config.ResolveEnvironment(cfg, envName)
		if err != nil {
			return "", err
		}
		return env.DatabaseURL, nil
	}

	// Try default environment
	if cfg != nil && len(cfg.Environments) > 0 {
		if defaultEnv, ok := cfg.Environments[defaultEnvKey]; ok {
			return defaultEnv.DatabaseURL, nil
		}
		// Use first environment as fallback
		for _, env := range cfg.Environments {
			return env.DatabaseURL, nil
		}
	}

	return "", fmt.Errorf("no database connection specified (use --target, --target-environment, or configure in lockplane.toml)")
}

func showNextSteps(st *state.State, plan *planner.MultiPhasePlan, completedPhase int) {
	fmt.Printf("\n")

	if completedPhase >= plan.TotalPhases {
		fmt.Println("🎉 All phases complete! Multi-phase migration finished.")
		fmt.Println("\nTo clean up state:")
		fmt.Printf("  rm %s\n", state.StateFile)
		return
	}

	nextPhaseNum := completedPhase + 1
	if nextPhaseNum <= len(plan.Phases) {
		nextPhase := plan.Phases[nextPhaseNum-1]

		fmt.Printf("Next Steps:\n")

		if nextPhase.RequiresCodeDeploy {
			fmt.Printf("  1. Deploy code changes:\n")
			for _, change := range nextPhase.CodeChangesRequired {
				fmt.Printf("     • %s\n", change)
			}
		}

		if len(nextPhase.Verification) > 0 {
			fmt.Printf("  %d. Verify phase %d:\n", 2, completedPhase)
			for _, v := range nextPhase.Verification {
				fmt.Printf("     • %s\n", v)
			}
		}

		fmt.Printf("  %d. Execute phase %d:\n", 3, nextPhaseNum)
		fmt.Printf("     lockplane apply-phase %s --phase %d\n", st.ActiveMigration.PlanPath, nextPhaseNum)
		fmt.Printf("     or: lockplane apply-phase %s --next\n", st.ActiveMigration.PlanPath)
	}
}
