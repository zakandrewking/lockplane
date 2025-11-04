package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEnvironmentDefaults(t *testing.T) {
	t.Parallel()

	env, err := ResolveEnvironment(&Config{}, "")
	if err != nil {
		t.Fatalf("ResolveEnvironment returned error: %v", err)
	}

	if env.Name != defaultEnvironmentName {
		t.Fatalf("Expected default environment name %q, got %q", defaultEnvironmentName, env.Name)
	}

	if env.DatabaseURL != defaultDatabaseURL {
		t.Fatalf("Expected default database URL %q, got %q", defaultDatabaseURL, env.DatabaseURL)
	}

	if env.ShadowDatabaseURL != defaultShadowDatabaseURL {
		t.Fatalf("Expected default shadow URL %q, got %q", defaultShadowDatabaseURL, env.ShadowDatabaseURL)
	}
}

func TestResolveEnvironmentFromDotenv(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dotenvPath := filepath.Join(tempDir, ".env.staging")
	if err := os.WriteFile(dotenvPath, []byte("DATABASE_URL=postgres://staging\nSHADOW_DATABASE_URL=postgres://staging-shadow\nSCHEMA_PATH=schemas/staging\n"), 0o600); err != nil {
		t.Fatalf("Failed to write dotenv file: %v", err)
	}

	config := &Config{
		DefaultEnvironment: "staging",
		configDir:          tempDir,
		Environments: map[string]EnvironmentConfig{
			"staging": {},
		},
	}

	env, err := ResolveEnvironment(config, "staging")
	if err != nil {
		t.Fatalf("ResolveEnvironment returned error: %v", err)
	}

	if env.DatabaseURL != "postgres://staging" {
		t.Fatalf("Expected dotenv database URL, got %q", env.DatabaseURL)
	}

	if env.ShadowDatabaseURL != "postgres://staging-shadow" {
		t.Fatalf("Expected dotenv shadow URL, got %q", env.ShadowDatabaseURL)
	}

	expectedSchema := filepath.Join(tempDir, "schemas/staging")
	if env.SchemaPath != expectedSchema {
		t.Fatalf("Expected schema path %q, got %q", expectedSchema, env.SchemaPath)
	}
}

func TestResolveEnvironmentMissingDefinition(t *testing.T) {
	t.Parallel()

	config := &Config{
		Environments: map[string]EnvironmentConfig{
			"local": {
				DatabaseURL: "postgres://local",
			},
		},
		configDir: t.TempDir(),
	}

	if _, err := ResolveEnvironment(config, "production"); err == nil {
		t.Fatal("Expected error resolving undefined environment, got nil")
	}
}
