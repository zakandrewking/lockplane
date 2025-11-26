package planner

import (
	"fmt"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/parser"
)

// GenerateRollback creates a rollback plan from a forward migration plan
// It reverses the operations and their order to undo the forward migration
func GenerateRollback(forwardPlan *Plan, beforeSchema *database.Schema, driver database.Driver) (*Plan, error) {
	rollbackPlan := &Plan{
		Steps: []PlanStep{},
	}

	// Process steps in reverse order
	for i := len(forwardPlan.Steps) - 1; i >= 0; i-- {
		step := forwardPlan.Steps[i]

		// Generate reverse operation based on the forward step
		reverseSteps, err := generateReverseOperation(step, beforeSchema, driver)
		if err != nil {
			return nil, fmt.Errorf("failed to generate reverse for step %d (%s): %w", i, step.Description, err)
		}

		rollbackPlan.Steps = append(rollbackPlan.Steps, reverseSteps...)
	}

	return rollbackPlan, nil
}

// generateReverseOperation creates the reverse operation for a given step
func generateReverseOperation(step PlanStep, beforeSchema *database.Schema, driver database.Driver) ([]PlanStep, error) {
	// Parse the SQL to determine operation type
	// For steps with multiple SQL statements, we check the first statement to determine the operation type
	sqlStmt := step.SQL[0]

	if parser.ContainsSQL(sqlStmt, "CREATE TABLE") {
		return generateReverseCreateTable(step)
	} else if parser.ContainsSQL(sqlStmt, "DROP TABLE") {
		return generateReverseDropTable(step, beforeSchema, driver)
	} else if parser.ContainsSQL(sqlStmt, "ADD COLUMN") {
		return generateReverseAddColumn(step)
	} else if parser.ContainsSQL(sqlStmt, "DROP COLUMN") {
		return generateReverseDropColumn(step, beforeSchema, driver)
	} else if parser.ContainsSQL(sqlStmt, "ALTER COLUMN") && parser.ContainsSQL(sqlStmt, "TYPE") {
		return generateReverseAlterColumnType(step, beforeSchema)
	} else if parser.ContainsSQL(sqlStmt, "SET NOT NULL") {
		return generateReverseSetNotNull(step)
	} else if parser.ContainsSQL(sqlStmt, "DROP NOT NULL") {
		return generateReverseDropNotNull(step)
	} else if parser.ContainsSQL(sqlStmt, "SET DEFAULT") {
		return generateReverseSetDefault(step, beforeSchema)
	} else if parser.ContainsSQL(sqlStmt, "DROP DEFAULT") {
		return generateReverseDropDefault(step, beforeSchema)
	} else if parser.ContainsSQL(sqlStmt, "CREATE INDEX") || parser.ContainsSQL(sqlStmt, "CREATE UNIQUE INDEX") {
		return generateReverseCreateIndex(step)
	} else if parser.ContainsSQL(sqlStmt, "DROP INDEX") {
		return generateReverseDropIndex(step, beforeSchema, driver)
	} else if parser.ContainsSQL(sqlStmt, "ADD CONSTRAINT") && parser.ContainsSQL(sqlStmt, "FOREIGN KEY") {
		return generateReverseAddForeignKey(step)
	} else if parser.ContainsSQL(sqlStmt, "DROP CONSTRAINT") {
		return generateReverseDropForeignKey(step, beforeSchema, driver)
	} else if parser.ContainsSQL(sqlStmt, "ENABLE ROW LEVEL SECURITY") {
		return generateReverseEnableRLS(step)
	} else if parser.ContainsSQL(sqlStmt, "DISABLE ROW LEVEL SECURITY") {
		return generateReverseDisableRLS(step)
	}

	return nil, fmt.Errorf("unsupported operation for rollback: %v", step.SQL)
}

