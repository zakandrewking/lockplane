package main

import (
	"fmt"
	"strings"
)

// GeneratePlan creates a migration plan from a schema diff
func GeneratePlan(diff *SchemaDiff) (*Plan, error) {
	plan := &Plan{
		Steps: []PlanStep{},
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
		sql, desc := generateAddTable(table)
		plan.Steps = append(plan.Steps, PlanStep{
			Description: desc,
			SQL:         sql,
		})

		// Add foreign keys for new tables (after table is created)
		for _, fk := range table.ForeignKeys {
			sql, desc := generateAddForeignKey(table.Name, fk)
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
			sql, desc := generateAddColumn(tableDiff.TableName, col)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Modify existing columns
		for _, colDiff := range tableDiff.ModifiedColumns {
			steps := generateModifyColumn(tableDiff.TableName, colDiff)
			plan.Steps = append(plan.Steps, steps...)
		}

		// Add new foreign keys
		for _, fk := range tableDiff.AddedForeignKeys {
			sql, desc := generateAddForeignKey(tableDiff.TableName, fk)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Add new indexes
		for _, idx := range tableDiff.AddedIndexes {
			sql, desc := generateAddIndex(tableDiff.TableName, idx)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Remove old indexes
		for _, idx := range tableDiff.RemovedIndexes {
			sql, desc := generateDropIndex(tableDiff.TableName, idx)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Remove old foreign keys
		for _, fk := range tableDiff.RemovedForeignKeys {
			sql, desc := generateDropForeignKey(tableDiff.TableName, fk)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}

		// Remove old columns
		for _, col := range tableDiff.RemovedColumns {
			sql, desc := generateDropColumn(tableDiff.TableName, col)
			plan.Steps = append(plan.Steps, PlanStep{
				Description: desc,
				SQL:         sql,
			})
		}
	}

	// Step 7: Remove old tables
	for _, table := range diff.RemovedTables {
		sql, desc := generateDropTable(table)
		plan.Steps = append(plan.Steps, PlanStep{
			Description: desc,
			SQL:         sql,
		})
	}

	return plan, nil
}

// generateAddTable creates SQL to add a new table
func generateAddTable(table Table) (string, string) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", table.Name))

	// Add columns
	for i, col := range table.Columns {
		sb.WriteString("  ")
		sb.WriteString(formatColumnDefinition(col))
		if i < len(table.Columns)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(")")

	description := fmt.Sprintf("Create table %s", table.Name)
	return sb.String(), description
}

// generateDropTable creates SQL to drop a table
func generateDropTable(table Table) (string, string) {
	sql := fmt.Sprintf("DROP TABLE %s CASCADE", table.Name)
	description := fmt.Sprintf("Drop table %s", table.Name)
	return sql, description
}

// generateAddColumn creates SQL to add a column
func generateAddColumn(tableName string, col Column) (string, string) {
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s",
		tableName,
		formatColumnDefinition(col))
	description := fmt.Sprintf("Add column %s to table %s", col.Name, tableName)
	return sql, description
}

// generateDropColumn creates SQL to drop a column
func generateDropColumn(tableName string, col Column) (string, string) {
	sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, col.Name)
	description := fmt.Sprintf("Drop column %s from table %s", col.Name, tableName)
	return sql, description
}

// generateModifyColumn creates SQL steps to modify a column
func generateModifyColumn(tableName string, colDiff ColumnDiff) []PlanStep {
	steps := []PlanStep{}

	// Handle type changes
	if contains(colDiff.Changes, "type") {
		sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s",
			tableName, colDiff.ColumnName, colDiff.New.Type)
		steps = append(steps, PlanStep{
			Description: fmt.Sprintf("Change type of %s.%s from %s to %s",
				tableName, colDiff.ColumnName, colDiff.Old.Type, colDiff.New.Type),
			SQL: sql,
		})
	}

	// Handle nullability changes
	if contains(colDiff.Changes, "nullable") {
		var sql string
		if colDiff.New.Nullable {
			sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL",
				tableName, colDiff.ColumnName)
		} else {
			sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL",
				tableName, colDiff.ColumnName)
		}
		steps = append(steps, PlanStep{
			Description: fmt.Sprintf("Change nullability of %s.%s to %t",
				tableName, colDiff.ColumnName, colDiff.New.Nullable),
			SQL: sql,
		})
	}

	// Handle default value changes
	if contains(colDiff.Changes, "default") {
		var sql string
		if colDiff.New.Default == nil {
			sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT",
				tableName, colDiff.ColumnName)
		} else {
			sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s",
				tableName, colDiff.ColumnName, *colDiff.New.Default)
		}
		steps = append(steps, PlanStep{
			Description: fmt.Sprintf("Change default of %s.%s",
				tableName, colDiff.ColumnName),
			SQL: sql,
		})
	}

	return steps
}

// generateAddIndex creates SQL to add an index
func generateAddIndex(tableName string, idx Index) (string, string) {
	uniqueStr := ""
	if idx.Unique {
		uniqueStr = "UNIQUE "
	}

	// Format column list
	columns := strings.Join(idx.Columns, ", ")

	sql := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
		uniqueStr, idx.Name, tableName, columns)

	description := fmt.Sprintf("Create index %s on table %s", idx.Name, tableName)
	return sql, description
}

// generateDropIndex creates SQL to drop an index
func generateDropIndex(tableName string, idx Index) (string, string) {
	sql := fmt.Sprintf("DROP INDEX %s", idx.Name)
	description := fmt.Sprintf("Drop index %s from table %s", idx.Name, tableName)
	return sql, description
}

// generateAddForeignKey creates SQL to add a foreign key constraint
func generateAddForeignKey(tableName string, fk ForeignKey) (string, string) {
	// Format column lists
	columns := strings.Join(fk.Columns, ", ")
	refColumns := strings.Join(fk.ReferencedColumns, ", ")

	sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		tableName, fk.Name, columns, fk.ReferencedTable, refColumns)

	// Add ON DELETE and ON UPDATE actions if specified
	if fk.OnDelete != nil {
		sql += fmt.Sprintf(" ON DELETE %s", *fk.OnDelete)
	}
	if fk.OnUpdate != nil {
		sql += fmt.Sprintf(" ON UPDATE %s", *fk.OnUpdate)
	}

	description := fmt.Sprintf("Add foreign key %s to table %s", fk.Name, tableName)
	return sql, description
}

// generateDropForeignKey creates SQL to drop a foreign key constraint
func generateDropForeignKey(tableName string, fk ForeignKey) (string, string) {
	sql := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", tableName, fk.Name)
	description := fmt.Sprintf("Drop foreign key %s from table %s", fk.Name, tableName)
	return sql, description
}

// formatColumnDefinition formats a column definition for CREATE/ALTER statements
func formatColumnDefinition(col Column) string {
	var sb strings.Builder

	// Column name and type
	sb.WriteString(fmt.Sprintf("%s %s", col.Name, col.Type))

	// Nullability
	if !col.Nullable {
		sb.WriteString(" NOT NULL")
	}

	// Default value
	if col.Default != nil {
		sb.WriteString(fmt.Sprintf(" DEFAULT %s", *col.Default))
	}

	// Primary key
	if col.IsPrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}

	return sb.String()
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
