package planner

import (
	"fmt"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/schema"
)

// GeneratePlan creates a migration plan from a schema diff using the provided driver
func GeneratePlan(diff *schema.SchemaDiff, driver database.Driver) (*Plan, error) {
	return GeneratePlanWithHash(diff, nil, driver)
}

// GeneratePlanWithHash creates a migration plan with a source schema hash using the provided driver
func GeneratePlanWithHash(diff *schema.SchemaDiff, sourceSchema *database.Schema, driver database.Driver) (*Plan, error) {
	plan := &Plan{
		Steps: []PlanStep{},
	}

	// Compute source schema hash if provided
	if sourceSchema != nil {
		hash, err := schema.ComputeSchemaHash(sourceSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to compute source schema hash: %w", err)
		}
		plan.SourceHash = hash
	}

	// Order of operations for safe migrations:
	// 1. Add new tables
	// 2. Add new columns to existing tables
	// 3. Modify columns (type changes, nullability, defaults)
	// 4. Add foreign keys (after referenced tables/columns exist)
	// 5. Add indexes
	// 6. Remove indexes (from removed tables or columns)
	// 7. Remove foreign keys (before referenced tables/columns are dropped)
	// 8. Remove columns
	// 9. Remove tables

	// Step 1: Add new tables
	for _, table := range diff.AddedTables {
		sql, desc := driver.CreateTable(table)
		plan.Steps = append(plan.Steps, PlanStep{
			Description: desc,
			SQL:         sql,
		})

		// Add foreign keys for new tables (after table is created)
		for _, fk := range table.ForeignKeys {
			sql, desc := driver.AddForeignKey(table.Name, fk)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}
	}

	// Step 2-4: Process table modifications
	for _, tableDiff := range diff.ModifiedTables {
		// Add new columns
		for _, col := range tableDiff.AddedColumns {
			sql, desc := driver.AddColumn(tableDiff.TableName, col)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Modify existing columns
		for _, colDiff := range tableDiff.ModifiedColumns {
			// Convert main.ColumnDiff to database.ColumnDiff
			dbColDiff := database.ColumnDiff{
				ColumnName: colDiff.ColumnName,
				Old:        colDiff.Old,
				New:        colDiff.New,
				Changes:    colDiff.Changes,
			}
			steps := driver.ModifyColumn(tableDiff.TableName, dbColDiff)
			// Convert []database.PlanStep to []PlanStep
			for _, step := range steps {
				plan.Steps = append(plan.Steps, PlanStep{
					Description: step.Description,
					SQL:         step.SQL,
				})
			}
		}

		// Add new foreign keys
		for _, fk := range tableDiff.AddedForeignKeys {
			sql, desc := driver.AddForeignKey(tableDiff.TableName, fk)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Add new indexes
		for _, idx := range tableDiff.AddedIndexes {
			sql, desc := driver.AddIndex(tableDiff.TableName, idx)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Remove old indexes
		for _, idx := range tableDiff.RemovedIndexes {
			sql, desc := driver.DropIndex(tableDiff.TableName, idx)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Remove old foreign keys
		for _, fk := range tableDiff.RemovedForeignKeys {
			sql, desc := driver.DropForeignKey(tableDiff.TableName, fk)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Remove old columns
		for _, col := range tableDiff.RemovedColumns {
			sql, desc := driver.DropColumn(tableDiff.TableName, col)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}
	}

	// Step 7: Remove old tables
	for _, table := range diff.RemovedTables {
		sql, desc := driver.DropTable(table)
		plan.Steps = append(plan.Steps, PlanStep{
			Description: desc,
			SQL:         sql,
		})
	}

	return plan, nil
}
