package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateJSONSchema_Valid(t *testing.T) {
	path := filepath.Join("examples", "schemas-json", "simple.json")
	if err := ValidateJSONSchema(path); err != nil {
		t.Fatalf("Expected schema %s to be valid, got error: %v", path, err)
	}
}

func TestValidateJSONSchema_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte(`{"tablesz": []}`), 0o600); err != nil {
		t.Fatalf("Failed to write invalid schema file: %v", err)
	}

	if err := ValidateJSONSchema(invalidPath); err == nil {
		t.Fatalf("Expected schema %s to be invalid", invalidPath)
	}
}
