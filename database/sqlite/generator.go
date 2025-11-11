package sqlite

import (
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/database"
)

// Generator implements database.SQLGenerator for SQLite
type Generator struct{}

// NewGenerator creates a new SQLite SQL generator
func NewGenerator() *Generator {
	return &Generator{}
}

// CreateTable generates SQLite SQL to create a table
func (g *Generator) CreateTable(table database.Table) (string, string) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", table.Name))

	// Add columns
	for i, col := range table.Columns {
		sb.WriteString("  ")
		sb.WriteString(g.FormatColumnDefinition(col))
		if i < len(table.Columns)-1 || len(table.ForeignKeys) > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	// Add foreign key constraints
	for i, fk := range table.ForeignKeys {
		sb.WriteString("  ")
		sb.WriteString(g.FormatForeignKeyConstraint(fk))
		if i < len(table.ForeignKeys)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(")")

	description := fmt.Sprintf("Create table %s", table.Name)
	return sb.String(), description
}

// DropTable generates SQLite SQL to drop a table
func (g *Generator) DropTable(table database.Table) (string, string) {
	// SQLite doesn't support CASCADE, but will fail if there are dependencies
	sql := fmt.Sprintf("DROP TABLE %s", table.Name)
	description := fmt.Sprintf("Drop table %s", table.Name)
	return sql, description
}

// AddColumn generates SQLite SQL to add a column
func (g *Generator) AddColumn(tableName string, col database.Column) (string, string) {
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s",
		tableName,
		g.FormatColumnDefinition(col))
	description := fmt.Sprintf("Add column %s to table %s", col.Name, tableName)
	return sql, description
}

// DropColumn generates SQLite SQL to drop a column
func (g *Generator) DropColumn(tableName string, col database.Column) (string, string) {
	// SQLite 3.35.0+ supports DROP COLUMN, but we'll use it directly
	sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, col.Name)
	description := fmt.Sprintf("Drop column %s from table %s", col.Name, tableName)
	return sql, description
}

// ModifyColumn generates SQLite SQL to modify a column
// SQLite doesn't support ALTER COLUMN, so this returns empty steps
// In a production system, you'd implement table recreation here
func (g *Generator) ModifyColumn(tableName string, diff database.ColumnDiff) []database.PlanStep {
	steps := []database.PlanStep{}

	// SQLite doesn't support ALTER COLUMN TYPE, SET NOT NULL, or SET DEFAULT
	// These would require table recreation:
	// 1. CREATE TABLE new_table (with new column definition)
	// 2. INSERT INTO new_table SELECT ... FROM old_table
	// 3. DROP TABLE old_table
	// 4. ALTER TABLE new_table RENAME TO old_table
	//
	// For now, we'll return a warning step indicating this limitation
	if len(diff.Changes) > 0 {
		description := fmt.Sprintf("SQLite limitation: Cannot modify column %s.%s (changes: %s). "+
			"Would require table recreation.", tableName, diff.ColumnName, strings.Join(diff.Changes, ", "))
		steps = append(steps, database.PlanStep{
			Description: description,
			SQL:         []string{fmt.Sprintf("-- %s", description)},
		})
	}

	return steps
}

// AddIndex generates SQLite SQL to add an index
func (g *Generator) AddIndex(tableName string, idx database.Index) (string, string) {
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

// DropIndex generates SQLite SQL to drop an index
func (g *Generator) DropIndex(tableName string, idx database.Index) (string, string) {
	sql := fmt.Sprintf("DROP INDEX %s", idx.Name)
	description := fmt.Sprintf("Drop index %s from table %s", idx.Name, tableName)
	return sql, description
}

// AddForeignKey generates SQLite SQL to add a foreign key
// SQLite doesn't support ALTER TABLE ADD FOREIGN KEY, so we use table recreation
func (g *Generator) AddForeignKey(tableName string, fk database.ForeignKey) (string, string) {
	// Note: This returns the foreign key constraint definition that will be used
	// during table recreation. The actual table recreation logic is handled by
	// RecreateTableWithForeignKey which generates multiple steps.
	description := fmt.Sprintf("Add foreign key %s to table %s (requires table recreation)", fk.Name, tableName)
	sql := g.FormatForeignKeyConstraint(fk)
	return sql, description
}

// DropForeignKey generates SQLite SQL to drop a foreign key
// SQLite doesn't support ALTER TABLE DROP CONSTRAINT, so we use table recreation
func (g *Generator) DropForeignKey(tableName string, fk database.ForeignKey) (string, string) {
	// Note: This returns a comment. The actual table recreation logic is handled by
	// RecreateTableWithoutForeignKey which generates multiple steps.
	description := fmt.Sprintf("Drop foreign key %s from table %s (requires table recreation)", fk.Name, tableName)
	sql := fmt.Sprintf("-- Drop foreign key %s", fk.Name)
	return sql, description
}

// FormatColumnDefinition formats a column definition for CREATE/ALTER statements
func (g *Generator) FormatColumnDefinition(col database.Column) string {
	var sb strings.Builder

	// Column name and type
	sb.WriteString(fmt.Sprintf("%s %s", col.Name, col.Type))

	// Primary key (must come before NOT NULL in SQLite)
	if col.IsPrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}

	// Nullability
	if !col.Nullable {
		sb.WriteString(" NOT NULL")
	}

	// Default value
	if col.Default != nil {
		sb.WriteString(fmt.Sprintf(" DEFAULT %s", *col.Default))
	}

	return sb.String()
}

