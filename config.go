package main

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultEnvironmentName   = "local"
	defaultDatabaseURL       = "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
	defaultShadowDatabaseURL = "postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable"
)

// EnvironmentConfig describes a single named environment from lockplane.toml.
type EnvironmentConfig struct {
	Description       string `toml:"description"`
	DatabaseURL       string `toml:"database_url"`
	ShadowDatabaseURL string `toml:"shadow_database_url"`
	SchemaPath        string `toml:"schema_path"`
}

// Config represents the lockplane.toml configuration file.
type Config struct {
	DefaultEnvironment string                       `toml:"default_environment"`
	SchemaPath         string                       `toml:"schema_path"`
	DatabaseURL        string                       `toml:"database_url"`        // legacy fallback
	ShadowDatabaseURL  string                       `toml:"shadow_database_url"` // legacy fallback
	Environments       map[string]EnvironmentConfig `toml:"environments"`
	configDir          string                       `toml:"-"`
}

// LoadConfig loads the lockplane.toml file from the current directory or any parent directory.
func LoadConfig() (*Config, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dir := startDir
	for {
		configPath := filepath.Join(dir, "lockplane.toml")
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, err
			}

			var config Config
			if err := toml.Unmarshal(data, &config); err != nil {
				return nil, err
			}

			config.configDir = dir
			return &config, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return &Config{configDir: startDir}, nil
}

// ConfigDir returns the directory containing the resolved lockplane.toml (or the working directory if none exists).
func (c *Config) ConfigDir() string {
	if c == nil {
		return ""
	}
	if c.configDir != "" {
		return c.configDir
	}
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return ""
}

// GetSchemaPath returns the schema path with priority: explicit value > environment config > global config > default.
func GetSchemaPath(explicitValue string, config *Config, env *ResolvedEnvironment, defaultValue string) string {
	if explicitValue != "" {
		return explicitValue
	}
	if env != nil && env.SchemaPath != "" {
		return env.SchemaPath
	}
	if config != nil && config.SchemaPath != "" {
		return config.SchemaPath
	}
	return defaultValue
}
