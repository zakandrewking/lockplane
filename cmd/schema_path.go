package cmd

import (
	"os"
	"path/filepath"
)

// detectDefaultSchemaDir returns the first schema directory found in priority order.
// It gives preference to Supabase's supabase/schema directory, then falls back
// to the traditional schema/ directory. The returned label is formatted for logs.
func detectDefaultSchemaDir() (path string, label string) {
	candidates := []struct {
		path  string
		label string
	}{
		{filepath.Join("supabase", "schema"), "supabase/schema/"},
		{"schema", "schema/"},
	}

	for _, cand := range candidates {
		if info, err := os.Stat(cand.path); err == nil && info.IsDir() {
			return cand.path, cand.label
		}
	}
	return "", ""
}
