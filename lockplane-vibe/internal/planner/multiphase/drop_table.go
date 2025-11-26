package multiphase

import (
	"fmt"
	"time"

	"github.com/lockplane/lockplane/internal/planner"
)

// GenerateDropTablePlan creates a multi-phase plan for safely dropping a table
// This uses the deprecation period pattern:
// - Phase 1: Stop writes to table
// - Phase 2 (Optional): Archive entire table
// - Phase 3: Stop reads from table
// - Phase 4: Drop table
func GenerateDropTablePlan(
	table string,
	archiveData bool,
	sourceHash string,
) (*planner.MultiPhasePlan, error) {
	if table == "" {
		return nil, fmt.Errorf("table name is required")
	}

	phases := []planner.Phase{}

	// Phase 1: Stop Writes
	phase1 := planner.Phase{
		PhaseNumber:        1,
		Name:               "stop_writes",
		Description:        fmt.Sprintf("Stop all writes to %s table", table),
		RequiresCodeDeploy: true,
		DependsOnPhase:     0,
		CodeChangesRequired: []string{
			fmt.Sprintf("Remove all code that writes to %s table", table),
			fmt.Sprintf("Keep code that reads from %s table (for now)", table),
			"Deploy and monitor for any write attempts",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps:      []planner.PlanStep{}, // No SQL changes
		},
		Verification: []string{
			fmt.Sprintf("Monitor database logs for any INSERT/UPDATE/DELETE statements on %s table", table),
			"Verify no application errors related to missing table writes",
			fmt.Sprintf("Check that %s table data is not changing", table),
			"Run: SELECT COUNT(*) FROM pg_stat_user_tables WHERE schemaname = 'public' AND relname = '" + table + "' AND n_tup_ins + n_tup_upd + n_tup_del > 0",
		},
		Rollback: &planner.PhaseRollback{
			Description:  fmt.Sprintf("Re-enable writes to %s table", table),
			SQL:          []string{},
			Note:         "Code deployment only - restore write logic",
			RequiresCode: true,
		},
		EstimatedDuration: "Instant (code deployment only)",
		LockImpact:        "None",
	}
	phases = append(phases, phase1)

	// Phase 2 (Optional): Archive Table
	if archiveData {
		phase2 := planner.Phase{
			PhaseNumber:        2,
			Name:               "archive",
			Description:        fmt.Sprintf("Archive %s table data for audit/recovery", table),
			RequiresCodeDeploy: false,
			DependsOnPhase:     1,
			CodeChangesRequired: []string{
				"No code changes required for this phase",
			},
			Plan: &planner.Plan{
				SourceHash: sourceHash,
				Steps: []planner.PlanStep{
					{
						Description: fmt.Sprintf("Create archive table for %s", table),
						SQL: []string{
							fmt.Sprintf("CREATE TABLE %s_archive AS TABLE %s", table, table),
							fmt.Sprintf("ALTER TABLE %s_archive ADD COLUMN archived_at TIMESTAMP DEFAULT NOW()", table),
						},
					},
					{
						Description: "Create index on archive timestamp for audit queries",
						SQL: []string{
							fmt.Sprintf("CREATE INDEX idx_%s_archive_archived_at ON %s_archive(archived_at)", table, table),
						},
					},
				},
			},
			Verification: []string{
				fmt.Sprintf("Verify archive table %s_archive was created", table),
				fmt.Sprintf("Check row counts match: SELECT COUNT(*) FROM %s; SELECT COUNT(*) FROM %s_archive;", table, table),
				"Verify data integrity in archive",
				"Test sample queries against archive table",
			},
			Rollback: &planner.PhaseRollback{
				Description: fmt.Sprintf("Drop archive table %s_archive", table),
				SQL: []string{
					fmt.Sprintf("DROP TABLE IF EXISTS %s_archive", table),
				},
				Note: "Archive can be safely dropped - original table still exists",
			},
			EstimatedDuration: "Depends on table size (full table copy)",
			LockImpact:        "None (reads only, archive table is new)",
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
		Description:        fmt.Sprintf("Stop all reads from %s table", table),
		RequiresCodeDeploy: true,
		DependsOnPhase:     stopReadsPhaseNum - 1,
		CodeChangesRequired: []string{
			fmt.Sprintf("Remove all code that reads from %s table", table),
			fmt.Sprintf("Verify no queries reference %s table", table),
			"Remove table from ORM models/schemas",
			"Deploy and monitor for any read attempts",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps:      []planner.PlanStep{}, // No SQL changes
		},
		Verification: []string{
			fmt.Sprintf("Monitor application logs for any SELECT statements on %s table", table),
			"Verify no application errors",
			"Check database slow query logs",
			fmt.Sprintf("Run: SELECT query FROM pg_stat_statements WHERE query ILIKE '%%%s%%' LIMIT 10", table),
		},
		Rollback: &planner.PhaseRollback{
			Description:  fmt.Sprintf("Re-enable reads from %s table", table),
			SQL:          []string{},
			Note:         "Code deployment only - restore read logic and ORM models",
			Warning:      "Must redeploy application code with table models restored",
			RequiresCode: true,
		},
		EstimatedDuration: "Instant (code deployment only)",
		LockImpact:        "None",
	}
	phases = append(phases, phase3)

	// Phase 4: Drop Table
	dropPhaseNum := stopReadsPhaseNum + 1

	phase4 := planner.Phase{
		PhaseNumber:        dropPhaseNum,
		Name:               "drop_table",
		Description:        fmt.Sprintf("Drop %s table permanently", table),
		RequiresCodeDeploy: false,
		DependsOnPhase:     dropPhaseNum - 1,
		CodeChangesRequired: []string{
			"No code changes required - table is no longer referenced",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: fmt.Sprintf("Drop table %s", table),
					SQL: []string{
						fmt.Sprintf("DROP TABLE %s", table),
					},
				},
			},
		},
		Verification: []string{
			"Verify application continues to work correctly",
			"Check that no errors appear in logs",
			"Confirm table is removed from schema",
			"Run: SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename = '" + table + "' (should return 0 rows)",
		},
		Rollback: &planner.PhaseRollback{
			Description: "Cannot fully rollback - table and all data are permanently lost",
			SQL:         []string{},
			Warning: fmt.Sprintf("Table %s and all its data are permanently lost. If archived, data exists in %s_archive table, but table structure, constraints, indexes, and foreign keys must be manually recreated.",
				table, table),
			RequiresCode: true,
			Note:         "Restoration requires: 1) Recreating table schema, 2) Restoring data from archive, 3) Redeploying application code",
		},
		EstimatedDuration: "< 1 minute (instant for small tables, longer for large tables with indexes)",
		LockImpact:        "Brief AccessExclusive lock during DROP TABLE",
	}
	phases = append(phases, phase4)

	safetyNotes := []string{
		"This is a deprecation period workflow - allows safe table removal",
		"Each phase is backward compatible with the previous phase",
		"Code must be deployed before SQL changes in each phase",
		fmt.Sprintf("Phase 1: Stop writes to %s table", table),
	}

	if archiveData {
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 2: Archive entire %s table to %s_archive", table, table))
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 3: Stop reads from %s table", table))
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 4: Drop %s table permanently", table))
	} else {
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 2: Stop reads from %s table", table))
		safetyNotes = append(safetyNotes, fmt.Sprintf("Phase 3: Drop %s table permanently", table))
	}

	safetyNotes = append(safetyNotes, "⚠️  Final phase is IRREVERSIBLE - entire table and all data will be lost")
	safetyNotes = append(safetyNotes, "⚠️  All foreign keys referencing this table must be removed first")
	safetyNotes = append(safetyNotes, "Monitor application between each phase to ensure no issues")
	safetyNotes = append(safetyNotes, "Consider keeping archive indefinitely for compliance/audit purposes")

	return &planner.MultiPhasePlan{
		MultiPhase:  true,
		Operation:   "drop_table",
		Description: fmt.Sprintf("Safely drop %s table using deprecation period pattern", table),
		Pattern:     "table_deprecation",
		TotalPhases: len(phases),
		Phases:      phases,
		SafetyNotes: safetyNotes,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}, nil
}
