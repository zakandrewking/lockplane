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
	SchemaDir         string
	ConfigPath        string
	EnvFiles          []string
	SchemaDirCreated  bool
	ConfigCreated     bool
	ConfigUpdated     bool
	GitignoreUpdated  bool
	EnvExampleCreated bool
	EnvExampleUpdated bool
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	yes := fs.Bool("yes", false, "Skip the wizard and use provided flag values")

	// Environment configuration
	envName := fs.String("env-name", "local", "Environment name")
	description := fs.String("description", "Local development database", "Environment description")
	dbType := fs.String("db-type", "postgres", "Database type (postgres, sqlite, libsql)")
	schemaPath := fs.String("schema-path", ".", "Schema path relative to config directory")

	// PostgreSQL options
	host := fs.String("host", "localhost", "PostgreSQL host")
	port := fs.String("port", "5432", "PostgreSQL port")
	database := fs.String("database", "lockplane", "PostgreSQL database name")
	user := fs.String("user", "lockplane", "PostgreSQL user")
	password := fs.String("password", "lockplane", "PostgreSQL password")
	sslMode := fs.String("ssl-mode", "", "PostgreSQL SSL mode (disable, require, verify-ca, verify-full)")
	shadowDBPort := fs.String("shadow-db-port", "5433", "PostgreSQL shadow database port")

	// SQLite options
	filePath := fs.String("file-path", "schema/lockplane.db", "SQLite database file path")

	// libSQL options
	url := fs.String("url", "", "libSQL database URL")
	authToken := fs.String("auth-token", "", "libSQL auth token")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lockplane init [flags]\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Interactive wizard for setting up Lockplane in your project.\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "The wizard guides you through:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Database type selection (PostgreSQL, SQLite, libSQL/Turso)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Connection details with smart defaults\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Connection testing to verify credentials\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Multiple environment setup (local, staging, production)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Automatic shadow DB configuration (PostgreSQL)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Secure file generation (.env.* with 0600 permissions)\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nFeatures:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Detects existing configs and offers to add environments\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Auto-configures SSL (disabled=localhost, required=remote)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Tests connections before saving\n")
		_, _ = fmt.Fprintf(os.Stderr, "  • Updates .gitignore to protect credentials\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nFlags:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --yes              Skip wizard, use flag values (for CI/automation)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --env-name         Environment name (default: local)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --description      Environment description\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --db-type          Database type: postgres, sqlite, libsql (default: postgres)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --schema-path      Schema path relative to config (default: .)\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nPostgreSQL flags:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --host             Host (default: localhost)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --port             Port (default: 5432)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --database         Database name (default: lockplane)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --user             User (default: lockplane)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --password         Password (default: lockplane)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --ssl-mode         SSL mode (auto-detected if not specified)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --shadow-db-port   Shadow database port (default: 5433)\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nSQLite flags:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --file-path        Database file path (default: schema/lockplane.db)\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nlibSQL flags:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --url              Database URL\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --auth-token       Auth token\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nExamples:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  # Interactive wizard\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "  # Non-interactive with defaults\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init --yes\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "  # Non-interactive with custom PostgreSQL config\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init --yes \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --env-name production \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --description \"Production database\" \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --host db.example.com \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --port 5432 \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --database myapp \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --user myuser \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --password \"$DB_PASSWORD\" \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --ssl-mode require\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "  # Non-interactive with SQLite\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init --yes --db-type sqlite --file-path ./myapp.db\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "  # Non-interactive with libSQL/Turso\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init --yes --db-type libsql \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --env-name production \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --url \"libsql://mydb-myorg.turso.io\" \\\n")
		_, _ = fmt.Fprintf(os.Stderr, "    --auth-token \"$TURSO_TOKEN\"\n")
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

		// Build environment input from flags
		envInput := wizard.EnvironmentInput{
			Name:         *envName,
			Description:  *description,
			DatabaseType: *dbType,
			SchemaPath:   *schemaPath,
			Host:         *host,
			Port:         *port,
			Database:     *database,
			User:         *user,
			Password:     *password,
			SSLMode:      *sslMode,
			ShadowDBPort: *shadowDBPort,
			FilePath:     *filePath,
			URL:          *url,
			AuthToken:    *authToken,
		}

		// Validate database type
		if envInput.DatabaseType != "postgres" && envInput.DatabaseType != "sqlite" && envInput.DatabaseType != "libsql" {
			_, _ = fmt.Fprintf(os.Stderr, "Error: Invalid database type '%s'. Must be one of: postgres, sqlite, libsql\n", envInput.DatabaseType)
			os.Exit(1)
		}

		// Test connection before creating files
		var connStr string
		switch envInput.DatabaseType {
		case "postgres":
			connStr = wizard.BuildPostgresConnectionString(envInput)
		case "sqlite":
			connStr = wizard.BuildSQLiteConnectionString(envInput)
		case "libsql":
			connStr = wizard.BuildLibSQLConnectionString(envInput)
		}

		if err := wizard.TestConnection(connStr, envInput.DatabaseType); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: Failed to connect to database: %v\n", err)
			_, _ = fmt.Fprintf(os.Stderr, "Please check your connection parameters and try again.\n")
			os.Exit(1)
		}

		// Generate files
		result, err := wizard.GenerateFiles([]wizard.EnvironmentInput{envInput})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Report success
		reportBootstrapResult(os.Stdout, &bootstrapResult{
			SchemaDir:         result.SchemaDir,
			ConfigPath:        result.ConfigPath,
			EnvFiles:          result.EnvFiles,
			SchemaDirCreated:  result.SchemaDirCreated,
			ConfigCreated:     result.ConfigCreated,
			ConfigUpdated:     result.ConfigUpdated,
			GitignoreUpdated:  result.GitignoreUpdated,
			EnvExampleCreated: result.EnvExampleCreated,
			EnvExampleUpdated: result.EnvExampleUpdated,
		})
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

	if result.EnvExampleCreated {
		_, _ = fmt.Fprintf(out, "✓ Created .env.example\n")
	} else if result.EnvExampleUpdated {
		_, _ = fmt.Fprintf(out, "✓ Updated .env.example\n")
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
