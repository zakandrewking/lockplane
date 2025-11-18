package config

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestResolveEnvironmentSQLiteFromDotenv(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dotenvPath := filepath.Join(tempDir, ".env.local")
	if err := os.WriteFile(dotenvPath, []byte("SQLITE_DB_PATH=schema/lockplane.db\nSQLITE_SHADOW_DB_PATH=schema/lockplane_shadow.db\n"), 0o600); err != nil {
		t.Fatalf("Failed to write dotenv file: %v", err)
	}

	config := &Config{
		DefaultEnvironment: "local",
		configDir:          tempDir,
		Environments: map[string]EnvironmentConfig{
			"local": {
				Description: "SQLite database",
			},
		},
	}

	env, err := ResolveEnvironment(config, "local")
	if err != nil {
		t.Fatalf("ResolveEnvironment returned error: %v", err)
	}

	if env.DatabaseURL != "schema/lockplane.db" {
		t.Fatalf("Expected SQLITE_DB_PATH value, got %q", env.DatabaseURL)
	}

	if env.ShadowDatabaseURL != "schema/lockplane_shadow.db" {
		t.Fatalf("Expected SQLITE_SHADOW_DB_PATH value, got %q", env.ShadowDatabaseURL)
	}
}

func TestResolveEnvironmentPostgresFromDotenv(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dotenvPath := filepath.Join(tempDir, ".env.prod")
	if err := os.WriteFile(dotenvPath, []byte("POSTGRES_URL=postgresql://user:pass@localhost:5432/db\nPOSTGRES_SHADOW_URL=postgresql://user:pass@localhost:5433/db_shadow\n"), 0o600); err != nil {
		t.Fatalf("Failed to write dotenv file: %v", err)
	}

	config := &Config{
		DefaultEnvironment: "prod",
		configDir:          tempDir,
		Environments: map[string]EnvironmentConfig{
			"prod": {
				Description: "PostgreSQL database",
			},
		},
	}

	env, err := ResolveEnvironment(config, "prod")
	if err != nil {
		t.Fatalf("ResolveEnvironment returned error: %v", err)
	}

	if env.DatabaseURL != "postgresql://user:pass@localhost:5432/db" {
		t.Fatalf("Expected POSTGRES_URL value, got %q", env.DatabaseURL)
	}

	if env.ShadowDatabaseURL != "postgresql://user:pass@localhost:5433/db_shadow" {
		t.Fatalf("Expected POSTGRES_SHADOW_URL value, got %q", env.ShadowDatabaseURL)
	}
}

func TestResolveEnvironmentLibSQLFromDotenv(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dotenvPath := filepath.Join(tempDir, ".env.turso")
	if err := os.WriteFile(dotenvPath, []byte("LIBSQL_URL=libsql://example.turso.io\nLIBSQL_AUTH_TOKEN=test-token\nLIBSQL_SHADOW_DB_PATH=./schema/turso_shadow.db\n"), 0o600); err != nil {
		t.Fatalf("Failed to write dotenv file: %v", err)
	}

	config := &Config{
		DefaultEnvironment: "turso",
		configDir:          tempDir,
		Environments: map[string]EnvironmentConfig{
			"turso": {
				Description: "libSQL/Turso database",
			},
		},
	}

	env, err := ResolveEnvironment(config, "turso")
	if err != nil {
		t.Fatalf("ResolveEnvironment returned error: %v", err)
	}

	expectedURL := "libsql://example.turso.io?authToken=test-token"
	if env.DatabaseURL != expectedURL {
		t.Fatalf("Expected LIBSQL_URL with auth token, got %q", env.DatabaseURL)
	}

	if env.ShadowDatabaseURL != "./schema/turso_shadow.db" {
		t.Fatalf("Expected LIBSQL_SHADOW_DB_PATH value, got %q", env.ShadowDatabaseURL)
	}
}

func TestResolveEnvironmentShadowSQLiteDBPathVariant(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dotenvPath := filepath.Join(tempDir, ".env.local")
	// Test the SHADOW_SQLITE_DB_PATH variant (user's actual configuration)
	if err := os.WriteFile(dotenvPath, []byte("SQLITE_DB_PATH=schema/lockplane.db\nSHADOW_SQLITE_DB_PATH=schema/lockplane_shadow.db\n"), 0o600); err != nil {
		t.Fatalf("Failed to write dotenv file: %v", err)
	}

	config := &Config{
		DefaultEnvironment: "local",
		configDir:          tempDir,
		Environments: map[string]EnvironmentConfig{
			"local": {
				Description: "SQLite database",
			},
		},
	}

	env, err := ResolveEnvironment(config, "local")
	if err != nil {
		t.Fatalf("ResolveEnvironment returned error: %v", err)
	}

	if env.DatabaseURL != "schema/lockplane.db" {
		t.Fatalf("Expected SQLITE_DB_PATH value, got %q", env.DatabaseURL)
	}

	if env.ShadowDatabaseURL != "schema/lockplane_shadow.db" {
		t.Fatalf("Expected SHADOW_SQLITE_DB_PATH value, got %q", env.ShadowDatabaseURL)
	}
}

func TestResolveEnvironmentShadowSchemaFallsBackToDatabase(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dotenvPath := filepath.Join(tempDir, ".env.local")
	data := `POSTGRES_URL=postgresql://user:pass@localhost:5432/db
SHADOW_SCHEMA=lockplane_shadow
`
	if err := os.WriteFile(dotenvPath, []byte(data), 0o600); err != nil {
		t.Fatalf("Failed to write dotenv file: %v", err)
	}

	config := &Config{
		DefaultEnvironment: "local",
		configDir:          tempDir,
		Environments: map[string]EnvironmentConfig{
			"local": {},
		},
	}

	env, err := ResolveEnvironment(config, "local")
	if err != nil {
		t.Fatalf("ResolveEnvironment returned error: %v", err)
	}

	if env.DatabaseURL != "postgresql://user:pass@localhost:5432/db" {
		t.Fatalf("expected POSTGRES_URL value, got %q", env.DatabaseURL)
	}
	if env.ShadowSchema != "lockplane_shadow" {
		t.Fatalf("expected SHADOW_SCHEMA to be set, got %q", env.ShadowSchema)
	}
	if env.ShadowDatabaseURL != env.DatabaseURL {
		t.Fatalf("expected shadow DB to reuse POSTGRES_URL, got %q", env.ShadowDatabaseURL)
	}
}

func TestResolveEnvironmentHonorsGlobalDialectAndSchemas(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	config := &Config{
		DefaultEnvironment: "local",
		configDir:          tempDir,
		Dialect:            "sqlite",
		Schemas:            []string{"public", "storage"},
		Environments: map[string]EnvironmentConfig{
			"local": {},
		},
	}

	env, err := ResolveEnvironment(config, "local")
	if err != nil {
		t.Fatalf("ResolveEnvironment returned error: %v", err)
	}

	if env.Dialect != "sqlite" {
		t.Fatalf("Expected dialect sqlite from global config, got %q", env.Dialect)
	}
	if !reflect.DeepEqual(env.Schemas, []string{"public", "storage"}) {
		t.Fatalf("Expected schemas %#v, got %#v", []string{"public", "storage"}, env.Schemas)
	}
}
