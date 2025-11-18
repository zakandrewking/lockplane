package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lockplane/lockplane/database"
	"github.com/xeipuuv/gojsonschema"
)

// TestLoadJSONSchema_RejectsExtraFields tests that schemas with extra fields are rejected
func TestLoadJSONSchema_RejectsExtraFields(t *testing.T) {
	tests := []struct {
		name        string
		jsonContent string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid schema",
			jsonContent: `{
				"tables": [{
					"name": "users",
					"columns": [{
						"name": "id",
						"type": "bigint",
						"nullable": false,
						"is_primary_key": true
					}]
				}]
			}`,
			wantErr: false,
		},
		{
			name: "extra field at root level",
			jsonContent: `{
				"tables": [{
					"name": "users",
					"columns": [{
						"name": "id",
						"type": "bigint",
						"nullable": false,
						"is_primary_key": true
					}]
				}],
				"extra_metadata": "should be rejected"
			}`,
			wantErr:     true,
			errContains: "Additional property extra_metadata is not allowed",
		},
		{
			name: "extra field in table",
			jsonContent: `{
				"tables": [{
					"name": "users",
					"columns": [{
						"name": "id",
						"type": "bigint",
						"nullable": false,
						"is_primary_key": true
					}],
					"custom_field": "not allowed"
				}]
			}`,
			wantErr:     true,
			errContains: "Additional property custom_field is not allowed",
		},
		{
			name: "extra field in column",
			jsonContent: `{
				"tables": [{
					"name": "users",
					"columns": [{
						"name": "id",
						"type": "bigint",
						"nullable": false,
						"is_primary_key": true,
						"my_annotation": "extra"
					}]
				}]
			}`,
			wantErr:     true,
			errContains: "Additional property my_annotation is not allowed",
		},
		{
			name: "extra field in index",
			jsonContent: `{
				"tables": [{
					"name": "users",
					"columns": [{
						"name": "id",
						"type": "bigint",
						"nullable": false,
						"is_primary_key": true
					}],
					"indexes": [{
						"name": "idx_users_id",
						"columns": ["id"],
						"unique": true,
						"extra_index_field": "not allowed"
					}]
				}]
			}`,
			wantErr:     true,
			errContains: "Additional property extra_index_field is not allowed",
		},
		{
			name: "extra field in foreign key",
			jsonContent: `{
				"tables": [{
					"name": "posts",
					"columns": [{
						"name": "id",
						"type": "bigint",
						"nullable": false,
						"is_primary_key": true
					}, {
						"name": "user_id",
						"type": "bigint",
						"nullable": false,
						"is_primary_key": false
					}],
					"foreign_keys": [{
						"name": "fk_posts_user_id",
						"columns": ["user_id"],
						"referenced_table": "users",
						"referenced_columns": ["id"],
						"custom_fk_field": "extra"
					}]
				}]
			}`,
			wantErr:     true,
			errContains: "Additional property custom_fk_field is not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			schemaPath := filepath.Join(tmpDir, "schema.json")

			if err := os.WriteFile(schemaPath, []byte(tt.jsonContent), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Try to load the schema
			_, err := LoadJSONSchema(schemaPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errContains != "" {
					// Accept either JSON Schema error message or Go unmarshaler error message
					hasJSONSchemaMsg := strings.Contains(err.Error(), tt.errContains)
					hasGoUnmarshalMsg := strings.Contains(err.Error(), "unknown field")
					if !hasJSONSchemaMsg && !hasGoUnmarshalMsg {
						t.Errorf("Error should contain %q or 'unknown field', got: %v", tt.errContains, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestLoadJSONSchema_DisallowUnknownFields tests that Go's DisallowUnknownFields catches extra fields
// even if JSON Schema validation is skipped (e.g., schema file not found)
func TestLoadJSONSchema_DisallowUnknownFields(t *testing.T) {
	// This test ensures that even if JSON Schema validation fails/skips,
	// the Go unmarshaler still rejects extra fields
	jsonContent := `{
		"tables": [{
			"name": "users",
			"columns": [{
				"name": "id",
				"type": "bigint",
				"nullable": false,
				"is_primary_key": true,
				"golang_will_catch_this": "extra field"
			}]
		}]
	}`

	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")

	if err := os.WriteFile(schemaPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadJSONSchema(schemaPath)
	if err == nil {
		t.Error("Expected error from DisallowUnknownFields but got none")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("Error should mention 'unknown field', got: %v", err)
	}
}

// TestSchemaJSON_ConsistencyWithGoTypes verifies that JSON schema matches Go struct definitions
// This prevents bugs where we add/change fields in Go but forget to update JSON schema
func TestSchemaJSON_ConsistencyWithGoTypes(t *testing.T) {
	defaultValue := "nextval('users_id_seq'::regclass)"
	onDelete := "CASCADE"

	// Create fully-populated Schema with ALL fields that exist in Go struct
	schema := &database.Schema{
		Dialect: database.DialectPostgres,
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
						TypeMetadata: &database.TypeMetadata{
							Logical: "integer",
							Raw:     "pg_catalog.int8",
							Dialect: database.DialectPostgres,
						},
						DefaultMetadata: &database.DefaultMetadata{
							Raw:     "nextval('users_id_seq'::regclass)",
							Dialect: database.DialectPostgres,
							Kind:    "sequence",
						},
					},
					{
						Name:     "email",
						Type:     "text",
						Nullable: false,
					},
				},
				Indexes: []database.Index{
					{
						Name:    "idx_users_email",
						Columns: []string{"email"},
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
						OnUpdate:          nil,
					},
				},
			},
			{
				Name: "posts",
				Columns: []database.Column{
					{
						Name:         "id",
						Type:         "bigint",
						Nullable:     false,
						IsPrimaryKey: true,
					},
				},
				Indexes:     []database.Index{},      // Empty slice, not nil
				ForeignKeys: []database.ForeignKey{}, // Empty slice, not nil
			},
		},
	}

	// Step 1: Marshal to JSON
	jsonData, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal schema to JSON: %v", err)
	}

	t.Logf("Generated JSON:\n%s", string(jsonData))

	// Step 2: Validate against JSON Schema
	// This will FAIL if JSON schema is missing fields or has wrong types
	// Find schema file relative to project root (../../schema-json/schema.json from internal/schema/)
	schemaPath := filepath.Join("..", "..", "schema-json", "schema.json")
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
	documentLoader := gojsonschema.NewBytesLoader(jsonData)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		t.Fatalf("JSON Schema validation failed to run: %v", err)
	}

	if !result.Valid() {
		t.Errorf("JSON Schema validation failed! This means schema.json is missing fields or has incorrect types:")
		for _, desc := range result.Errors() {
			t.Errorf("  - %s", desc)
		}
		t.FailNow()
	}

	// Step 3: Round-trip test - unmarshal and verify all fields preserved
	var decoded database.Schema
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON back to Schema: %v", err)
	}

	// Verify critical fields are preserved
	if decoded.Dialect != schema.Dialect {
		t.Errorf("Dialect not preserved: expected %v, got %v", schema.Dialect, decoded.Dialect)
	}

	if len(decoded.Tables) != len(schema.Tables) {
		t.Errorf("Table count mismatch: expected %d, got %d", len(schema.Tables), len(decoded.Tables))
	}

	// Check first table
	if len(decoded.Tables) > 0 {
		origTable := schema.Tables[0]
		decodedTable := decoded.Tables[0]

		if decodedTable.Name != origTable.Name {
			t.Errorf("Table name not preserved: expected %s, got %s", origTable.Name, decodedTable.Name)
		}

		// Check first column with all metadata
		if len(decodedTable.Columns) > 0 && len(origTable.Columns) > 0 {
			origCol := origTable.Columns[0]
			decodedCol := decodedTable.Columns[0]

			if decodedCol.Name != origCol.Name {
				t.Errorf("Column name not preserved: expected %s, got %s", origCol.Name, decodedCol.Name)
			}

			if decodedCol.TypeMetadata == nil {
				t.Error("TypeMetadata not preserved - this field is missing from JSON schema or not round-tripping correctly")
			} else {
				if decodedCol.TypeMetadata.Logical != origCol.TypeMetadata.Logical {
					t.Errorf("TypeMetadata.Logical not preserved: expected %s, got %s",
						origCol.TypeMetadata.Logical, decodedCol.TypeMetadata.Logical)
				}
				if decodedCol.TypeMetadata.Raw != origCol.TypeMetadata.Raw {
					t.Errorf("TypeMetadata.Raw not preserved: expected %s, got %s",
						origCol.TypeMetadata.Raw, decodedCol.TypeMetadata.Raw)
				}
			}

			if decodedCol.DefaultMetadata == nil {
				t.Error("DefaultMetadata not preserved - this field is missing from JSON schema or not round-tripping correctly")
			} else {
				if decodedCol.DefaultMetadata.Raw != origCol.DefaultMetadata.Raw {
					t.Errorf("DefaultMetadata.Raw not preserved: expected %s, got %s",
						origCol.DefaultMetadata.Raw, decodedCol.DefaultMetadata.Raw)
				}
				if decodedCol.DefaultMetadata.Kind != origCol.DefaultMetadata.Kind {
					t.Errorf("DefaultMetadata.Kind not preserved: expected %s, got %s",
						origCol.DefaultMetadata.Kind, decodedCol.DefaultMetadata.Kind)
				}
			}
		}

		// Check indexes
		if len(decodedTable.Indexes) != len(origTable.Indexes) {
			t.Errorf("Index count mismatch: expected %d, got %d",
				len(origTable.Indexes), len(decodedTable.Indexes))
		}

		// Check foreign keys
		if len(decodedTable.ForeignKeys) != len(origTable.ForeignKeys) {
			t.Errorf("Foreign key count mismatch: expected %d, got %d",
				len(origTable.ForeignKeys), len(decodedTable.ForeignKeys))
		}
	}

	t.Log("✅ JSON schema is consistent with Go types - all fields validated and round-tripped successfully")
}

// TestDialectPrecedence verifies that dialect resolution follows the correct precedence order:
// 1. Inline file comment (-- dialect: sqlite)
// 2. Config (opts.Dialect)
// 3. Default (postgres)
func TestDialectPrecedence(t *testing.T) {
	tests := []struct {
		name            string
		sqlContent      string
		configDialect   database.Dialect
		expectedDialect database.Dialect
		expectWarning   bool
	}{
		{
			name:            "inline comment takes precedence over config",
			sqlContent:      "-- dialect: sqlite\nCREATE TABLE users (id INTEGER PRIMARY KEY);",
			configDialect:   database.DialectPostgres,
			expectedDialect: database.DialectSQLite,
			expectWarning:   true, // Should warn about conflict
		},
		{
			name:            "config used when no inline comment",
			sqlContent:      "CREATE TABLE users (id BIGSERIAL PRIMARY KEY);",
			configDialect:   database.DialectSQLite,
			expectedDialect: database.DialectSQLite,
			expectWarning:   false,
		},
		{
			name:            "default to postgres when neither specified",
			sqlContent:      "CREATE TABLE users (id BIGSERIAL PRIMARY KEY);",
			configDialect:   database.DialectUnknown,
			expectedDialect: database.DialectPostgres,
			expectWarning:   false,
		},
		{
			name:            "inline comment without conflict",
			sqlContent:      "-- dialect: postgres\nCREATE TABLE users (id BIGSERIAL PRIMARY KEY);",
			configDialect:   database.DialectUnknown,
			expectedDialect: database.DialectPostgres,
			expectWarning:   false,
		},
		{
			name:            "inline and config match - no warning",
			sqlContent:      "-- dialect: postgres\nCREATE TABLE users (id BIGSERIAL PRIMARY KEY);",
			configDialect:   database.DialectPostgres,
			expectedDialect: database.DialectPostgres,
			expectWarning:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &SchemaLoadOptions{
				Dialect: tt.configDialect,
			}

			schema, err := LoadSQLSchemaFromBytes([]byte(tt.sqlContent), opts)
			if err != nil {
				t.Fatalf("LoadSQLSchemaFromBytes failed: %v", err)
			}

			if schema.Dialect != tt.expectedDialect {
				t.Errorf("Expected dialect %s, got %s", tt.expectedDialect, schema.Dialect)
			}

			// Note: We can't easily test for warning output in this test,
			// but we verify the precedence logic works correctly
		})
	}
}
