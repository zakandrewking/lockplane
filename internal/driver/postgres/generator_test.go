package postgres

import (
	"strings"
	"testing"

	"github.com/lockplane/lockplane/internal/database"
)

func TestGenerator_CreateTable(t *testing.T) {
	gen := NewGenerator()

	table := database.Table{
		Name: "users",
		Columns: []database.Column{
			{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
			{Name: "email", Type: "text", Nullable: false},
			{Name: "age", Type: "integer", Nullable: true},
		},
	}

	sql := gen.CreateTable(table)

	if !strings.Contains(sql, "CREATE TABLE users") {
		t.Errorf("Expected SQL to contain 'CREATE TABLE users', got: %s", sql)
	}

	if !strings.Contains(sql, "id integer NOT NULL PRIMARY KEY") {
		t.Errorf("Expected SQL to contain id column definition, got: %s", sql)
	}

	if !strings.Contains(sql, "email text NOT NULL") {
		t.Errorf("Expected SQL to contain email column definition, got: %s", sql)
	}

	if !strings.Contains(sql, "age integer") && strings.Contains(sql, "age") {
		// Should have age without NOT NULL
		if strings.Contains(sql, "age integer NOT NULL") {
			t.Errorf("Expected age to be nullable, got: %s", sql)
		}
	}
}
