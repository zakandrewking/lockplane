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

	// No directories present
	if path, label := detectDefaultSchemaDir(); path != "" || label != "" {
		t.Fatalf("expected no schema detection, got %q (%s)", path, label)
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
