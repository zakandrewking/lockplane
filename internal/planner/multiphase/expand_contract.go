package multiphase

import (
	"fmt"
	"time"

	"github.com/lockplane/lockplane/internal/planner"
)

// GenerateExpandContractPlan creates a multi-phase plan for column rename or compatible type change
// This uses the expand/contract pattern:
// - Phase 1 (Expand): Add new column, enable dual-write
// - Phase 2 (Migrate): Switch application to read from new column
// - Phase 3 (Contract): Remove old column
func GenerateExpandContractPlan(
	table string,
	oldColumn string,
	newColumn string,
	columnType string,
	sourceHash string,
) (*planner.MultiPhasePlan, error) {
	if table == "" || oldColumn == "" || newColumn == "" {
		return nil, fmt.Errorf("table, oldColumn, and newColumn are required")
	}

	if oldColumn == newColumn {
		return nil, fmt.Errorf("oldColumn and newColumn must be different")
	}

	// Generate unique migration ID
	migrationID := fmt.Sprintf("rename_%s_%s_%d", table, oldColumn, time.Now().Unix())

	// Phase 1: Expand - Add new column and backfill
	phase1 := planner.Phase{
		PhaseNumber:        1,
		Name:               "expand",
		Description:        fmt.Sprintf("Add %s column and backfill from %s", newColumn, oldColumn),
		RequiresCodeDeploy: true,
		DependsOnPhase:     0, // No dependency
		CodeChangesRequired: []string{
			fmt.Sprintf("Update application to write to both %s and %s columns", oldColumn, newColumn),
			fmt.Sprintf("Keep reading from %s column", oldColumn),
			"Deploy this code before proceeding to Phase 2",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: fmt.Sprintf("Add %s column (nullable)", newColumn),
					SQL: []string{
						fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, newColumn, columnType),
					},
				},
				{
					Description: fmt.Sprintf("Backfill %s from %s", newColumn, oldColumn),
					SQL: []string{
						fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s IS NULL", table, newColumn, oldColumn, newColumn),
					},
				},
			},
		},
		Verification: []string{
			fmt.Sprintf("Verify dual-write is working: SELECT COUNT(*) FROM %s WHERE %s IS NULL AND %s IS NOT NULL", table, newColumn, oldColumn),
			fmt.Sprintf("Monitor application logs for %s writes", newColumn),
			fmt.Sprintf("Check that all new rows have both %s and %s populated", oldColumn, newColumn),
		},
		Rollback: &planner.PhaseRollback{
			Description: fmt.Sprintf("Drop %s column", newColumn),
			SQL: []string{
				fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, newColumn),
			},
			Note:         "Safe to rollback - old column still in use",
			Warning:      "Any data written to new column will be lost",
			RequiresCode: true, // Need to stop dual-write in code
		},
		EstimatedDuration: "< 1 minute (depends on table size)",
		LockImpact:        "Brief AccessExclusive lock during ALTER TABLE",
	}

	// Phase 2: Migrate Reads - Switch to reading from new column (code-only)
	phase2 := planner.Phase{
		PhaseNumber:        2,
		Name:               "migrate_reads",
		Description:        fmt.Sprintf("Switch application to read from %s column", newColumn),
		RequiresCodeDeploy: true,
		DependsOnPhase:     1,
		CodeChangesRequired: []string{
			fmt.Sprintf("Update application to read from %s column", newColumn),
			fmt.Sprintf("Continue writing to both %s and %s columns", oldColumn, newColumn),
			"Deploy this code before proceeding to Phase 3",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps:      []planner.PlanStep{}, // No SQL changes in this phase
		},
		Verification: []string{
			fmt.Sprintf("Monitor application logs to confirm reading from %s", newColumn),
			fmt.Sprintf("Verify no queries are reading from %s column", oldColumn),
			"Check application metrics for any errors",
		},
		Rollback: &planner.PhaseRollback{
			Description:  fmt.Sprintf("Switch reads back to %s column", oldColumn),
			SQL:          []string{}, // Code deployment only
			Note:         "Code deployment only - no SQL changes",
			Warning:      "Must redeploy application to read from old column",
			RequiresCode: true,
		},
		EstimatedDuration: "Instant (code deployment only)",
		LockImpact:        "None",
	}

	// Phase 3: Contract - Remove old column
	phase3 := planner.Phase{
		PhaseNumber:        3,
		Name:               "contract",
		Description:        fmt.Sprintf("Remove old %s column", oldColumn),
		RequiresCodeDeploy: true,
		DependsOnPhase:     2,
		CodeChangesRequired: []string{
			fmt.Sprintf("Remove all references to %s column from application code", oldColumn),
			fmt.Sprintf("Use only %s column", newColumn),
			"Deploy this code before executing Phase 3",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: fmt.Sprintf("Drop old %s column", oldColumn),
					SQL: []string{
						fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, oldColumn),
					},
				},
			},
		},
		Verification: []string{
			"Verify application is working correctly with new column",
			fmt.Sprintf("Check that no errors related to %s column appear in logs", oldColumn),
			"Monitor application metrics",
		},
		Rollback: &planner.PhaseRollback{
			Description: fmt.Sprintf("Re-add %s column and backfill from %s", oldColumn, newColumn),
			SQL: []string{
				fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, oldColumn, columnType),
				fmt.Sprintf("UPDATE %s SET %s = %s", table, oldColumn, newColumn),
			},
			Warning:      "Rollback requires redeploying code to dual-write again",
			RequiresCode: true,
		},
		EstimatedDuration: "< 1 minute",
		LockImpact:        "Brief AccessExclusive lock during DROP COLUMN",
	}

	return &planner.MultiPhasePlan{
		MultiPhase:  true,
		Operation:   "rename_column",
		Description: fmt.Sprintf("Rename %s.%s to %s.%s using expand/contract pattern", table, oldColumn, table, newColumn),
		Pattern:     "expand_contract",
		TotalPhases: 3,
		Phases:      []planner.Phase{phase1, phase2, phase3},
		SafetyNotes: []string{
			"Each phase is backward compatible with the previous phase",
			"Code must be deployed between phases",
			"Rollback is possible at any phase but may require code changes",
			"Monitor application behavior between phases",
			fmt.Sprintf("Phase 1: Both columns exist, app writes to both, reads from %s", oldColumn),
			fmt.Sprintf("Phase 2: Both columns exist, app writes to both, reads from %s", newColumn),
			fmt.Sprintf("Phase 3: Only %s column exists", newColumn),
		},
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}