// generateReverseCreateTable creates a DROP TABLE statement
func generateReverseCreateTable(step PlanStep) ([]PlanStep, error) {
	// Extract table name from "CREATE TABLE tablename ..."
	// Simplified: assumes format "CREATE TABLE <name> ..."
	// For multi-statement steps, we look at the first statement
	sqlStmt := step.SQL[0]
	tableName, err := parser.ExtractTableNameFromCreate(sqlStmt)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("DROP TABLE %s CASCADE", tableName)
	desc := fmt.Sprintf("Rollback: Drop table %s", tableName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseDropTable recreates the table
func generateReverseDropTable(step PlanStep, beforeSchema *database.Schema, driver database.Driver) ([]PlanStep, error) {
	// Extract table name from "DROP TABLE tablename"
	sqlStmt := step.SQL[0]
	tableName, err := parser.ExtractTableNameFromDrop(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Find the table in the before schema
	var table *database.Table
	for i := range beforeSchema.Tables {
		if beforeSchema.Tables[i].Name == tableName {
			table = &beforeSchema.Tables[i]
			break
		}
	}

	if table == nil {
		return nil, fmt.Errorf("table %s not found in before schema", tableName)
	}

	// Generate CREATE TABLE statement using driver
	sql, desc := driver.CreateTable(*table)
	return []PlanStep{{Description: fmt.Sprintf("Rollback: %s", desc), SQL: []string{sql}}}, nil
}

// generateReverseAddColumn creates a DROP COLUMN statement
func generateReverseAddColumn(step PlanStep) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	tableName, columnName, err := parser.ExtractTableAndColumnFromAddColumn(sqlStmt)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, columnName)
	desc := fmt.Sprintf("Rollback: Drop column %s from table %s", columnName, tableName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseDropColumn recreates the column
func generateReverseDropColumn(step PlanStep, beforeSchema *database.Schema, driver database.Driver) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	tableName, columnName, err := parser.ExtractTableAndColumnFromDropColumn(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Find the column definition in the before schema
	column, err := findColumn(beforeSchema, tableName, columnName)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", tableName, driver.FormatColumnDefinition(*column))
	desc := fmt.Sprintf("Rollback: Add column %s to table %s", columnName, tableName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseAlterColumnType changes the column type back
func generateReverseAlterColumnType(step PlanStep, beforeSchema *database.Schema) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	tableName, columnName, err := parser.ExtractTableAndColumnFromAlterType(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Find the original column type
	column, err := findColumn(beforeSchema, tableName, columnName)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s", tableName, columnName, column.Type)
	desc := fmt.Sprintf("Rollback: Change type of %s.%s back to %s", tableName, columnName, column.Type)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseSetNotNull drops NOT NULL
func generateReverseSetNotNull(step PlanStep) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	tableName, columnName, err := parser.ExtractTableAndColumnFromAlterNotNull(sqlStmt)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL", tableName, columnName)
	desc := fmt.Sprintf("Rollback: Allow nulls in %s.%s", tableName, columnName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseDropNotNull sets NOT NULL
func generateReverseDropNotNull(step PlanStep) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	tableName, columnName, err := parser.ExtractTableAndColumnFromAlterNotNull(sqlStmt)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL", tableName, columnName)
	desc := fmt.Sprintf("Rollback: Require non-null in %s.%s", tableName, columnName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseSetDefault drops the default
func generateReverseSetDefault(step PlanStep, beforeSchema *database.Schema) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	tableName, columnName, err := parser.ExtractTableAndColumnFromSetDefault(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Check if there was a previous default
	column, err := findColumn(beforeSchema, tableName, columnName)
	if err != nil {
		return nil, err
	}

	var sql string
	if column.Default == nil {
		sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT", tableName, columnName)
	} else {
		sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s", tableName, columnName, *column.Default)
	}

	desc := fmt.Sprintf("Rollback: Restore default for %s.%s", tableName, columnName)
	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseDropDefault restores the default
func generateReverseDropDefault(step PlanStep, beforeSchema *database.Schema) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	tableName, columnName, err := parser.ExtractTableAndColumnFromDropDefault(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Find the original default value
	column, err := findColumn(beforeSchema, tableName, columnName)
	if err != nil {
		return nil, err
	}

	if column.Default == nil {
		return nil, fmt.Errorf("column %s.%s had no default value", tableName, columnName)
	}

	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s", tableName, columnName, *column.Default)
	desc := fmt.Sprintf("Rollback: Restore default for %s.%s", tableName, columnName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseCreateIndex drops the index
func generateReverseCreateIndex(step PlanStep) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	indexName, err := parser.ExtractIndexNameFromCreate(sqlStmt)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("DROP INDEX %s", indexName)
	desc := fmt.Sprintf("Rollback: Drop index %s", indexName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseDropIndex recreates the index
func generateReverseDropIndex(step PlanStep, beforeSchema *database.Schema, driver database.Driver) ([]PlanStep, error) {
	sqlStmt := step.SQL[0]
	indexName, err := parser.ExtractIndexNameFromDrop(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Find the index in the before schema
	tableName, index, err := findIndex(beforeSchema, indexName)
	if err != nil {
		return nil, err
	}

	sql, desc := driver.AddIndex(tableName, *index)
	return []PlanStep{{Description: fmt.Sprintf("Rollback: %s", desc), SQL: []string{sql}}}, nil
}

// Helper function to find a column in a schema
func findColumn(schema *database.Schema, tableName, columnName string) (*database.Column, error) {
	for _, table := range schema.Tables {
		if table.Name == tableName {
			for i := range table.Columns {
				if table.Columns[i].Name == columnName {
					return &table.Columns[i], nil
				}
			}
			return nil, fmt.Errorf("column %s not found in table %s", columnName, tableName)
		}
	}
	return nil, fmt.Errorf("table %s not found", tableName)
}

// Helper function to find an index in a schema
func findIndex(schema *database.Schema, indexName string) (string, *database.Index, error) {
	for _, table := range schema.Tables {
		for i := range table.Indexes {
			if table.Indexes[i].Name == indexName {
				return table.Name, &table.Indexes[i], nil
			}
		}
	}
	return "", nil, fmt.Errorf("index %s not found", indexName)
}

// Helper function to find a foreign key in a schema
func findForeignKey(schema *database.Schema, fkName string) (string, *database.ForeignKey, error) {
	for _, table := range schema.Tables {
		for i := range table.ForeignKeys {
			if table.ForeignKeys[i].Name == fkName {
				return table.Name, &table.ForeignKeys[i], nil
			}
		}
	}
	return "", nil, fmt.Errorf("foreign key %s not found", fkName)
}

// generateReverseAddForeignKey creates a DROP CONSTRAINT statement
func generateReverseAddForeignKey(step PlanStep) ([]PlanStep, error) {
	// Extract table name and constraint name from "ALTER TABLE tablename ADD CONSTRAINT constraintname ..."
	// For multi-statement steps (SQLite), look at the first CREATE TABLE statement
	sqlStmt := step.SQL[0]
	tableName, constraintName, err := parser.ExtractTableAndConstraintFromAddConstraint(sqlStmt)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", tableName, constraintName)
	desc := fmt.Sprintf("Rollback: Drop foreign key %s from table %s", constraintName, tableName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseDropForeignKey recreates the foreign key constraint
func generateReverseDropForeignKey(step PlanStep, beforeSchema *database.Schema, driver database.Driver) ([]PlanStep, error) {
	// Extract table name and constraint name from "ALTER TABLE tablename DROP CONSTRAINT constraintname"
	// For multi-statement steps (SQLite), we need to extract from the description or first statement
	sqlStmt := step.SQL[0]
	tableName, constraintName, err := parser.ExtractTableAndConstraintFromDropConstraint(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Find the foreign key in the before schema
	foundTableName, fk, err := findForeignKey(beforeSchema, constraintName)
	if err != nil {
		return nil, err
	}

	// Verify table name matches
	if foundTableName != tableName {
		return nil, fmt.Errorf("foreign key %s found in table %s, expected %s", constraintName, foundTableName, tableName)
	}

	sql, desc := driver.AddForeignKey(tableName, *fk)
	return []PlanStep{{Description: fmt.Sprintf("Rollback: %s", desc), SQL: []string{sql}}}, nil
}

// generateReverseEnableRLS creates a DISABLE ROW LEVEL SECURITY statement
func generateReverseEnableRLS(step PlanStep) ([]PlanStep, error) {
	// Extract table name from "ALTER TABLE tablename ENABLE ROW LEVEL SECURITY"
	sqlStmt := step.SQL[0]
	tableName, err := parser.ExtractTableNameFromAlter(sqlStmt)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER TABLE %s DISABLE ROW LEVEL SECURITY", tableName)
	desc := fmt.Sprintf("Rollback: Disable row level security on table %s", tableName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}

// generateReverseDisableRLS creates an ENABLE ROW LEVEL SECURITY statement
func generateReverseDisableRLS(step PlanStep) ([]PlanStep, error) {
	// Extract table name from "ALTER TABLE tablename DISABLE ROW LEVEL SECURITY"
	sqlStmt := step.SQL[0]
	tableName, err := parser.ExtractTableNameFromAlter(sqlStmt)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", tableName)
	desc := fmt.Sprintf("Rollback: Enable row level security on table %s", tableName)

	return []PlanStep{{Description: desc, SQL: []string{sql}}}, nil
}
