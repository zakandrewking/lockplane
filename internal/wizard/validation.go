package wizard

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// ValidateEnvironmentName checks if an environment name is valid
func ValidateEnvironmentName(name string) error {
	if name == "" {
		return fmt.Errorf("environment name cannot be empty")
	}

	// Must be alphanumeric or underscore
	for _, ch := range name {
		isValid := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-'
		if !isValid {
			return fmt.Errorf("environment name must contain only letters, numbers, underscores, and hyphens")
		}
	}

	return nil
}

// ValidatePort checks if a port number is valid
func ValidatePort(port string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("port must be a number")
	}

	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	return nil
}

// ValidateConnectionString checks if a connection string is well-formed
func ValidateConnectionString(connStr string, dbType string) error {
	if connStr == "" {
		return fmt.Errorf("connection string cannot be empty")
	}

	switch dbType {
	case "postgres":
		// Check for postgresql:// or postgres://
		if !strings.HasPrefix(connStr, "postgres://") &&
			!strings.HasPrefix(connStr, "postgresql://") {
			return fmt.Errorf("PostgreSQL connection string must start with postgres:// or postgresql://")
		}

	case "sqlite":
		// Check for sqlite:// or file path
		if !strings.HasPrefix(connStr, "sqlite://") &&
			!strings.HasPrefix(connStr, "./") &&
			!strings.HasPrefix(connStr, "/") &&
			!strings.Contains(connStr, ".db") {
			return fmt.Errorf("SQLite connection string must be sqlite:// or a file path")
		}

	case "libsql":
		// Check for libsql://
		if !strings.HasPrefix(connStr, "libsql://") {
			return fmt.Errorf("libSQL connection string must start with libsql://")
		}
	}

	return nil
}

// TestConnection attempts to connect to the database
func TestConnection(connStr string, dbType string) error {
	var driverName string
	switch dbType {
	case "postgres":
		driverName = "postgres"
	case "sqlite":
		driverName = "sqlite"
		// For SQLite, adjust the connection string format
		connStr = strings.TrimPrefix(connStr, "sqlite://")
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	db, err := sql.Open(driverName, connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

// BuildPostgresConnectionString constructs a PostgreSQL connection string
func BuildPostgresConnectionString(env EnvironmentInput) string {
	// Auto-detect SSL mode based on host
	sslMode := env.SSLMode
	if sslMode == "" {
		if env.Host == "localhost" || env.Host == "127.0.0.1" {
			sslMode = "disable"
		} else {
			sslMode = "require"
		}
	}

	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		env.User, env.Password, env.Host, env.Port, env.Database, sslMode)
}

// BuildPostgresShadowConnectionString constructs a shadow DB connection string
func BuildPostgresShadowConnectionString(env EnvironmentInput) string {
	sslMode := env.SSLMode
	if sslMode == "" {
		if env.Host == "localhost" || env.Host == "127.0.0.1" {
			sslMode = "disable"
		} else {
			sslMode = "require"
		}
	}

	shadowPort := env.ShadowDBPort
	if shadowPort == "" {
		shadowPort = "5433"
	}

	shadowDB := env.Database + "_shadow"

	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		env.User, env.Password, env.Host, shadowPort, shadowDB, sslMode)
}

// BuildSQLiteConnectionString constructs a SQLite connection string
func BuildSQLiteConnectionString(env EnvironmentInput) string {
	filePath := env.FilePath
	if filePath == "" {
		filePath = "schema/lockplane.db"
	}

	return fmt.Sprintf("sqlite://%s", filePath)
}

// BuildLibSQLConnectionString constructs a libSQL connection string
func BuildLibSQLConnectionString(env EnvironmentInput) string {
	if env.AuthToken != "" {
		return fmt.Sprintf("%s?authToken=%s", env.URL, env.AuthToken)
	}
	return env.URL
}
