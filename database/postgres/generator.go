package postgres

import (
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/database"
)

// Generator implements database.SQLGenerator for PostgreSQL
type Generator struct{}

// NewGenerator creates a new PostgreSQL SQL generator
func NewGenerator() *Generator {
	return &Generator{}
}

// CreateTable generates PostgreSQL SQL to create a table
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

// DropTable generates PostgreSQL SQL to drop a table
func (g *Generator) DropTable(table database.Table) (string, string) {
	sql := fmt.Sprintf("DROP TABLE %s CASCADE", table.Name)
	description := fmt.Sprintf("Drop table %s", table.Name)
	return sql, description
}

// AddColumn generates PostgreSQL SQL to add a column
func (g *Generator) AddColumn(tableName string, col database.Column) (string, string) {
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s",
		tableName,
		g.FormatColumnDefinition(col))
	description := fmt.Sprintf("Add column %s to table %s", col.Name, tableName)
	return sql, description
}

// DropColumn generates PostgreSQL SQL to drop a column
func (g *Generator) DropColumn(tableName string, col database.Column) (string, string) {
	sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, col.Name)
	description := fmt.Sprintf("Drop column %s from table %s", col.Name, tableName)
	return sql, description
}

// ModifyColumn generates PostgreSQL SQL to modify a column
func (g *Generator) ModifyColumn(tableName string, diff database.ColumnDiff) []database.PlanStep {
	steps := []database.PlanStep{}

	// Handle type changes
	if contains(diff.Changes, "type") {
		sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s",
			tableName, diff.ColumnName, diff.New.Type)
		steps = append(steps, database.PlanStep{
			Description: fmt.Sprintf("Change type of %s.%s from %s to %s",
				tableName, diff.ColumnName, diff.Old.Type, diff.New.Type),
			SQL: []string{sql},
		})
	}

	// Handle nullability changes
	if contains(diff.Changes, "nullable") {
		var sql string
		if diff.New.Nullable {
			sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL",
				tableName, diff.ColumnName)
		} else {
			sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL",
				tableName, diff.ColumnName)
		}
		steps = append(steps, database.PlanStep{
			Description: fmt.Sprintf("Change nullability of %s.%s to %t",
				tableName, diff.ColumnName, diff.New.Nullable),
			SQL: []string{sql},
		})
	}

	// Handle default value changes
	if contains(diff.Changes, "default") {
		var sql string
		if diff.New.Default == nil {
			sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT",
				tableName, diff.ColumnName)
		} else {
			sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s",
				tableName, diff.ColumnName, *diff.New.Default)
		}
		steps = append(steps, database.PlanStep{
			Description: fmt.Sprintf("Change default of %s.%s",
				tableName, diff.ColumnName),
			SQL: []string{sql},
		})
	}

	return steps
}

// AddIndex generates PostgreSQL SQL to add an index
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

// DropIndex generates PostgreSQL SQL to drop an index
func (g *Generator) DropIndex(tableName string, idx database.Index) (string, string) {
	sql := fmt.Sprintf("DROP INDEX %s", idx.Name)
	description := fmt.Sprintf("Drop index %s from table %s", idx.Name, tableName)
	return sql, description
}

// AddForeignKey generates PostgreSQL SQL to add a foreign key
func (g *Generator) AddForeignKey(tableName string, fk database.ForeignKey) (string, string) {
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

// DropForeignKey generates PostgreSQL SQL to drop a foreign key
func (g *Generator) DropForeignKey(tableName string, fk database.ForeignKey) (string, string) {
	sql := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", tableName, fk.Name)
	description := fmt.Sprintf("Drop foreign key %s from table %s", fk.Name, tableName)
	return sql, description
}

// FormatColumnDefinition formats a column definition for CREATE/ALTER statements
func (g *Generator) FormatColumnDefinition(col database.Column) string {
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

// ParameterPlaceholder returns the PostgreSQL parameter placeholder ($1, $2, etc.)
func (g *Generator) ParameterPlaceholder(position int) string {
	return fmt.Sprintf("$%d", position)
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
