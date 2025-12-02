package postgres

import (
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/schema"
)

// Generator implements database.SQLGenerator for PostgreSQL
type Generator struct{}

// NewGenerator creates a new PostgreSQL SQL generator
func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) GenerateMigration(diff *schema.SchemaDiff) string {
	migration := ""
	for _, table := range diff.AddedTables {
		migration += g.CreateTable(table) + "\n\n"
	}
	for _, tableDiff := range diff.ModifiedTables {
		for _, columnDiff := range tableDiff.ModifiedColumns {
			migration += g.ModifyColumn(tableDiff.TableName, columnDiff) + "\n\n"
		}
	}
	for _, table := range diff.RemovedTables {
		migration += g.DropTable(table) + "\n\n"
	}
	return strings.TrimSpace(migration)
}

// CreateTable generates PostgreSQL SQL to create a table
func (g *Generator) CreateTable(table database.Table) string {
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

	return sb.String()
}

// DropTable generates PostgreSQL SQL to drop a table
func (g *Generator) DropTable(table database.Table) string {
	return fmt.Sprintf("DROP TABLE %s CASCADE", table.Name)
}

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

func (g *Generator) AddColumn(tableName string, col database.Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", tableName, g.FormatColumnDefinition(col))
}

func (g *Generator) DropColumn(tableName string, col database.Column) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, col.Name)
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

func (g *Generator) ModifyColumn(tableName string, diff schema.ColumnDiff) string {
	sql := ""

	// Handle type changes
	if contains(diff.Changes, "type") {
		sql += fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s\n\n",
			tableName, diff.ColumnName, diff.New.Type)
	}

	// Handle nullability changes
	if contains(diff.Changes, "nullable") {
		if diff.New.Nullable {
			sql += fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL\n\n",
				tableName, diff.ColumnName)
		} else {
			sql += fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL\n\n",
				tableName, diff.ColumnName)
		}
	}

	// Handle default value changes
	if contains(diff.Changes, "default") {
		if diff.New.Default == nil {
			sql += fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT\n\n",
				tableName, diff.ColumnName)
		} else {
			sql += fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s\n\n",
				tableName, diff.ColumnName, *diff.New.Default)
		}
	}

	return strings.TrimSpace(sql)
}
