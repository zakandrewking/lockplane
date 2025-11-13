package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
