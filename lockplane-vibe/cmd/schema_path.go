package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

// detectDefaultSchemaDir returns the first schema directory found in priority order.
// It gives preference to Supabase's supabase/schema directory, then falls back
// to the traditional schema/ directory. The returned label is formatted for logs.
func detectDefaultSchemaDir() (path string, label string) {
	type candidate struct {
		path  string
		label string
	}
	candidates := make([]candidate, 0, 4)

	if custom := os.Getenv("LOCKPLANE_SCHEMA_DIR"); custom != "" {
		for _, part := range filepath.SplitList(custom) {
			if part == "" {
				continue
			}
			label := filepath.ToSlash(part)
			if !strings.HasSuffix(label, "/") {
				label += "/"
			}
			candidates = append(candidates, candidate{path: part, label: label})
		}
	}

	candidates = append(candidates,
		candidate{path: filepath.Join("supabase", "schema"), label: "supabase/schema/"},
		candidate{path: "schema", label: "schema/"},
	)

	for _, cand := range candidates {
		if info, err := os.Stat(cand.path); err == nil && info.IsDir() {
			return cand.path, cand.label
		}
	}
	return "", ""
}
