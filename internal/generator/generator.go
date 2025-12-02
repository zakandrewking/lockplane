package generator

import (
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/internal/database"
)

// GenerateMigrationSQL generates SQL migration statements to transform the current schema into the desired schema
func GenerateMigrationSQL(current *database.Schema, desired *database.Schema) ([]string, error) {
	var statements []string

	// Build maps for quick lookups
	currentTables := make(map[string]*database.Table)
	for i := range current.Tables {
		currentTables[current.Tables[i].Name] = &current.Tables[i]
	}

	desiredTables := make(map[string]*database.Table)
	for i := range desired.Tables {
		desiredTables[desired.Tables[i].Name] = &desired.Tables[i]
	}

	// Find tables to create
	for _, desiredTable := range desired.Tables {
		if _, exists := currentTables[desiredTable.Name]; !exists {
			stmt, err := generateCreateTableSQL(&desiredTable)
			if err != nil {
				return nil, fmt.Errorf("failed to generate CREATE TABLE for %s: %w", desiredTable.Name, err)
			}
			statements = append(statements, stmt)

			// Add RLS enable statement if needed
			if desiredTable.RLSEnabled {
				statements = append(statements, fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY;", desiredTable.Name))
			}
		}
	}

	// Find tables to alter or drop
	for _, currentTable := range current.Tables {
		desiredTable, exists := desiredTables[currentTable.Name]
		if !exists {
			// Table should be dropped
			statements = append(statements, fmt.Sprintf("DROP TABLE %s;", currentTable.Name))
			continue
		}

		// Check for RLS changes
		if currentTable.RLSEnabled != desiredTable.RLSEnabled {
			if desiredTable.RLSEnabled {
				statements = append(statements, fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY;", currentTable.Name))
			} else {
				statements = append(statements, fmt.Sprintf("ALTER TABLE %s DISABLE ROW LEVEL SECURITY;", currentTable.Name))
			}
		}

		// TODO: Handle column changes, indexes, constraints, etc.
	}

	return statements, nil
}

func generateCreateTableSQL(table *database.Table) (string, error) {
	var parts []string

	// Add columns
	for _, col := range table.Columns {
		colDef := fmt.Sprintf("%s %s", col.Name, col.Type)

		if col.IsPrimaryKey {
			colDef += " PRIMARY KEY"
		}

		if !col.Nullable {
			colDef += " NOT NULL"
		}

		if col.Default != nil {
			colDef += fmt.Sprintf(" DEFAULT %s", *col.Default)
		}

		parts = append(parts, colDef)
	}

	return fmt.Sprintf("CREATE TABLE %s (\n  %s\n);", table.Name, strings.Join(parts, ",\n  ")), nil
}
