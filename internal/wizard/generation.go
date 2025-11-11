package wizard

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	_ "modernc.org/sqlite"
)

// GenerateFiles creates the lockplane.toml and .env files
func GenerateFiles(environments []EnvironmentInput) (*InitResult, error) {
	result := &InitResult{
		EnvFiles: []string{},
	}

	// Create schema directory if it doesn't exist
	schemaDir := "schema"
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create schema directory: %w", err)
	}
	result.SchemaDir = schemaDir
	result.SchemaDirCreated = true

	// Generate or update lockplane.toml in current directory
	configPath := "lockplane.toml"
	fileExists := false
	if _, err := os.Stat(configPath); err == nil {
		fileExists = true
	}

	if err := generateLockplaneTOML(configPath, environments); err != nil {
		return nil, fmt.Errorf("failed to generate lockplane.toml: %w", err)
	}
	result.ConfigPath = configPath
	if fileExists {
		result.ConfigUpdated = true
	} else {
		result.ConfigCreated = true
	}

	// Generate .env files
	for _, env := range environments {
		envFilePath := fmt.Sprintf(".env.%s", env.Name)
		if err := generateEnvFile(envFilePath, env); err != nil {
			return nil, fmt.Errorf("failed to generate %s: %w", envFilePath, err)
		}
		result.EnvFiles = append(result.EnvFiles, envFilePath)
	}

	// Create or update .env.example
	examplePath := ".env.example"
	exampleExists := false
	if _, err := os.Stat(examplePath); err == nil {
		exampleExists = true
	}
	if err := createOrUpdateEnvExample(environments); err != nil {
		return nil, fmt.Errorf("failed to create/update .env.example: %w", err)
	}
	if exampleExists {
		result.EnvExampleUpdated = true
	} else {
		result.EnvExampleCreated = true
	}

	// Update .gitignore
	if err := updateGitignore(); err != nil {
		return nil, fmt.Errorf("failed to update .gitignore: %w", err)
	}
	result.GitignoreUpdated = true

	// Create SQLite database files if needed
	for _, env := range environments {
		if env.DatabaseType == "sqlite" {
			dbPath := env.FilePath
			shadowPath := ""

			// Extract shadow DB path if it's configured
			if strings.Contains(dbPath, ".db") {
				// Generate shadow DB path (e.g., lockplane.db -> lockplane_shadow.db)
				ext := filepath.Ext(dbPath)
				base := strings.TrimSuffix(dbPath, ext)
				shadowPath = base + "_shadow" + ext
			}

			// Create main database
			if err := createSQLiteDatabaseFile(dbPath); err != nil {
				return nil, fmt.Errorf("failed to create SQLite database %s: %w", dbPath, err)
			}

			// Create shadow database
			if shadowPath != "" {
				if err := createSQLiteDatabaseFile(shadowPath); err != nil {
					return nil, fmt.Errorf("failed to create SQLite shadow database %s: %w", shadowPath, err)
				}
			}
		}
	}

	return result, nil
}

// createSQLiteDatabaseFile creates an empty SQLite database file
func createSQLiteDatabaseFile(filePath string) error {
	// Skip if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return nil
	}
	
	// Create parent directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	// Create the database
	db, err := sql.Open("sqlite", filePath)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer func() { _ = db.Close() }()
	
	// Initialize the database by creating a minimal table
	// SQLite won't create the file until we actually write something
	// We create and immediately drop a table to ensure the file is created
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS _lockplane_init (id INTEGER PRIMARY KEY); DROP TABLE IF EXISTS _lockplane_init;")
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	
	return nil
}

