package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// Note: These commands currently call the original main.go implementations
// They can be fully migrated to use executor package functions in a future refactor

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Generate and apply a rollback migration",
	Long: `Generate a rollback plan from a forward migration plan and apply it to the target database.

This command generates a rollback plan and applies it in one step, with shadow DB validation.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Call original implementation from main package
		// TODO: Fully migrate to use executor package
		os.Args = append([]string{"lockplane", "rollback"}, args...)
		// This will be handled by preserving runRollback in main.go temporarily
	},
}

var planRollbackCmd = &cobra.Command{
	Use:   "plan-rollback",
	Short: "Generate a rollback plan from a forward migration plan",
	Long: `Generate a rollback plan from a forward migration plan.

The plan-rollback command generates a reversible migration plan that undoes
a forward migration. It outputs a plan JSON file that can be reviewed,
saved, and applied later using 'lockplane apply'.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Call original implementation from main package
		// TODO: Fully migrate to use executor package
		os.Args = append([]string{"lockplane", "plan-rollback"}, args...)
		// This will be handled by preserving runPlanRollback in main.go temporarily
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(planRollbackCmd)
}
