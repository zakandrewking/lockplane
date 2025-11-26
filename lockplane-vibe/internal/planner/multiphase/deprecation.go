package multiphase

import (
	"fmt"
	"time"

	"github.com/lockplane/lockplane/internal/planner"
)

// GenerateDeprecationPlan creates a multi-phase plan for safely dropping a column
// This uses the deprecation period pattern:
// - Phase 1: Stop writes to column
// - Phase 2 (Optional): Archive data
// - Phase 3: Stop reads from column
// - Phase 4: Drop column
func GenerateDeprecationPlan(
	table string,
	column string,
	columnType string,
	archiveData bool,
	sourceHash string,
) (*planner.MultiPhasePlan, error) {
	if table == "" || column == "" {
		return nil, fmt.Errorf("table and column are required")
	}

	phases := []planner.Phase{}

	// Phase 1: Stop Writes
	phase1 := planner.Phase{
		PhaseNumber:        1,
		Name:               "stop_writes",
		Description:        fmt.Sprintf("Stop all writes to %s.%s column", table, column),
		RequiresCodeDeploy: true,
		DependsOnPhase:     0,
		CodeChangesRequired: []string{
			fmt.Sprintf("Remove all code that writes to %s column", column),
			fmt.Sprintf("Keep code that reads from %s column (for now)", column),
			"Deploy and monitor for any write attempts",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps:      []planner.PlanStep{}, // No SQL changes
		},
		Verification: []string{
			"Monitor database logs for any INSERT/UPDATE statements affecting this column",
			"Verify no application errors related to missing column writes",
			"Check that column values are not changing",
		},
		Rollback: &planner.PhaseRollback{
			Description:  fmt.Sprintf("Re-enable writes to %s column", column),
			SQL:          []string{},
			Note:         "Code deployment only - restore write logic",
			RequiresCode: true,
		},
		EstimatedDuration: "Instant (code deployment only)",
		LockImpact:        "None",
	}
	phases = append(phases, phase1)

	// Phase 2 (Optional): Archive Data
	if archiveData {
		phase2 := planner.Phase{
			PhaseNumber:        2,
			Name:               "archive",
			Description:        fmt.Sprintf("Archive %s.%s data for audit/recovery", table, column),
			RequiresCodeDeploy: false,
			DependsOnPhase:     1,
			CodeChangesRequired: []string{
				"No code changes required for this phase",
			},
			Plan: &planner.Plan{
				SourceHash: sourceHash,
				Steps: []planner.PlanStep{
					{
						Description: fmt.Sprintf("Create archive table for %s.%s", table, column),
						SQL: []string{
							fmt.Sprintf("CREATE TABLE %s_%s_archive AS SELECT id, %s, NOW() as archived_at FROM %s WHERE %s IS NOT NULL",
								table, column, column, table, column),
						},
					},
				},
			},
			Verification: []string{
				fmt.Sprintf("Verify archive table %s_%s_archive was created", table, column),
				"Check row counts match",
				"Verify data integrity in archive",
			},
			Rollback: &planner.PhaseRollback{
				Description: fmt.Sprintf("Drop archive table %s_%s_archive", table, column),
				SQL: []string{
					fmt.Sprintf("DROP TABLE IF EXISTS %s_%s_archive", table, column),
				},
				Note: "Archive can be safely dropped - original column still exists",
			},
			EstimatedDuration: "Depends on table size",
			LockImpact:        "None (reads only)",
		}
		phases = append(phases, phase2)
	}

	// Phase 3: Stop Reads
	stopReadsPhaseNum := 2
	if archiveData {
		stopReadsPhaseNum = 3
	}

	phase3 := planner.Phase{
		PhaseNumber:        stopReadsPhaseNum,
		Name:               "stop_reads",
		Description:        fmt.Sprintf("Stop all reads from %s.%s column", table, column),
		RequiresCodeDeploy: true,
		DependsOnPhase:     stopReadsPhaseNum - 1,
		CodeChangesRequired: []string{
			fmt.Sprintf("Remove all code that reads from %s column", column),
			"Verify no queries reference this column",
			"Deploy and monitor for any read attempts",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps:      []planner.PlanStep{}, // No SQL changes
		},
		Verification: []string{
			"Monitor application logs for any SELECT statements referencing this column",
			"Verify no application errors",
			"Check database slow query logs",
		},
		Rollback: &planner.PhaseRollback{
			Description:  fmt.Sprintf("Re-enable reads from %s column", column),
			SQL:          []string{},
			Note:         "Code deployment only - restore read logic",
			Warning:      "Must redeploy application code",
			RequiresCode: true,
		},
		EstimatedDuration: "Instant (code deployment only)",
		LockImpact:        "None",
	}
	phases = append(phases, phase3)

	// Phase 4: Drop Column
	dropPhaseNum := stopReadsPhaseNum + 1

	phase4 := planner.Phase{
		PhaseNumber:        dropPhaseNum,
		Name:               "drop_column",
		Description:        fmt.Sprintf("Drop %s.%s column", table, column),
		RequiresCodeDeploy: false,
		DependsOnPhase:     dropPhaseNum - 1,
		CodeChangesRequired: []string{
			"No code changes required - column is no longer referenced",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: fmt.Sprintf("Drop column %s.%s", table, column),
					SQL: []string{
						fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column),
					},
				},
			},
		},
		Verification: []string{
			"Verify application continues to work correctly",
			"Check that no errors appear in logs",
			"Confirm column is removed from schema",
		},
		Rollback: &planner.PhaseRollback{
			Description: "Cannot fully rollback - data is permanently lost",
			SQL:         []string{},
			Warning: fmt.Sprintf("Column %s data is permanently lost. If archived, can restore from %s_%s_archive table, but requires manual intervention.",
				column, table, column),
			RequiresCode: true,
		},
		EstimatedDuration: "< 1 minute",
		LockImpact:        "Brief AccessExclusive lock during DROP COLUMN",
	}
	phases = append(phases, phase4)

	safetyNotes := []string{
		"This is a deprecation period workflow - allows safe column removal",
		"Each phase is backward compatible",
		"Code must be deployed before SQL changes in each phase",
		fmt.Sprintf("Phase 1: Stop writes to %s", column),
	}

	if archiveData {
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 2: Archive %s data to %s_%s_archive", column, table, column))
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 3: Stop reads from %s", column))
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 4: Drop %s column permanently", column))
	} else {
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 2: Stop reads from %s", column))
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 3: Drop %s column permanently", column))
	}

	safetyNotes = append(safetyNotes, "⚠️  Final phase is irreversible - column data will be lost")
	safetyNotes = append(safetyNotes, "Monitor application between each phase to ensure no issues")

	return &planner.MultiPhasePlan{
		MultiPhase:  true,
		Operation:   "drop_column",
		Description: fmt.Sprintf("Safely drop %s.%s using deprecation period pattern", table, column),
		Pattern:     "deprecation",
		TotalPhases: len(phases),
		Phases:      phases,
		SafetyNotes: safetyNotes,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}, nil
}
