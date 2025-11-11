package multiphase

import (
	"fmt"
	"time"

	"github.com/lockplane/lockplane/internal/planner"
)

// GenerateValidationPhasePlan creates a multi-phase plan for adding constraints safely
// This uses the validation phase pattern for ADD NOT NULL, ADD CHECK, ADD UNIQUE:
// - Phase 1: Backfill existing data
// - Phase 2: Add constraint (NOT VALID) - validates new data only
// - Phase 3: Validate existing data
// - Phase 4: Make constraint enforced
func GenerateValidationPhasePlan(
	table string,
	column string,
	columnType string,
	constraintType string, // "not_null", "check", "unique"
	backfillValue string, // Value to use for backfill, or SQL expression
	checkExpression string, // For CHECK constraints
	sourceHash string,
) (*planner.MultiPhasePlan, error) {
	if table == "" || column == "" {
		return nil, fmt.Errorf("table and column are required")
	}

	if constraintType != "not_null" && constraintType != "check" && constraintType != "unique" {
		return nil, fmt.Errorf("constraintType must be 'not_null', 'check', or 'unique'")
	}

	if constraintType == "not_null" && backfillValue == "" {
		return nil, fmt.Errorf("backfillValue is required for NOT NULL constraints")
	}

	if constraintType == "check" && checkExpression == "" {
		return nil, fmt.Errorf("checkExpression is required for CHECK constraints")
	}

	phases := []planner.Phase{}

	// Phase 1: Backfill
	var backfillSQL string
	var backfillDesc string

	switch constraintType {
	case "not_null":
		backfillSQL = fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s IS NULL", table, column, backfillValue, column)
		backfillDesc = fmt.Sprintf("Backfill NULL values in %s.%s with %s", table, column, backfillValue)
	case "check":
		// For CHECK constraints, might need to fix invalid data
		backfillSQL = fmt.Sprintf("-- Review and fix data that violates: %s", checkExpression)
		backfillDesc = fmt.Sprintf("Fix data in %s.%s to satisfy CHECK constraint", table, column)
	case "unique":
		backfillSQL = fmt.Sprintf("-- Review and deduplicate values in %s.%s", table, column)
		backfillDesc = fmt.Sprintf("Deduplicate values in %s.%s for UNIQUE constraint", table, column)
	}

	phase1 := planner.Phase{
		PhaseNumber:        1,
		Name:               "backfill",
		Description:        backfillDesc,
		RequiresCodeDeploy: false,
		DependsOnPhase:     0,
		CodeChangesRequired: []string{
			"No code changes required for this phase",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: backfillDesc,
					SQL:         []string{backfillSQL},
				},
			},
		},
		Verification: []string{
			fmt.Sprintf("Verify no NULL values remain: SELECT COUNT(*) FROM %s WHERE %s IS NULL", table, column),
			"Check affected row count",
		},
		Rollback: &planner.PhaseRollback{
			Description: "Backfill cannot be rolled back",
			SQL:         []string{},
			Note:        "Data modifications are permanent, but constraint is not yet added",
			Warning:     "If backfill was incorrect, must manually fix data",
		},
		EstimatedDuration: "Depends on table size and number of NULL values",
		LockImpact:        "Row-level locks during UPDATE",
	}
	phases = append(phases, phase1)

	// Phase 2: Add Constraint (NOT VALID)
	var addConstraintSQL string
	var addConstraintDesc string
	constraintName := fmt.Sprintf("%s_%s_%s", table, column, constraintType)

	switch constraintType {
	case "not_null":
		// PostgreSQL: Use CHECK constraint with NOT VALID
		addConstraintSQL = fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s IS NOT NULL) NOT VALID",
			table, constraintName, column)
		addConstraintDesc = fmt.Sprintf("Add NOT NULL check constraint (NOT VALID) on %s.%s", table, column)
	case "check":
		addConstraintSQL = fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s) NOT VALID",
			table, constraintName, checkExpression)
		addConstraintDesc = fmt.Sprintf("Add CHECK constraint (NOT VALID) on %s.%s", table, column)
	case "unique":
		// Note: UNIQUE constraints can't use NOT VALID, so we build a UNIQUE INDEX CONCURRENTLY instead
		addConstraintSQL = fmt.Sprintf("CREATE UNIQUE INDEX CONCURRENTLY %s_idx ON %s(%s)",
			constraintName, table, column)
		addConstraintDesc = fmt.Sprintf("Create UNIQUE index concurrently on %s.%s", table, column)
	}

	phase2 := planner.Phase{
		PhaseNumber:        2,
		Name:               "add_constraint_not_valid",
		Description:        addConstraintDesc,
		RequiresCodeDeploy: false,
		DependsOnPhase:     1,
		CodeChangesRequired: []string{
			"No code changes required",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: addConstraintDesc,
					SQL:         []string{addConstraintSQL},
				},
			},
		},
		Verification: []string{
			fmt.Sprintf("Verify constraint exists: Check pg_constraint for %s", constraintName),
			"Verify new inserts/updates are validated",
		},
		Rollback: &planner.PhaseRollback{
			Description: fmt.Sprintf("Drop constraint %s", constraintName),
			SQL: []string{
				fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", table, constraintName),
			},
			Note: "Safe to rollback - constraint not yet validated on existing rows",
		},
		EstimatedDuration: "< 1 second for NOT VALID constraint",
		LockImpact:        "ShareUpdateExclusive lock (allows reads and writes)",
	}

	if constraintType == "unique" {
		phase2.EstimatedDuration = "Depends on table size (concurrent index build)"
		phase2.LockImpact = "None (CONCURRENTLY avoids locks)"
	}

	phases = append(phases, phase2)

	// Phase 3: Validate Constraint
	var validateSQL string
	var validateDesc string

	switch constraintType {
	case "not_null", "check":
		validateSQL = fmt.Sprintf("ALTER TABLE %s VALIDATE CONSTRAINT %s", table, constraintName)
		validateDesc = fmt.Sprintf("Validate %s constraint on existing rows", constraintType)
	case "unique":
		// For UNIQUE, we convert the index to a constraint
		validateSQL = fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE USING INDEX %s_idx",
			table, constraintName, constraintName)
		validateDesc = fmt.Sprintf("Convert unique index to UNIQUE constraint on %s.%s", table, column)
	}

	phase3 := planner.Phase{
		PhaseNumber:        3,
		Name:               "validate",
		Description:        validateDesc,
		RequiresCodeDeploy: false,
		DependsOnPhase:     2,
		CodeChangesRequired: []string{
			"No code changes required",
		},
		Plan: &planner.Plan{
			SourceHash: sourceHash,
			Steps: []planner.PlanStep{
				{
					Description: validateDesc,
					SQL:         []string{validateSQL},
				},
			},
		},
		Verification: []string{
			"Verify validation completed without errors",
			fmt.Sprintf("Check constraint is valid: SELECT convalidated FROM pg_constraint WHERE conname = '%s'", constraintName),
		},
		Rollback: &planner.PhaseRollback{
			Description: fmt.Sprintf("Drop constraint %s", constraintName),
			SQL: []string{
				fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", table, constraintName),
			},
			Warning: "Rollback removes constraint that was just validated",
		},
		EstimatedDuration: "Depends on table size (scans all rows)",
		LockImpact:        "ShareUpdateExclusive lock during validation",
	}
	phases = append(phases, phase3)

	// Phase 4: Make NOT NULL (for NOT NULL constraints)
	if constraintType == "not_null" {
		phase4 := planner.Phase{
			PhaseNumber:        4,
			Name:               "make_not_null",
			Description:        fmt.Sprintf("Set %s.%s to NOT NULL", table, column),
			RequiresCodeDeploy: false,
			DependsOnPhase:     3,
			CodeChangesRequired: []string{
				"No code changes required",
			},
			Plan: &planner.Plan{
				SourceHash: sourceHash,
				Steps: []planner.PlanStep{
					{
						Description: fmt.Sprintf("Set %s column to NOT NULL", column),
						SQL: []string{
							fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL", table, column),
						},
					},
					{
						Description: "Drop CHECK constraint (now redundant with NOT NULL)",
						SQL: []string{
							fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", table, constraintName),
						},
					},
				},
			},
			Verification: []string{
				fmt.Sprintf("Verify column is NOT NULL: Check pg_attribute for %s.%s", table, column),
				"Attempt to insert NULL (should fail)",
			},
			Rollback: &planner.PhaseRollback{
				Description: fmt.Sprintf("Remove NOT NULL constraint from %s", column),
				SQL: []string{
					fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL", table, column),
				},
				Warning: "Removes NOT NULL - allows NULL values again",
			},
			EstimatedDuration: "< 1 second (no table scan needed)",
			LockImpact:        "AccessExclusive lock (brief)",
		}
		phases = append(phases, phase4)
	}

	constraintDesc := map[string]string{
		"not_null": "NOT NULL",
		"check":    "CHECK",
		"unique":   "UNIQUE",
	}

	safetyNotes := []string{
		fmt.Sprintf("Adding %s constraint safely using validation phase pattern", constraintDesc[constraintType]),
		"Phase 1: Fix existing data to satisfy constraint",
		"Phase 2: Add constraint (NOT VALID) - validates new data only",
		"Phase 3: Validate existing rows (lighter lock)",
	}

	if constraintType == "not_null" {
		safetyNotes = append(safetyNotes, "Phase 4: Make column NOT NULL and drop CHECK constraint")
	}

	safetyNotes = append(safetyNotes, "This approach avoids heavyweight AccessExclusive lock on large tables")

	return &planner.MultiPhasePlan{
		MultiPhase:  true,
		Operation:   fmt.Sprintf("add_%s_constraint", constraintType),
		Description: fmt.Sprintf("Add %s constraint to %s.%s using validation phase pattern", constraintDesc[constraintType], table, column),
		Pattern:     "validation",
		TotalPhases: len(phases),
		Phases:      phases,
		SafetyNotes: safetyNotes,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}, nil
}
