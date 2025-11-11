package planner

import (
	"fmt"

	"github.com/lockplane/lockplane/database"
	sqlitedb "github.com/lockplane/lockplane/database/sqlite"
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
		// For SQLite, foreign keys are included in CREATE TABLE, so skip this step
		if driver.SupportsFeature("ALTER_ADD_FOREIGN_KEY") {
			for _, fk := range table.ForeignKeys {
				sql, desc := driver.AddForeignKey(table.Name, fk)
				plan.Steps = append(plan.Steps, PlanStep{
					Description: desc,
					SQL:         sql,
				})
			}
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
			// For SQLite, adding foreign keys requires table recreation
			if driver.Name() == "sqlite" && !driver.SupportsFeature("ALTER_ADD_FOREIGN_KEY") {
				if sqliteGen, ok := driver.(*sqlitedb.Driver); ok {
					// Find the source table to get its current definition
					var sourceTable *database.Table
					if sourceSchema != nil {
						for i := range sourceSchema.Tables {
							if sourceSchema.Tables[i].Name == tableDiff.TableName {
								sourceTable = &sourceSchema.Tables[i]
								break
							}
						}
					}

					if sourceTable != nil {
						// Use table recreation for SQLite
						steps := sqliteGen.Generator.RecreateTableWithForeignKey(*sourceTable, fk)
						for _, step := range steps {
							plan.Steps = append(plan.Steps, PlanStep{
								Description: step.Description,
								SQL:         step.SQL,
							})
						}
					} else {
						// Fallback if we can't find the source table
						sql, desc := driver.AddForeignKey(tableDiff.TableName, fk)
						plan.Steps = append(plan.Steps, PlanStep{
							Description: desc,
							SQL:         sql,
						})
					}
				} else {
					// Should not happen, but fallback just in case
					sql, desc := driver.AddForeignKey(tableDiff.TableName, fk)
					plan.Steps = append(plan.Steps, PlanStep{
						Description: desc,
						SQL:         sql,
					})
				}
			} else {
				// PostgreSQL and other databases can add foreign keys directly
				sql, desc := driver.AddForeignKey(tableDiff.TableName, fk)
				plan.Steps = append(plan.Steps, PlanStep{
					Description: desc,
					SQL:         sql,
				})
			}
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
			// For SQLite, dropping foreign keys requires table recreation
			if driver.Name() == "sqlite" && !driver.SupportsFeature("ALTER_ADD_FOREIGN_KEY") {
				if sqliteGen, ok := driver.(*sqlitedb.Driver); ok {
					// Find the source table to get its current definition
					var sourceTable *database.Table
					if sourceSchema != nil {
						for i := range sourceSchema.Tables {
							if sourceSchema.Tables[i].Name == tableDiff.TableName {
								sourceTable = &sourceSchema.Tables[i]
								break
							}
						}
					}

					if sourceTable != nil {
						// Use table recreation for SQLite
						steps := sqliteGen.Generator.RecreateTableWithoutForeignKey(*sourceTable, fk.Name)
						for _, step := range steps {
							plan.Steps = append(plan.Steps, PlanStep{
								Description: step.Description,
								SQL:         step.SQL,
							})
						}
					} else {
						// Fallback if we can't find the source table
						sql, desc := driver.DropForeignKey(tableDiff.TableName, fk)
						plan.Steps = append(plan.Steps, PlanStep{
							Description: desc,
							SQL:         sql,
						})
					}
				} else {
					// Should not happen, but fallback just in case
					sql, desc := driver.DropForeignKey(tableDiff.TableName, fk)
					plan.Steps = append(plan.Steps, PlanStep{
						Description: desc,
						SQL:         sql,
					})
				}
			} else {
				// PostgreSQL and other databases can drop foreign keys directly
				sql, desc := driver.DropForeignKey(tableDiff.TableName, fk)
				plan.Steps = append(plan.Steps, PlanStep{
					Description: desc,
					SQL:         sql,
				})
			}
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
