package database

import (
	"encoding/json"
	"testing"
)

func TestSchemaJSONMarshaling(t *testing.T) {
	schema := &Schema{
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "email", Type: "text", Nullable: false},
				},
				Indexes: []Index{
					{Name: "idx_users_email", Columns: []string{"email"}, Unique: true},
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal schema to JSON: %v", err)
	}

	// Unmarshal back
	var unmarshaled Schema
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal schema from JSON: %v", err)
	}

	// Verify structure
	if len(unmarshaled.Tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(unmarshaled.Tables))
	}

	table := unmarshaled.Tables[0]
	if table.Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", table.Name)
	}

	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}

	if len(table.Indexes) != 1 {
		t.Errorf("Expected 1 index, got %d", len(table.Indexes))
	}
}

func TestTableWithForeignKeys(t *testing.T) {
	onDelete := "CASCADE"
	table := Table{
		Name: "posts",
		Columns: []Column{
			{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
			{Name: "user_id", Type: "integer", Nullable: false},
		},
		ForeignKeys: []ForeignKey{
			{
				Name:              "fk_posts_user_id",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
				OnDelete:          &onDelete,
			},
		},
	}

	// Marshal and unmarshal
	data, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("Failed to marshal table: %v", err)
	}

	var unmarshaled Table
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal table: %v", err)
	}

	// Verify foreign key
	if len(unmarshaled.ForeignKeys) != 1 {
		t.Fatalf("Expected 1 foreign key, got %d", len(unmarshaled.ForeignKeys))
	}

	fk := unmarshaled.ForeignKeys[0]
	if fk.Name != "fk_posts_user_id" {
		t.Errorf("Expected FK name 'fk_posts_user_id', got '%s'", fk.Name)
	}

	if fk.ReferencedTable != "users" {
		t.Errorf("Expected referenced table 'users', got '%s'", fk.ReferencedTable)
	}

	if fk.OnDelete == nil || *fk.OnDelete != "CASCADE" {
		t.Errorf("Expected OnDelete 'CASCADE', got %v", fk.OnDelete)
	}
}

func TestColumnWithDefault(t *testing.T) {
	defaultVal := "now()"
	column := Column{
		Name:     "created_at",
		Type:     "timestamp",
		Nullable: false,
		Default:  &defaultVal,
	}

	data, err := json.Marshal(column)
	if err != nil {
		t.Fatalf("Failed to marshal column: %v", err)
	}

	var unmarshaled Column
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal column: %v", err)
	}

	if unmarshaled.Default == nil {
		t.Fatal("Expected default value, got nil")
	}

	if *unmarshaled.Default != "now()" {
		t.Errorf("Expected default 'now()', got '%s'", *unmarshaled.Default)
	}
}

func TestColumnWithoutDefault(t *testing.T) {
	column := Column{
		Name:     "name",
		Type:     "text",
		Nullable: true,
	}

	data, err := json.Marshal(column)
	if err != nil {
		t.Fatalf("Failed to marshal column: %v", err)
	}

	var unmarshaled Column
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal column: %v", err)
	}

	if unmarshaled.Default != nil {
		t.Errorf("Expected no default value, got %v", unmarshaled.Default)
	}
}

func TestPlanStepMarshaling(t *testing.T) {
	step := PlanStep{
		Description: "Create table users",
		SQL:         "CREATE TABLE users (id integer PRIMARY KEY)",
	}

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("Failed to marshal plan step: %v", err)
	}

	var unmarshaled PlanStep
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal plan step: %v", err)
	}

	if unmarshaled.Description != step.Description {
		t.Errorf("Expected description '%s', got '%s'", step.Description, unmarshaled.Description)
	}

	if unmarshaled.SQL != step.SQL {
		t.Errorf("Expected SQL '%s', got '%s'", step.SQL, unmarshaled.SQL)
	}
}

func TestColumnDiff(t *testing.T) {
	diff := ColumnDiff{
		ColumnName: "age",
		Old:        Column{Name: "age", Type: "integer", Nullable: true},
		New:        Column{Name: "age", Type: "bigint", Nullable: false},
		Changes:    []string{"type", "nullable"},
	}

	if diff.ColumnName != "age" {
		t.Errorf("Expected column name 'age', got '%s'", diff.ColumnName)
	}

	if len(diff.Changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(diff.Changes))
	}
}

func TestIndexWithMultipleColumns(t *testing.T) {
	index := Index{
		Name:    "idx_users_name_email",
		Columns: []string{"name", "email"},
		Unique:  true,
	}

	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Failed to marshal index: %v", err)
	}

	var unmarshaled Index
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal index: %v", err)
	}

	if len(unmarshaled.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(unmarshaled.Columns))
	}

	if unmarshaled.Columns[0] != "name" || unmarshaled.Columns[1] != "email" {
		t.Errorf("Expected columns [name, email], got %v", unmarshaled.Columns)
	}

	if !unmarshaled.Unique {
		t.Error("Expected unique index")
	}
}
