package main

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the lockplane.toml configuration file
type Config struct {
	DatabaseURL       string `toml:"database_url"`
	ShadowDatabaseURL string `toml:"shadow_database_url"`
	SchemaPath        string `toml:"schema_path"`
}

// LoadConfig loads the lockplane.toml file from the current directory or any parent directory
func LoadConfig() (*Config, error) {
	// Start from current directory and walk up to find lockplane.toml
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	for {
		configPath := filepath.Join(dir, "lockplane.toml")
		if _, err := os.Stat(configPath); err == nil {
			// Found config file, load it
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, err
			}

			var config Config
			if err := toml.Unmarshal(data, &config); err != nil {
				return nil, err
			}

			return &config, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, no config file found
			break
		}
		dir = parent
	}

	// No config file found, return empty config
	return &Config{}, nil
}

// GetDatabaseURL returns the database URL with priority: explicit value > env var > config > default
func GetDatabaseURL(explicitValue string, config *Config, defaultValue string) string {
	if explicitValue != "" {
		return explicitValue
	}
	if envValue := os.Getenv("DATABASE_URL"); envValue != "" {
		return envValue
	}
	if config != nil && config.DatabaseURL != "" {
		return config.DatabaseURL
	}
	return defaultValue
}

// GetShadowDatabaseURL returns the shadow database URL with priority: explicit value > env var > config > default
func GetShadowDatabaseURL(explicitValue string, config *Config, defaultValue string) string {
	if explicitValue != "" {
		return explicitValue
	}
	if envValue := os.Getenv("SHADOW_DATABASE_URL"); envValue != "" {
		return envValue
	}
	if config != nil && config.ShadowDatabaseURL != "" {
		return config.ShadowDatabaseURL
	}
	return defaultValue
}

// GetSchemaPath returns the schema path with priority: explicit value > config > default
func GetSchemaPath(explicitValue string, config *Config, defaultValue string) string {
	if explicitValue != "" {
		return explicitValue
	}
	if config != nil && config.SchemaPath != "" {
		return config.SchemaPath
	}
	return defaultValue
}
