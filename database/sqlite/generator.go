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
		if i < len(table.Columns)-1 {
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
			SQL:         fmt.Sprintf("-- %s", description),
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
// Note: SQLite only supports foreign keys defined at table creation
func (g *Generator) AddForeignKey(tableName string, fk database.ForeignKey) (string, string) {
	// SQLite doesn't support ALTER TABLE ADD FOREIGN KEY
	// Foreign keys must be defined at table creation
	description := fmt.Sprintf("SQLite limitation: Cannot add foreign key %s to existing table %s. "+
		"Foreign keys must be defined at table creation.", fk.Name, tableName)
	sql := fmt.Sprintf("-- %s", description)
	return sql, description
}

// DropForeignKey generates SQLite SQL to drop a foreign key
func (g *Generator) DropForeignKey(tableName string, fk database.ForeignKey) (string, string) {
	// SQLite doesn't support ALTER TABLE DROP CONSTRAINT
	description := fmt.Sprintf("SQLite limitation: Cannot drop foreign key %s from table %s. "+
		"Would require table recreation.", fk.Name, tableName)
	sql := fmt.Sprintf("-- %s", description)
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

// ParameterPlaceholder returns the SQLite parameter placeholder (?)
func (g *Generator) ParameterPlaceholder(position int) string {
	return "?"
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