func generateLockplaneTOML(path string, newEnvironments []EnvironmentInput) error {
	// Load existing config if it exists
	existingEnvs := make(map[string]tomlEnvironment)
	var defaultEnv string

	if data, err := os.ReadFile(path); err == nil {
		// Parse existing config
		var existingConfig struct {
			DefaultEnvironment string                     `toml:"default_environment"`
			Environments       map[string]tomlEnvironment `toml:"environments"`
		}
		if err := toml.Unmarshal(data, &existingConfig); err == nil {
			existingEnvs = existingConfig.Environments
			defaultEnv = existingConfig.DefaultEnvironment
		}
	}

	// Merge new environments (new ones override existing with same name)
	for _, env := range newEnvironments {
		description := env.Description
		if description == "" {
			switch env.DatabaseType {
			case "postgres":
				description = "PostgreSQL database"
			case "sqlite":
				description = "SQLite database"
			case "libsql":
				description = "libSQL/Turso database"
			}
		}

		existingEnvs[env.Name] = tomlEnvironment{
			Description: description,
			Comment:     fmt.Sprintf("Connection: .env.%s", env.Name),
		}
	}

	// Set default environment if not already set
	if defaultEnv == "" && len(newEnvironments) > 0 {
		defaultEnv = newEnvironments[0].Name
	}

	// Build the TOML file
	var b strings.Builder

	b.WriteString("# Lockplane Configuration\n")
	b.WriteString("# Generated by: lockplane init\n")
	b.WriteString("#\n")
	b.WriteString("# Config location: Project root (lockplane.toml)\n")
	b.WriteString("# Credentials: Stored in .env.* files (never in this file)\n\n")

	if defaultEnv != "" {
		b.WriteString(fmt.Sprintf("default_environment = \"%s\"\n\n", defaultEnv))
	}

	// Write all environment sections
	for envName, env := range existingEnvs {
		b.WriteString(fmt.Sprintf("[environments.%s]\n", envName))
		b.WriteString(fmt.Sprintf("description = \"%s\"\n", env.Description))
		b.WriteString(fmt.Sprintf("# %s\n", env.Comment))
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

// tomlEnvironment represents an environment in the TOML file
type tomlEnvironment struct {
	Description string `toml:"description"`
	Comment     string `toml:"-"` // Not serialized, just for generation
}

func generateEnvFile(path string, env EnvironmentInput) error {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Lockplane Environment: %s\n", env.Name))
	b.WriteString("# Generated by: lockplane init\n")
	b.WriteString("#\n")
	b.WriteString("# Do not commit this file if it contains secrets!\n")
	b.WriteString("#\n")

	switch env.DatabaseType {
	case "postgres":
		connStr := BuildPostgresConnectionString(env)
		b.WriteString(fmt.Sprintf("# PostgreSQL connection (auto-detected sslmode=%s)\n", env.SSLMode))
		b.WriteString(fmt.Sprintf("POSTGRES_URL=%s\n", connStr))

		shadowConnStr := BuildPostgresShadowConnectionString(env)
		b.WriteString("# Shadow database (always configured for PostgreSQL - safe migrations)\n")
		b.WriteString(fmt.Sprintf("POSTGRES_SHADOW_URL=%s\n", shadowConnStr))

	case "sqlite":
		connStr := BuildSQLiteConnectionString(env)
		b.WriteString("# SQLite connection (file-based)\n")
		b.WriteString(fmt.Sprintf("SQLITE_DB_PATH=%s\n", connStr))

		shadowConnStr := BuildSQLiteShadowConnectionString(env)
		b.WriteString("# Shadow database (configured for SQLite - safe migrations)\n")
		b.WriteString(fmt.Sprintf("SQLITE_SHADOW_DB_PATH=%s\n", shadowConnStr))

	case "libsql":
		connStr := BuildLibSQLConnectionString(env)
		b.WriteString("# libSQL/Turso connection (remote edge database)\n")
		b.WriteString(fmt.Sprintf("LIBSQL_URL=%s\n", connStr))

		// Add auth token as separate variable for better security and flexibility
		if env.AuthToken != "" {
			b.WriteString(fmt.Sprintf("LIBSQL_AUTH_TOKEN=%s\n", env.AuthToken))
		} else {
			b.WriteString("LIBSQL_AUTH_TOKEN=\n")
		}

		shadowConnStr := BuildLibSQLShadowConnectionString(env)
		b.WriteString("# Shadow database (local SQLite for validation - safe migrations)\n")
		b.WriteString(fmt.Sprintf("LIBSQL_SHADOW_DB_PATH=%s\n", shadowConnStr))
	}

	// Write with restrictive permissions (owner read/write only)
	return os.WriteFile(path, []byte(b.String()), 0600)
}

func createOrUpdateEnvExample(environments []EnvironmentInput) error {
	examplePath := ".env.example"

	// Read existing .env.example if it exists
	existingContent := ""
	if data, err := os.ReadFile(examplePath); err == nil {
		existingContent = string(data)
	}

	// Check if database-specific variables already exist
	hasPostgresURL := strings.Contains(existingContent, "POSTGRES_URL=")
	hasPostgresShadowURL := strings.Contains(existingContent, "POSTGRES_SHADOW_URL=")
	hasSQLiteDBPath := strings.Contains(existingContent, "SQLITE_DB_PATH=")
	hasSQLiteShadowDBPath := strings.Contains(existingContent, "SQLITE_SHADOW_DB_PATH=")
	hasLibSQLURL := strings.Contains(existingContent, "LIBSQL_URL=")
	hasLibSQLAuthToken := strings.Contains(existingContent, "LIBSQL_AUTH_TOKEN=")
	hasLibSQLShadowDBPath := strings.Contains(existingContent, "LIBSQL_SHADOW_DB_PATH=")

	// Determine which database types are being used
	hasPostgres := false
	hasSQLite := false
	hasLibSQL := false
	for _, env := range environments {
		switch env.DatabaseType {
		case "postgres":
			hasPostgres = true
		case "sqlite":
			hasSQLite = true
		case "libsql":
			hasLibSQL = true
		}
	}

	// Check if we need to add anything
	needsPostgres := hasPostgres && (!hasPostgresURL || !hasPostgresShadowURL)
	needsSQLite := hasSQLite && (!hasSQLiteDBPath || !hasSQLiteShadowDBPath)
	needsLibSQL := hasLibSQL && (!hasLibSQLURL || !hasLibSQLAuthToken || !hasLibSQLShadowDBPath)

	// If nothing needs to be added, we're done
	if !needsPostgres && !needsSQLite && !needsLibSQL {
		return nil
	}

	// Build the content to append
	var b strings.Builder

	// If file exists and has content, ensure there's a newline separator
	if existingContent != "" && !strings.HasSuffix(existingContent, "\n") {
		b.WriteString("\n")
	}

	// Add header comment if file is new or doesn't have lockplane section
	if existingContent == "" || !strings.Contains(existingContent, "Lockplane") {
		b.WriteString("\n# Lockplane Configuration\n")
		b.WriteString("# Copy to .env.<environment> and fill in your actual values\n")
	}

	// Add PostgreSQL variables if needed
	if needsPostgres {
		b.WriteString("\n# PostgreSQL\n")
		if !hasPostgresURL {
			b.WriteString("POSTGRES_URL=postgresql://user:password@localhost:5432/database?sslmode=disable\n")
		}
		if !hasPostgresShadowURL {
			b.WriteString("POSTGRES_SHADOW_URL=postgresql://user:password@localhost:5433/database_shadow?sslmode=disable\n")
		}
	}

	// Add SQLite variables if needed
	if needsSQLite {
		b.WriteString("\n# SQLite\n")
		if !hasSQLiteDBPath {
			b.WriteString("SQLITE_DB_PATH=./schema/lockplane.db\n")
		}
		if !hasSQLiteShadowDBPath {
			b.WriteString("SQLITE_SHADOW_DB_PATH=./schema/lockplane_shadow.db\n")
		}
	}

	// Add libSQL variables if needed
	if needsLibSQL {
		b.WriteString("\n# libSQL/Turso\n")
		if !hasLibSQLURL {
			b.WriteString("LIBSQL_URL=libsql://your-database.turso.io\n")
		}
		if !hasLibSQLAuthToken {
			b.WriteString("LIBSQL_AUTH_TOKEN=your_turso_auth_token_here\n")
		}
		if !hasLibSQLShadowDBPath {
			b.WriteString("LIBSQL_SHADOW_DB_PATH=./schema/turso_shadow.db\n")
		}
	}

	// Append to existing content
	newContent := existingContent + b.String()

	// Write with standard permissions (readable by all, writable by owner)
	return os.WriteFile(examplePath, []byte(newContent), 0644)
}

func updateGitignore() error {
	gitignorePath := ".gitignore"

	// Read existing .gitignore if it exists
	content := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	// Check if .env.* pattern already exists
	if strings.Contains(content, ".env.*") || strings.Contains(content, ".env.") {
		// Already has the pattern, don't add again
		return nil
	}

	// Append the lockplane section
	lockplaneSection := `
# Lockplane environment files (added by lockplane init)
# DO NOT remove - contains database credentials
.env.*
!.env.*.example
`

	content += lockplaneSection

	return os.WriteFile(gitignorePath, []byte(content), 0644)
}
