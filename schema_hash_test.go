package main

import (
	"testing"

	"github.com/lockplane/lockplane/database"
)

// TestComputeSchemaHash_LogicalTypeEquivalence verifies that schemas with
// different raw types but same logical types produce the same hash
func TestComputeSchemaHash_LogicalTypeEquivalence(t *testing.T) {
	// SQLite schema with INTEGER type
	sqliteSchema := &Schema{
		Dialect: database.DialectSQLite,
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{
						Name:         "id",
						Type:         "INTEGER",
						Nullable:     false,
						IsPrimaryKey: true,
						TypeMetadata: &database.TypeMetadata{
							Logical: "integer",
							Raw:     "INTEGER",
							Dialect: database.DialectSQLite,
						},
					},
					{
						Name:     "email",
						Type:     "TEXT",
						Nullable: false,
						TypeMetadata: &database.TypeMetadata{
							Logical: "text",
							Raw:     "TEXT",
							Dialect: database.DialectSQLite,
						},
					},
				},
			},
		},
	}

	// PostgreSQL schema with pg_catalog.int4 type (normalized to integer)
	postgresSchema := &Schema{
		Dialect: database.DialectPostgres,
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{
						Name:         "id",
						Type:         "integer",
						Nullable:     false,
						IsPrimaryKey: true,
						TypeMetadata: &database.TypeMetadata{
							Logical: "integer",
							Raw:     "pg_catalog.int4",
							Dialect: database.DialectPostgres,
						},
					},
					{
						Name:     "email",
						Type:     "text",
						Nullable: false,
						TypeMetadata: &database.TypeMetadata{
							Logical: "text",
							Raw:     "text",
							Dialect: database.DialectPostgres,
						},
					},
				},
			},
		},
	}

	sqliteHash, err := ComputeSchemaHash(sqliteSchema)
	if err != nil {
		t.Fatalf("failed to compute SQLite schema hash: %v", err)
	}

	postgresHash, err := ComputeSchemaHash(postgresSchema)
	if err != nil {
		t.Fatalf("failed to compute Postgres schema hash: %v", err)
	}

	if sqliteHash != postgresHash {
		t.Errorf("expected same hash for logically equivalent schemas\nSQLite:   %s\nPostgres: %s",
			sqliteHash, postgresHash)
	}
}

// TestComputeSchemaHash_DifferentTypes verifies that schemas with
// different logical types produce different hashes
func TestComputeSchemaHash_DifferentTypes(t *testing.T) {
	schema1 := &Schema{
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{
						Name:         "id",
						Type:         "integer",
						Nullable:     false,
						IsPrimaryKey: true,
					},
				},
			},
		},
	}

	schema2 := &Schema{
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{
						Name:         "id",
						Type:         "bigint",
						Nullable:     false,
						IsPrimaryKey: true,
					},
				},
			},
		},
	}

	hash1, err := ComputeSchemaHash(schema1)
	if err != nil {
		t.Fatalf("failed to compute schema1 hash: %v", err)
	}

	hash2, err := ComputeSchemaHash(schema2)
	if err != nil {
		t.Fatalf("failed to compute schema2 hash: %v", err)
	}

	if hash1 == hash2 {
		t.Errorf("expected different hashes for schemas with different types\nBoth: %s", hash1)
	}
}

// TestComputeSchemaHash_Deterministic verifies that the same schema
// produces the same hash consistently
func TestComputeSchemaHash_Deterministic(t *testing.T) {
	schema := &Schema{
		Tables: []Table{
			{
				Name: "products",
				Columns: []Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
					{Name: "name", Type: "text", Nullable: false},
					{Name: "price", Type: "numeric", Nullable: true},
				},
				Indexes: []Index{
					{Name: "idx_name", Columns: []string{"name"}, Unique: false},
				},
			},
		},
	}

	hash1, err := ComputeSchemaHash(schema)
	if err != nil {
		t.Fatalf("failed to compute hash (attempt 1): %v", err)
	}

	hash2, err := ComputeSchemaHash(schema)
	if err != nil {
		t.Fatalf("failed to compute hash (attempt 2): %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected deterministic hash\nAttempt 1: %s\nAttempt 2: %s", hash1, hash2)
	}
}

// TestComputeSchemaHash_NilSchema verifies that nil schema has a consistent hash
func TestComputeSchemaHash_NilSchema(t *testing.T) {
	hash1, err := ComputeSchemaHash(nil)
	if err != nil {
		t.Fatalf("failed to compute nil schema hash: %v", err)
	}

	hash2, err := ComputeSchemaHash(&Schema{Tables: []Table{}})
	if err != nil {
		t.Fatalf("failed to compute empty schema hash: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected nil and empty schema to have same hash\nNil:   %s\nEmpty: %s", hash1, hash2)
	}
}
