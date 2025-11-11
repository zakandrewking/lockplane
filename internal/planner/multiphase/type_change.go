package multiphase

import (
	"fmt"
	"time"

	"github.com/lockplane/lockplane/internal/planner"
)

// GenerateTypeChangePlan creates a multi-phase plan for incompatible column type changes
// This uses the dual-write pattern:
// - Phase 1: Add new column with new type
// - Phase 2: Enable dual-write (write to both columns)
// - Phase 3: Backfill new column from old column
// - Phase 4: Migrate reads to new column
// - Phase 5: Drop old column
func GenerateTypeChangePlan(
	table string,
	column string,
	oldType string,
	newType string,
	conversionExpr string, // SQL expression to convert old type to new type
	sourceHash string,
) (*planner.MultiPhasePlan, error) {
	if table == "" || column == "" {
		return nil, fmt.Errorf("table and column are required")
	}

	if oldType == "" || newType == "" {
		return nil, fmt.Errorf("oldType and newType are required")
	}

	if conversionExpr == "" {
		// Default: try casting
		conversionExpr = fmt.Sprintf("CAST(%s AS %s)", column, newType)
	}

	migrationID := fmt.Sprintf("alter_type_%s_%s_%d", table, column, time.Now().Unix())
	newColumnName := column + "_new"

	// Phase 1: Add new column
	phase1 := planner.Phase{
		PhaseNumber:        1,
		Name:               "add_new_column",
		Description:        fmt.Sprintf("Add %s column with %s type", newColumnName, newType),
		RequiresCodeDeploy: false,
		DependsOnPhase:     0,
		CodeChangesRequired: []string{
			"No code changes yet - new column is not used",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: fmt.Sprintf("Add %s column (type: %s)", newColumnName, newType),
					SQL: []string{
						fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, newColumnName, newType),
					},
				},
			},
		},
		Verification: []string{
			fmt.Sprintf("Verify column exists: SELECT %s FROM %s LIMIT 1", newColumnName, table),
			"Check column type is correct",
		},
		Rollback: &planner.PhaseRollback{
			Description: fmt.Sprintf("Drop %s column", newColumnName),
			SQL: []string{
				fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, newColumnName),
			},
			Note: "Safe to rollback - old column still in use",
		},
		EstimatedDuration: "< 1 second",
		LockImpact:        "AccessExclusive lock (brief)",
	}

	// Phase 2: Enable dual-write
	phase2 := planner.Phase{
		PhaseNumber:        2,
		Name:               "enable_dual_write",
		Description:        fmt.Sprintf("Update application to write to both %s and %s", column, newColumnName),
		RequiresCodeDeploy: true,
		DependsOnPhase:     1,
		CodeChangesRequired: []string{
			fmt.Sprintf("Update INSERT statements to include %s", newColumnName),
			fmt.Sprintf("Update UPDATE statements to modify both %s and %s", column, newColumnName),
			fmt.Sprintf("Apply conversion logic: %s", conversionExpr),
			fmt.Sprintf("Continue reading from %s", column),
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps:      []planner.PlanStep{}, // Code only
		},
		Verification: []string{
			"Monitor application logs for dual-write activity",
			fmt.Sprintf("Verify new rows have both %s and %s populated", column, newColumnName),
			"Check for any errors in conversion logic",
		},
		Rollback: &planner.PhaseRollback{
			Description:  fmt.Sprintf("Stop writing to %s", newColumnName),
			SQL:          []string{},
			Note:         "Code deployment only - remove dual-write logic",
			RequiresCode: true,
		},
		EstimatedDuration: "Instant (code deployment only)",
		LockImpact:        "None",
	}

	// Phase 3: Backfill new column
	phase3 := planner.Phase{
		PhaseNumber:        3,
		Name:               "backfill",
		Description:        fmt.Sprintf("Backfill %s from %s using conversion expression", newColumnName, column),
		RequiresCodeDeploy: false,
		DependsOnPhase:     2,
		CodeChangesRequired: []string{
			"No code changes required for this phase",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: fmt.Sprintf("Backfill %s from %s", newColumnName, column),
					SQL: []string{
						fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s IS NULL AND %s IS NOT NULL",
							table, newColumnName, conversionExpr, newColumnName, column),
					},
				},
			},
		},
		Verification: []string{
			fmt.Sprintf("Verify all rows have %s populated: SELECT COUNT(*) FROM %s WHERE %s IS NULL AND %s IS NOT NULL",
				newColumnName, table, newColumnName, column),
			"Check data integrity - compare sample rows",
			"Verify conversion logic worked correctly",
		},
		Rollback: &planner.PhaseRollback{
			Description: fmt.Sprintf("Clear %s column", newColumnName),
			SQL: []string{
				fmt.Sprintf("UPDATE %s SET %s = NULL", table, newColumnName),
			},
			Note:    "Clears backfilled data but keeps column",
			Warning: "Recent data from dual-write will also be cleared",
		},
		EstimatedDuration: "Depends on table size",
		LockImpact:        "Row-level locks during UPDATE",
	}

	// Phase 4: Migrate reads
	phase4 := planner.Phase{
		PhaseNumber:        4,
		Name:               "migrate_reads",
		Description:        fmt.Sprintf("Update application to read from %s", newColumnName),
		RequiresCodeDeploy: true,
		DependsOnPhase:     3,
		CodeChangesRequired: []string{
			fmt.Sprintf("Update all SELECT statements to read from %s", newColumnName),
			fmt.Sprintf("Continue writing to both %s and %s", column, newColumnName),
			"Update any business logic that depends on the column type",
			"Deploy and monitor carefully",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps:      []planner.PlanStep{}, // Code only
		},
		Verification: []string{
			fmt.Sprintf("Monitor application logs - confirm reading from %s", newColumnName),
			fmt.Sprintf("Verify no queries reference %s column", column),
			"Check application metrics for any type-related errors",
			"Validate business logic still works correctly",
		},
		Rollback: &planner.PhaseRollback{
			Description:  fmt.Sprintf("Switch reads back to %s", column),
			SQL:          []string{},
			Note:         "Code deployment only",
			Warning:      "Must redeploy to read from old column again",
			RequiresCode: true,
		},
		EstimatedDuration: "Instant (code deployment only)",
		LockImpact:        "None",
	}

	// Phase 5: Drop old column
	phase5 := planner.Phase{
		PhaseNumber:        5,
		Name:               "drop_old_column",
		Description:        fmt.Sprintf("Drop old %s column and rename %s to %s", column, newColumnName, column),
		RequiresCodeDeploy: true,
		DependsOnPhase:     4,
		CodeChangesRequired: []string{
			fmt.Sprintf("Update all references from %s to %s in application code", newColumnName, column),
			fmt.Sprintf("Remove dual-write logic for %s", column),
			"Use only the renamed column",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: fmt.Sprintf("Drop old %s column", column),
					SQL: []string{
						fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column),
					},
				},
				{
					Description: fmt.Sprintf("Rename %s to %s", newColumnName, column),
					SQL: []string{
						fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", table, newColumnName, column),
					},
				},
			},
		},
		Verification: []string{
			fmt.Sprintf("Verify %s column exists with correct type", column),
			fmt.Sprintf("Verify old column is gone: SELECT * FROM information_schema.columns WHERE table_name = '%s'", table),
			"Confirm application works with final schema",
		},
		Rollback: &planner.PhaseRollback{
			Description: "Cannot easily rollback - requires re-creating old column with old type",
			SQL:         []string{},
			Warning: fmt.Sprintf("Complex rollback: would need to add %s column back with type %s, backfill from %s, update code",
				column, oldType, newColumnName),
			RequiresCode: true,
		},
		EstimatedDuration: "< 1 second",
		LockImpact:        "AccessExclusive lock during DROP and RENAME",
	}

	return &planner.MultiPhasePlan{
		MultiPhase:  true,
		Operation:   "alter_column_type",
		Description: fmt.Sprintf("Change %s.%s type from %s to %s using dual-write pattern", table, column, oldType, newType),
		Pattern:     "type_change",
		TotalPhases: 5,
		Phases: []planner.Phase{
			phase1,
			phase2,
			phase3,
			phase4,
			phase5,
		},
		SafetyNotes: []string{
			"Type change requires coordination between database and application",
			"Each phase is backward compatible with previous phase",
			fmt.Sprintf("Phase 1: Add %s column (%s type)", newColumnName, newType),
			"Phase 2: Application writes to both columns (dual-write)",
			fmt.Sprintf("Phase 3: Backfill %s from %s", newColumnName, column),
			fmt.Sprintf("Phase 4: Application reads from %s", newColumnName),
			fmt.Sprintf("Phase 5: Drop %s, rename %s to %s", column, newColumnName, column),
			"⚠️  Monitor application behavior carefully during Phase 4 (read migration)",
			fmt.Sprintf("Conversion expression: %s", conversionExpr),
		},
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}
