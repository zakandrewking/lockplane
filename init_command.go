package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lockplane/lockplane/internal/wizard"
)

const (
	defaultSchemaDir         = "schema"
	lockplaneConfigFilename  = "lockplane.toml"
	defaultLockplaneTomlBody = `default_environment = "local"

[environments.local]
description = "Local development database"
schema_path = "."
database_url = "postgresql://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
shadow_database_url = "postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable"
`
)

type bootstrapResult struct {
	SchemaDir        string
	ConfigPath       string
	EnvFiles         []string
	SchemaDirCreated bool
	ConfigCreated    bool
	ConfigUpdated    bool
	GitignoreUpdated bool
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	yes := fs.Bool("yes", false, "Skip the wizard and accept default values")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lockplane init [--yes]\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Launch the interactive Lockplane project wizard. The wizard bootstraps\n")
		_, _ = fmt.Fprintf(os.Stderr, "the schema/ directory and creates schema/lockplane.toml.\n")
		_, _ = fmt.Fprintf(os.Stderr, "Use --yes to accept defaults without prompts.\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nExamples:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init --yes\n")
	}

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	if *yes {
		existingPath, err := checkExistingConfig()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error checking for existing config: %v\n", err)
			os.Exit(1)
		}
		if existingPath != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Config already exists at /%s. ", *existingPath)
			_, _ = fmt.Fprintf(os.Stderr, "To use defaults, first delete the existing config file, and then run `lockplane init --yes` again.\n")
			os.Exit(1)
		}
		result, err := bootstrapSchemaDirectory()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		reportBootstrapResult(os.Stdout, result)
		return
	}

	if err := startInitWizard(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func ensureSchemaDir(path string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultSchemaDir
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, nil
		}
		return false, fmt.Errorf("%s exists but is not a directory", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, os.MkdirAll(path, 0o755)
}

func bootstrapSchemaDirectory() (*bootstrapResult, error) {
	dirCreated, err := ensureSchemaDir(defaultSchemaDir)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(defaultSchemaDir, lockplaneConfigFilename)
	if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
		return nil, fmt.Errorf("/%s already exists. Edit the existing file or delete it if you want to re-initialize", configPath)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err := os.WriteFile(configPath, []byte(defaultLockplaneTomlBody), 0o644); err != nil {
		return nil, err
	}

	return &bootstrapResult{
		SchemaDir:        defaultSchemaDir,
		ConfigPath:       configPath,
		SchemaDirCreated: dirCreated,
		ConfigCreated:    true,
	}, nil
}

func reportBootstrapResult(out *os.File, result *bootstrapResult) {
	if result == nil {
		return
	}

	if result.SchemaDirCreated {
		_, _ = fmt.Fprintf(out, "✓ Created %s/\n", filepath.ToSlash(result.SchemaDir))
	} else {
		_, _ = fmt.Fprintf(out, "• Using existing %s/\n", filepath.ToSlash(result.SchemaDir))
	}

	if result.ConfigCreated {
		_, _ = fmt.Fprintf(out, "✓ Wrote %s\n", filepath.ToSlash(result.ConfigPath))
	} else if result.ConfigUpdated {
		_, _ = fmt.Fprintf(out, "✓ Updated %s\n", filepath.ToSlash(result.ConfigPath))
	}

	for _, envFile := range result.EnvFiles {
		_, _ = fmt.Fprintf(out, "✓ Wrote %s\n", envFile)
	}

	if result.GitignoreUpdated {
		_, _ = fmt.Fprintf(out, "✓ Updated .gitignore\n")
	}

	_, _ = fmt.Fprintf(out, "\nNext steps:\n")
	_, _ = fmt.Fprintf(out, "  1. Run: lockplane introspect\n")
	_, _ = fmt.Fprintf(out, "  2. Review: %s\n", filepath.ToSlash(result.ConfigPath))
}

func startInitWizard() error {
	return wizard.Run()
}

func checkExistingConfig() (*string, error) {
	legacyPath := lockplaneConfigFilename
	if _, err := os.Stat(legacyPath); err == nil {
		return &legacyPath, nil
	}
	configPath := filepath.Join(defaultSchemaDir, lockplaneConfigFilename)
	if _, err := os.Stat(configPath); err == nil {
		return &configPath, nil
	}
	return nil, nil
}
