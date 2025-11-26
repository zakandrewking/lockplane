package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectDefaultSchemaDir(t *testing.T) {
	tmpDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	if err := os.Setenv("LOCKPLANE_SCHEMA_DIR", ""); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("LOCKPLANE_SCHEMA_DIR", "")
	})

	// No directories present
	if path, label := detectDefaultSchemaDir(); path != "" || label != "" {
		t.Fatalf("expected no schema detection, got %q (%s)", path, label)
	}

	// Configure custom env dir and ensure it's honored
	if err := os.Setenv("LOCKPLANE_SCHEMA_DIR", filepath.Join("custom", "schema")); err != nil {
		t.Fatalf("setenv custom: %v", err)
	}
	if err := os.MkdirAll(filepath.Join("custom", "schema"), 0o755); err != nil {
		t.Fatalf("mkdir custom: %v", err)
	}
	if path, label := detectDefaultSchemaDir(); path != filepath.Join("custom", "schema") || label != filepath.ToSlash(filepath.Join("custom", "schema"))+"/" {
		t.Fatalf("expected custom schema, got %q (%s)", path, label)
	}
	// Clear env to test defaults
	if err := os.Setenv("LOCKPLANE_SCHEMA_DIR", ""); err != nil {
		t.Fatalf("reset env: %v", err)
	}

	// Create schema/ and ensure it's selected when supabase/schema missing
	if err := os.MkdirAll("schema", 0o755); err != nil {
		t.Fatalf("mkdir schema: %v", err)
	}
	if path, label := detectDefaultSchemaDir(); path != "schema" || label != "schema/" {
		t.Fatalf("expected schema/, got %q (%s)", path, label)
	}

	// Create supabase/schema and expect it to take precedence
	supaPath := filepath.Join("supabase", "schema")
	if err := os.MkdirAll(supaPath, 0o755); err != nil {
		t.Fatalf("mkdir supabase schema: %v", err)
	}
	if path, label := detectDefaultSchemaDir(); path != supaPath || label != "supabase/schema/" {
		t.Fatalf("expected supabase/schema, got %q (%s)", path, label)
	}
}
