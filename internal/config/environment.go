package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// ResolvedEnvironment represents a fully-resolved environment with concrete values.
type ResolvedEnvironment struct {
	Name              string
	DatabaseURL       string
	ShadowDatabaseURL string
	SchemaPath        string
	DotenvPath        string
	FromConfig        bool
	FromDotenv        bool
	ResolvedConfigDir string
}

// ResolveEnvironment resolves a named environment into concrete connection strings.
func ResolveEnvironment(config *Config, name string) (*ResolvedEnvironment, error) {
	envName := strings.TrimSpace(name)
	if envName == "" {
		if config != nil && config.DefaultEnvironment != "" {
			envName = config.DefaultEnvironment
		} else {
			envName = defaultEnvironmentName
		}
	}

	var (
		envConfig EnvironmentConfig
		envExists bool
	)
	if config != nil && config.Environments != nil {
		if cfg, ok := config.Environments[envName]; ok {
			envConfig = cfg
			envExists = true
		}
	}

	resolved := &ResolvedEnvironment{
		Name:              envName,
		SchemaPath:        "",
		ResolvedConfigDir: "",
	}

	if config != nil {
		resolved.ResolvedConfigDir = config.ConfigDir()
		if config.SchemaPath != "" {
			resolved.SchemaPath = config.SchemaPath
		}
		if config.DatabaseURL != "" && envConfig.DatabaseURL == "" {
			envConfig.DatabaseURL = config.DatabaseURL
		}
		if config.ShadowDatabaseURL != "" && envConfig.ShadowDatabaseURL == "" {
			envConfig.ShadowDatabaseURL = config.ShadowDatabaseURL
		}
	}

	if envConfig.SchemaPath != "" {
		resolved.SchemaPath = envConfig.SchemaPath
	}

	resolved.DatabaseURL = envConfig.DatabaseURL
	resolved.ShadowDatabaseURL = envConfig.ShadowDatabaseURL
	if envExists {
		resolved.FromConfig = true
	}

	var (
		baseDir        string
		dotenvFileName = ".env." + envName
	)
	var projectDir string
	if config != nil {
		baseDir = config.ConfigDir()
		projectDir = config.ProjectDir()
	} else if cwd, err := os.Getwd(); err == nil {
		baseDir = cwd
	}

	if baseDir != "" {
		resolved.DotenvPath = filepath.Join(baseDir, dotenvFileName)
	} else {
		resolved.DotenvPath = dotenvFileName
	}

	if _, err := os.Stat(resolved.DotenvPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to access %s: %w", resolved.DotenvPath, err)
		}
		if projectDir != "" && projectDir != baseDir {
			altPath := filepath.Join(projectDir, dotenvFileName)
			if altInfo, altErr := os.Stat(altPath); altErr == nil && !altInfo.IsDir() {
				resolved.DotenvPath = altPath
			}
		}
	}

	if info, err := os.Stat(resolved.DotenvPath); err == nil && !info.IsDir() {
		values, err := godotenv.Read(resolved.DotenvPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", resolved.DotenvPath, err)
		}
		resolved.FromDotenv = true

		// Check for generic DATABASE_URL first
		if value := values["DATABASE_URL"]; value != "" {
			resolved.DatabaseURL = value
		}
		if value := values["SHADOW_DATABASE_URL"]; value != "" {
			resolved.ShadowDatabaseURL = value
		}

		// Then check for database-specific variables (these take precedence if DATABASE_URL wasn't set)
		// PostgreSQL
		if resolved.DatabaseURL == "" {
			if value := values["POSTGRES_URL"]; value != "" {
				resolved.DatabaseURL = value
			}
		}
		if resolved.ShadowDatabaseURL == "" {
			if value := values["POSTGRES_SHADOW_URL"]; value != "" {
				resolved.ShadowDatabaseURL = value
			}
		}

		// SQLite
		if resolved.DatabaseURL == "" {
			if value := values["SQLITE_DB_PATH"]; value != "" {
				resolved.DatabaseURL = value
			}
		}
		if resolved.ShadowDatabaseURL == "" {
			if value := values["SQLITE_SHADOW_DB_PATH"]; value != "" {
				resolved.ShadowDatabaseURL = value
			}
			// Also check the variant with different naming
			if resolved.ShadowDatabaseURL == "" {
				if value := values["SHADOW_SQLITE_DB_PATH"]; value != "" {
					resolved.ShadowDatabaseURL = value
				}
			}
		}

		// libSQL
		if resolved.DatabaseURL == "" {
			if value := values["LIBSQL_URL"]; value != "" {
				// Construct libSQL connection string with auth token if available
				if authToken := values["LIBSQL_AUTH_TOKEN"]; authToken != "" {
					resolved.DatabaseURL = fmt.Sprintf("%s?authToken=%s", value, authToken)
				} else {
					resolved.DatabaseURL = value
				}
			}
		}
		if resolved.ShadowDatabaseURL == "" {
			if value := values["LIBSQL_SHADOW_DB_PATH"]; value != "" {
				resolved.ShadowDatabaseURL = value
			}
		}

		if resolved.SchemaPath == "" {
			if value := values["SCHEMA_PATH"]; value != "" {
				resolved.SchemaPath = value
			}
		}
	}

	if resolved.DatabaseURL == "" {
		resolved.DatabaseURL = defaultDatabaseURL
	}
	if resolved.ShadowDatabaseURL == "" {
		resolved.ShadowDatabaseURL = defaultShadowDatabaseURL
	}

	if resolved.SchemaPath != "" {
		base := resolved.ResolvedConfigDir
		if base == "" && config != nil {
			base = config.ConfigDir()
		}
		resolved.SchemaPath = resolveSchemaPath(resolved.SchemaPath, base)
	}

	if config != nil && config.Environments != nil && len(config.Environments) > 0 && !envExists {
		if !resolved.FromDotenv {
			return nil, fmt.Errorf("environment %q not defined in lockplane.toml and %s not found", envName, resolved.DotenvPath)
		}
	}

	return resolved, nil
}
