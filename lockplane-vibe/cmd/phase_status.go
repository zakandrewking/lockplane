package cmd

import (
	"fmt"
	"log"

	"github.com/lockplane/lockplane/internal/state"
	"github.com/spf13/cobra"
)

var phaseStatusCmd = &cobra.Command{
	Use:   "phase-status",
	Short: "Show status of active multi-phase migration",
	Long: `Display the current status of any active multi-phase migration.

Shows which phases have been completed, which phase is current, and
what needs to be done next.`,
	Example: `  # Show current migration status
  lockplane phase-status

  # Show detailed status
  lockplane phase-status --verbose`,
	Run: runPhaseStatus,
}

var psVerbose bool

func init() {
	rootCmd.AddCommand(phaseStatusCmd)

	phaseStatusCmd.Flags().BoolVarP(&psVerbose, "verbose", "v", false, "Show detailed status information")
}

func runPhaseStatus(cmd *cobra.Command, args []string) {
	// Load state
	st, err := state.Load()
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	if st.ActiveMigration == nil {
		fmt.Println("No active multi-phase migration")
		fmt.Println("\nTo start a new multi-phase migration:")
		fmt.Println("  1. Generate plan: lockplane plan-multiphase --pattern <pattern> ...")
		fmt.Println("  2. Execute phase 1: lockplane apply-phase <plan-file> --phase 1")
		return
	}

	// Display active migration status
	m := st.ActiveMigration

	fmt.Printf("📋 Active Multi-Phase Migration\n\n")
	fmt.Printf("ID:          %s\n", m.ID)
	fmt.Printf("Operation:   %s\n", m.Operation)
	fmt.Printf("Pattern:     %s\n", m.Pattern)
	fmt.Printf("Table:       %s\n", m.Table)
	if m.Column != "" {
		fmt.Printf("Column:      %s\n", m.Column)
	}
	fmt.Printf("Started:     %s\n", m.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Last Update: %s\n", m.LastUpdated.Format("2006-01-02 15:04:05"))
	fmt.Printf("\n")

	// Progress
	fmt.Printf("Progress: %d/%d phases complete\n", len(m.PhasesCompleted), m.TotalPhases)
	fmt.Printf("\n")

	// Show phase status
	fmt.Println("Phase Status:")
	for i := 1; i <= m.TotalPhases; i++ {
		status := getPhaseStatus(m, i)
		icon := getPhaseIcon(m, i)
		fmt.Printf("  %s Phase %d: %s\n", icon, i, status)
	}
	fmt.Printf("\n")

	// Next steps
	if m.CurrentPhase >= m.TotalPhases {
		fmt.Println("✅ All phases complete!")
		fmt.Println("\nTo clean up:")
		fmt.Printf("  rm %s\n", state.StateFile)
	} else {
		nextPhase := m.CurrentPhase + 1
		fmt.Printf("Next: Execute phase %d\n", nextPhase)
		fmt.Printf("  lockplane apply-phase %s --phase %d\n", m.PlanPath, nextPhase)
		fmt.Printf("  or: lockplane apply-phase %s --next\n", m.PlanPath)
	}

	if psVerbose {
		fmt.Printf("\nState file: %s\n", state.StateFile)
		fmt.Printf("Plan file:  %s\n", m.PlanPath)
	}
}

func getPhaseStatus(m *state.ActiveMigration, phaseNum int) string {
	// Check if completed
	for _, completed := range m.PhasesCompleted {
		if completed == phaseNum {
			return "Complete"
		}
	}

	// Check if current
	if phaseNum == m.CurrentPhase+1 {
		return "Ready to execute"
	}

	if phaseNum <= m.CurrentPhase {
		return "Complete"
	}

	return "Pending"
}

func getPhaseIcon(m *state.ActiveMigration, phaseNum int) string {
	// Check if completed
	for _, completed := range m.PhasesCompleted {
		if completed == phaseNum {
			return "✅"
		}
	}

	// Check if next
	if phaseNum == m.CurrentPhase+1 {
		return "▶️"
	}

	if phaseNum <= m.CurrentPhase {
		return "✅"
	}

	return "⏸️"
}
