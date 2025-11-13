package schema

import (
	"testing"

	"github.com/lockplane/lockplane/database"
)

// TestComputeSchemaHash_LogicalTypeEquivalence verifies that schemas with
// different raw types but same logical types produce the same hash
func TestComputeSchemaHash_LogicalTypeEquivalence(t *testing.T) {
	// SQLite schema with INTEGER type
	sqliteSchema := &database.Schema{
		Dialect: database.DialectSQLite,
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
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
	postgresSchema := &database.Schema{
		Dialect: database.DialectPostgres,
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
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
	schema1 := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
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

	schema2 := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
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
	schema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "products",
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
					{Name: "name", Type: "text", Nullable: false},
					{Name: "price", Type: "numeric", Nullable: true},
				},
				Indexes: []database.Index{
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

	hash2, err := ComputeSchemaHash(&database.Schema{Tables: []database.Table{}})
	if err != nil {
		t.Fatalf("failed to compute empty schema hash: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected nil and empty schema to have same hash\nNil:   %s\nEmpty: %s", hash1, hash2)
	}
}

// TestComputeSchemaHash_AllFieldsAffectHash verifies that every schema field
// is included in hash computation. This prevents bugs where adding a new field
// to the schema doesn't affect the hash, leading to false "no changes" diffs.
func TestComputeSchemaHash_AllFieldsAffectHash(t *testing.T) {
	defaultValue := "42"
	onDelete := "CASCADE"
	onUpdate := "RESTRICT"

	baseSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{
						Name:         "id",
						Type:         "bigint",
						Nullable:     false,
						IsPrimaryKey: true,
						Default:      &defaultValue,
					},
				},
				Indexes: []database.Index{
					{
						Name:    "idx_users_id",
						Columns: []string{"id"},
						Unique:  true,
					},
				},
				ForeignKeys: []database.ForeignKey{
					{
						Name:              "fk_users_org",
						Columns:           []string{"org_id"},
						ReferencedTable:   "orgs",
						ReferencedColumns: []string{"id"},
						OnDelete:          &onDelete,
						OnUpdate:          &onUpdate,
					},
				},
			},
		},
	}

	baseHash, err := ComputeSchemaHash(baseSchema)
	if err != nil {
		t.Fatalf("failed to compute base hash: %v", err)
	}

	tests := []struct {
		name   string
		modify func(*database.Schema)
	}{
		{
			name: "table name change",
			modify: func(s *database.Schema) {
				s.Tables[0].Name = "users_renamed"
			},
		},
		{
			name: "column name change",
			modify: func(s *database.Schema) {
				s.Tables[0].Columns[0].Name = "user_id"
			},
		},
		{
			name: "column type change",
			modify: func(s *database.Schema) {
				s.Tables[0].Columns[0].Type = "integer"
			},
		},
		{
			name: "column nullable change",
			modify: func(s *database.Schema) {
				s.Tables[0].Columns[0].Nullable = true
			},
		},
		{
			name: "column primary key change",
			modify: func(s *database.Schema) {
				s.Tables[0].Columns[0].IsPrimaryKey = false
			},
		},
		{
			name: "column default change",
			modify: func(s *database.Schema) {
				newDefault := "100"
				s.Tables[0].Columns[0].Default = &newDefault
			},
		},
		{
			name: "column default removed",
			modify: func(s *database.Schema) {
				s.Tables[0].Columns[0].Default = nil
			},
		},
		{
			name: "index name change",
			modify: func(s *database.Schema) {
				s.Tables[0].Indexes[0].Name = "idx_users_id_new"
			},
		},
		{
			name: "index columns change",
			modify: func(s *database.Schema) {
				s.Tables[0].Indexes[0].Columns = []string{"id", "name"}
			},
		},
		{
			name: "index unique change",
			modify: func(s *database.Schema) {
				s.Tables[0].Indexes[0].Unique = false
			},
		},
		{
			name: "foreign key name change",
			modify: func(s *database.Schema) {
				s.Tables[0].ForeignKeys[0].Name = "fk_users_org_new"
			},
		},
		{
			name: "foreign key columns change",
			modify: func(s *database.Schema) {
				s.Tables[0].ForeignKeys[0].Columns = []string{"organization_id"}
			},
		},
		{
			name: "foreign key referenced table change",
			modify: func(s *database.Schema) {
				s.Tables[0].ForeignKeys[0].ReferencedTable = "organizations"
			},
		},
		{
			name: "foreign key referenced columns change",
			modify: func(s *database.Schema) {
				s.Tables[0].ForeignKeys[0].ReferencedColumns = []string{"org_id"}
			},
		},
		{
			name: "foreign key on_delete change",
			modify: func(s *database.Schema) {
				newOnDelete := "SET NULL"
				s.Tables[0].ForeignKeys[0].OnDelete = &newOnDelete
			},
		},
		{
			name: "foreign key on_update change",
			modify: func(s *database.Schema) {
				newOnUpdate := "CASCADE"
				s.Tables[0].ForeignKeys[0].OnUpdate = &newOnUpdate
			},
		},
		{
			name: "foreign key on_delete removed",
			modify: func(s *database.Schema) {
				s.Tables[0].ForeignKeys[0].OnDelete = nil
			},
		},
		{
			name: "foreign key on_update removed",
			modify: func(s *database.Schema) {
				s.Tables[0].ForeignKeys[0].OnUpdate = nil
			},
		},
		{
			name: "add table",
			modify: func(s *database.Schema) {
				s.Tables = append(s.Tables, database.Table{
					Name: "posts",
					Columns: []database.Column{
						{Name: "id", Type: "bigint", IsPrimaryKey: true},
					},
				})
			},
		},
		{
			name: "add column",
			modify: func(s *database.Schema) {
				s.Tables[0].Columns = append(s.Tables[0].Columns, database.Column{
					Name: "email",
					Type: "text",
				})
			},
		},
		{
			name: "add index",
			modify: func(s *database.Schema) {
				s.Tables[0].Indexes = append(s.Tables[0].Indexes, database.Index{
					Name:    "idx_users_email",
					Columns: []string{"email"},
					Unique:  false,
				})
			},
		},
		{
			name: "add foreign key",
			modify: func(s *database.Schema) {
				s.Tables[0].ForeignKeys = append(s.Tables[0].ForeignKeys, database.ForeignKey{
					Name:              "fk_users_dept",
					Columns:           []string{"dept_id"},
					ReferencedTable:   "departments",
					ReferencedColumns: []string{"id"},
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a deep copy of the base schema
			modifiedSchema := &database.Schema{
				Tables: make([]database.Table, len(baseSchema.Tables)),
			}
			for i, table := range baseSchema.Tables {
				modifiedSchema.Tables[i] = database.Table{
					Name:        table.Name,
					Columns:     make([]database.Column, len(table.Columns)),
					Indexes:     make([]database.Index, len(table.Indexes)),
					ForeignKeys: make([]database.ForeignKey, len(table.ForeignKeys)),
				}
				copy(modifiedSchema.Tables[i].Columns, table.Columns)
				copy(modifiedSchema.Tables[i].Indexes, table.Indexes)
				copy(modifiedSchema.Tables[i].ForeignKeys, table.ForeignKeys)
			}

			// Apply the modification
			tt.modify(modifiedSchema)

			// Compute hash
			modifiedHash, err := ComputeSchemaHash(modifiedSchema)
			if err != nil {
				t.Fatalf("failed to compute modified hash: %v", err)
			}

			// Hash must be different
			if modifiedHash == baseHash {
				t.Errorf("modification %q did not affect hash!\nBase:     %s\nModified: %s",
					tt.name, baseHash, modifiedHash)
			}
		})
	}
}
