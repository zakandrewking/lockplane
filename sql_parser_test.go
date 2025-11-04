package main

import (
	"testing"
)

func TestParseSQLSchemaCreateTable(t *testing.T) {
	sql := `
CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    email TEXT NOT NULL,
    team_id BIGINT,
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_team_fk FOREIGN KEY (team_id) REFERENCES teams(id)
);
`

	schema, err := ParseSQLSchema(sql)
	if err != nil {
		t.Fatalf("ParseSQLSchema returned error: %v", err)
	}

	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}

	table := schema.Tables[0]
	if table.Name != "users" {
		t.Fatalf("expected table name users, got %s", table.Name)
	}

	if len(table.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(table.Columns))
	}

	idCol := table.Columns[0]
	if !idCol.IsPrimaryKey {
		t.Fatalf("expected id to be primary key")
	}
	if idCol.Nullable {
		t.Fatalf("expected id to be NOT NULL")
	}

	emailCol := table.Columns[1]
	if emailCol.Nullable {
		t.Fatalf("expected email to be NOT NULL")
	}

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}
	if table.Indexes[0].Name != "users_email_key" {
		t.Fatalf("expected unique constraint name users_email_key, got %s", table.Indexes[0].Name)
	}
	if !table.Indexes[0].Unique {
		t.Fatalf("expected users_email_key to be unique")
	}

	if len(table.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key, got %d", len(table.ForeignKeys))
	}
	fk := table.ForeignKeys[0]
	if fk.Name != "users_team_fk" {
		t.Fatalf("expected foreign key name users_team_fk, got %s", fk.Name)
	}
	if fk.ReferencedTable != "teams" {
		t.Fatalf("expected foreign key referenced table teams, got %s", fk.ReferencedTable)
	}
}

func TestParseSQLSchemaAlterColumns(t *testing.T) {
	sql := `
CREATE TABLE users (
    id BIGINT,
    email TEXT
);
ALTER TABLE users ADD COLUMN bio TEXT;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ALTER COLUMN bio TYPE VARCHAR(100);
ALTER TABLE users ALTER COLUMN email SET DEFAULT 'n/a';
`

	schema, err := ParseSQLSchema(sql)
	if err != nil {
		t.Fatalf("ParseSQLSchema returned error: %v", err)
	}

	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}

	table := schema.Tables[0]
	if len(table.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(table.Columns))
	}

	emailCol := table.Columns[1]
	if emailCol.Nullable {
		t.Fatalf("expected email to be NOT NULL")
	}
	if emailCol.Default == nil || *emailCol.Default != "'n/a'" {
		t.Fatalf("expected email default 'n/a', got %v", emailCol.Default)
	}

	bioCol := table.Columns[2]
	if bioCol.Type != "varchar(100)" {
		t.Fatalf("expected bio type varchar(100), got %s", bioCol.Type)
	}
}

func TestParseSQLSchemaAlterColumnCleanup(t *testing.T) {
	sql := `
CREATE TABLE users (
    id BIGINT,
    email TEXT NOT NULL DEFAULT 'n/a'
);
ALTER TABLE users ALTER COLUMN email DROP DEFAULT;
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users DROP COLUMN id;
`

	schema, err := ParseSQLSchema(sql)
	if err != nil {
		t.Fatalf("ParseSQLSchema returned error: %v", err)
	}

	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}

	table := schema.Tables[0]
	if len(table.Columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(table.Columns))
	}

	emailCol := table.Columns[0]
	if emailCol.Default != nil {
		t.Fatalf("expected email default to be nil, got %v", *emailCol.Default)
	}
	if !emailCol.Nullable {
		t.Fatalf("expected email to be nullable after DROP NOT NULL")
	}
}

func TestParseSQLSchemaAlterConstraints(t *testing.T) {
	sql := `
CREATE TABLE users (
    id BIGINT,
    email TEXT
);
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
ALTER TABLE users ADD CONSTRAINT users_team_fk FOREIGN KEY (id) REFERENCES teams(id);
`

	schema, err := ParseSQLSchema(sql)
	if err != nil {
		t.Fatalf("ParseSQLSchema returned error: %v", err)
	}

	table := schema.Tables[0]
	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index after ADD CONSTRAINT, got %d", len(table.Indexes))
	}
	if table.Indexes[0].Name != "users_email_key" {
		t.Fatalf("expected unique constraint users_email_key, got %s", table.Indexes[0].Name)
	}

	if len(table.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key after ADD CONSTRAINT, got %d", len(table.ForeignKeys))
	}
	if table.ForeignKeys[0].Name != "users_team_fk" {
		t.Fatalf("expected foreign key users_team_fk, got %s", table.ForeignKeys[0].Name)
	}
}

func TestParseSQLSchemaAlterConstraintsDrop(t *testing.T) {
	sql := `
CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    email TEXT,
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_team_fk FOREIGN KEY (id) REFERENCES teams(id)
);
ALTER TABLE users DROP CONSTRAINT users_email_key;
ALTER TABLE users DROP CONSTRAINT users_team_fk;
ALTER TABLE users DROP CONSTRAINT users_pkey;
`

	schema, err := ParseSQLSchema(sql)
	if err != nil {
		t.Fatalf("ParseSQLSchema returned error: %v", err)
	}

	table := schema.Tables[0]
	if len(table.Indexes) != 0 {
		t.Fatalf("expected unique constraint to be dropped, still have %d indexes", len(table.Indexes))
	}
	if len(table.ForeignKeys) != 0 {
		t.Fatalf("expected foreign key to be dropped, still have %d foreign keys", len(table.ForeignKeys))
	}
	for _, col := range table.Columns {
		if col.IsPrimaryKey {
			t.Fatalf("expected primary key flags to be cleared")
		}
	}
}