// FormatForeignKeyConstraint formats a foreign key constraint for CREATE TABLE
func (g *Generator) FormatForeignKeyConstraint(fk database.ForeignKey) string {
	var sb strings.Builder

	// CONSTRAINT name FOREIGN KEY (columns) REFERENCES table (columns)
	sb.WriteString(fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		fk.Name,
		strings.Join(fk.Columns, ", "),
		fk.ReferencedTable,
		strings.Join(fk.ReferencedColumns, ", ")))

	// Add ON DELETE and ON UPDATE actions if specified
	if fk.OnDelete != nil {
		sb.WriteString(fmt.Sprintf(" ON DELETE %s", *fk.OnDelete))
	}
	if fk.OnUpdate != nil {
		sb.WriteString(fmt.Sprintf(" ON UPDATE %s", *fk.OnUpdate))
	}

	return sb.String()
}

// RecreateTableWithForeignKey generates a single atomic step to recreate a table with an added foreign key
// This is the standard SQLite pattern for adding constraints to existing tables
func (g *Generator) RecreateTableWithForeignKey(table database.Table, fk database.ForeignKey) database.PlanStep {
	tmpTableName := fmt.Sprintf("%s_new", table.Name)

	// Create new table with the foreign key
	newTable := table
	newTable.Name = tmpTableName
	newTable.ForeignKeys = append(newTable.ForeignKeys, fk)

	createSQL, _ := g.CreateTable(newTable)

	// Build column list for data copy
	columnNames := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		columnNames[i] = col.Name
	}
	columnsStr := strings.Join(columnNames, ", ")

	// Return single step with all SQL statements
	return database.PlanStep{
		Description: fmt.Sprintf("Add foreign key %s to table %s", fk.Name, table.Name),
		SQL: []string{
			createSQL,
			fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", tmpTableName, columnsStr, columnsStr, table.Name),
			fmt.Sprintf("DROP TABLE %s", table.Name),
			fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tmpTableName, table.Name),
		},
	}
}

// RecreateTableWithoutForeignKey generates a single atomic step to recreate a table without a specific foreign key
func (g *Generator) RecreateTableWithoutForeignKey(table database.Table, fkName string) database.PlanStep {
	tmpTableName := fmt.Sprintf("%s_new", table.Name)

	// Create new table without the foreign key
	newTable := table
	newTable.Name = tmpTableName
	newForeignKeys := []database.ForeignKey{}
	for _, existingFK := range table.ForeignKeys {
		if existingFK.Name != fkName {
			newForeignKeys = append(newForeignKeys, existingFK)
		}
	}
	newTable.ForeignKeys = newForeignKeys

	createSQL, _ := g.CreateTable(newTable)

	// Build column list for data copy
	columnNames := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		columnNames[i] = col.Name
	}
	columnsStr := strings.Join(columnNames, ", ")

	// Return single step with all SQL statements
	return database.PlanStep{
		Description: fmt.Sprintf("Drop foreign key %s from table %s", fkName, table.Name),
		SQL: []string{
			createSQL,
			fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", tmpTableName, columnsStr, columnsStr, table.Name),
			fmt.Sprintf("DROP TABLE %s", table.Name),
			fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tmpTableName, table.Name),
		},
	}
}

// ParameterPlaceholder returns the SQLite parameter placeholder (?)
func (g *Generator) ParameterPlaceholder(position int) string {
	return "?"
}
