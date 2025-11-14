package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/state"
	"github.com/spf13/cobra"
)

var rollbackPhaseCmd = &cobra.Command{
	Use:   "rollback-phase <plan-file>",
	Short: "Rollback a specific phase of a multi-phase migration",
	Long: `Rollback a specific phase of a multi-phase migration.

WARNING: Phase rollbacks may require code changes to be reverted.
Always check the rollback instructions in the plan before proceeding.

Some phases cannot be safely rolled back if data has been written
in the new schema shape.`,
	Example: `  # Rollback current phase
  lockplane rollback-phase rename-email.json

  # Rollback specific phase
  lockplane rollback-phase rename-email.json --phase 2

  # Show rollback plan without executing
  lockplane rollback-phase rename-email.json --dry-run`,
	Args: cobra.ExactArgs(1),
	Run:  runRollbackPhase,
}

var (
	rbPhase       int
	rbForce       bool
	rbTargetEnv   string
	rbTarget      string
	rbDryRun      bool
	rbVerbose     bool
	rbAutoApprove bool
)

func init() {
	rootCmd.AddCommand(rollbackPhaseCmd)

	rollbackPhaseCmd.Flags().IntVar(&rbPhase, "phase", 0, "Phase number to rollback (defaults to current phase)")
	rollbackPhaseCmd.Flags().BoolVar(&rbForce, "force", false, "Force rollback, skip safety checks (dangerous)")
	rollbackPhaseCmd.Flags().StringVar(&rbTargetEnv, "target-environment", "", "Target database environment")
	rollbackPhaseCmd.Flags().StringVar(&rbTarget, "target", "", "Target database connection string")
	rollbackPhaseCmd.Flags().BoolVar(&rbDryRun, "dry-run", false, "Show rollback plan without executing")
	rollbackPhaseCmd.Flags().BoolVarP(&rbVerbose, "verbose", "v", false, "Enable verbose logging")
	rollbackPhaseCmd.Flags().BoolVar(&rbAutoApprove, "auto-approve", false, "Automatically approve rollback without prompting")
}

func runRollbackPhase(cmd *cobra.Command, args []string) {
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

	// Determine which phase to rollback
	phaseNumber := rbPhase
	if phaseNumber == 0 {
		if st.ActiveMigration == nil {
			log.Fatal("No active migration to rollback")
		}
		phaseNumber = st.ActiveMigration.CurrentPhase
		if phaseNumber == 0 {
			log.Fatal("No phases have been executed yet")
		}
		fmt.Printf("Rolling back current phase: %d\n", phaseNumber)
	}

	if phaseNumber < 1 || phaseNumber > multiPhasePlan.TotalPhases {
		log.Fatalf("Invalid phase number %d (plan has %d phases)", phaseNumber, multiPhasePlan.TotalPhases)
	}

	// Get the phase
	phase := multiPhasePlan.Phases[phaseNumber-1]
	if phase.Rollback == nil {
		log.Fatalf("Phase %d does not have rollback instructions", phaseNumber)
	}

	// Display rollback information
	fmt.Printf("\n")
	fmt.Printf("⚠️  Rolling Back Phase %d/%d: %s\n", phaseNumber, multiPhasePlan.TotalPhases, phase.Name)
	fmt.Printf("Description: %s\n", phase.Description)
	fmt.Printf("\n")
	fmt.Printf("Rollback: %s\n", phase.Rollback.Description)
	if phase.Rollback.Note != "" {
		fmt.Printf("Note: %s\n", phase.Rollback.Note)
	}
	if phase.Rollback.Warning != "" {
		fmt.Printf("⚠️  WARNING: %s\n", phase.Rollback.Warning)
	}
	fmt.Printf("\n")

	// Show code changes required for rollback
	if phase.Rollback.RequiresCode {
		fmt.Printf("⚠️  Code Rollback Required:\n")
		fmt.Printf("  This rollback requires reverting code changes from this phase.\n")
		fmt.Printf("  Redeploy the previous version of your application before proceeding.\n")
		fmt.Printf("\n")
	}

	// Show SQL rollback steps
	if len(phase.Rollback.SQL) > 0 {
		fmt.Printf("Rollback SQL:\n")
		for i, sql := range phase.Rollback.SQL {
			fmt.Printf("  %d. %s\n", i+1, sql)
		}
		fmt.Printf("\n")
	} else {
		fmt.Printf("No SQL rollback (code-only rollback)\n\n")
	}

	// Dry run mode
	if rbDryRun {
		fmt.Println("🔍 DRY RUN: No changes will be applied")
		return
	}

	// Require approval unless auto-approved
	if !rbAutoApprove {
		fmt.Printf("⚠️  Proceed with phase %d rollback? This may cause data loss. (yes/no): ", phaseNumber)
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

	// If no SQL to execute, just provide guidance
	if len(phase.Rollback.SQL) == 0 {
		fmt.Println("No SQL to execute for rollback.")
		if phase.Rollback.RequiresCode {
			fmt.Println("Ensure you have reverted the code changes before marking this complete.")
		}

		// Update state to mark phase as not complete
		if st.ActiveMigration != nil {
			// Remove phase from completed list
			newCompleted := []int{}
			for _, p := range st.ActiveMigration.PhasesCompleted {
				if p != phaseNumber {
					newCompleted = append(newCompleted, p)
				}
			}
			st.ActiveMigration.PhasesCompleted = newCompleted
			st.ActiveMigration.CurrentPhase = phaseNumber - 1
			if err := st.Save(); err != nil {
				log.Fatalf("Failed to update state: %v", err)
			}
		}

		fmt.Printf("✅ Phase %d rollback complete\n", phaseNumber)
		return
	}

	// Resolve target database
	targetConnStr, err := resolveConnection(cfg, rbTarget, rbTargetEnv, "target")
	if err != nil {
		log.Fatalf("Failed to resolve target database: %v", err)
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

	// Create a rollback plan
	rollbackPlan := &planner.Plan{
		Steps: []planner.PlanStep{
			{
				Description: fmt.Sprintf("Rollback phase %d: %s", phaseNumber, phase.Rollback.Description),
				SQL:         phase.Rollback.SQL,
			},
		},
	}

	// Execute the rollback plan
	fmt.Printf("Executing rollback...\n")
	result, err := executor.ApplyPlan(ctx, targetDB, rollbackPlan, nil, currentSchema, driver, rbVerbose)
	if err != nil {
		log.Fatalf("Failed to execute rollback: %v", err)
	}

	if !result.Success {
		fmt.Printf("❌ Rollback failed:\n")
		for _, errMsg := range result.Errors {
			fmt.Printf("  - %s\n", errMsg)
		}
		log.Fatal("Rollback failed")
	}

	// Update state
	if st.ActiveMigration != nil {
		// Remove phase from completed list
		newCompleted := []int{}
		for _, p := range st.ActiveMigration.PhasesCompleted {
			if p != phaseNumber {
				newCompleted = append(newCompleted, p)
			}
		}
		st.ActiveMigration.PhasesCompleted = newCompleted
		st.ActiveMigration.CurrentPhase = phaseNumber - 1
		if err := st.Save(); err != nil {
			log.Fatalf("Failed to update state: %v", err)
		}
	}

	// Success
	fmt.Printf("\n✅ Phase %d rollback complete\n", phaseNumber)

	if phase.Rollback.RequiresCode {
		fmt.Println("\n⚠️  Don't forget to redeploy the previous version of your application!")
	}
}
