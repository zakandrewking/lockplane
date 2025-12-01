package driver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/driver/postgres"
)

// Driver represents a database driver with introspection and SQL generation
type Driver interface {
	// Name returns the database driver name
	Name() string

	// TestConnection attempts to connect to the database
	// TODO when to pass as pointer?
	OpenConnection(cfg database.ConnectionConfig) (*sql.DB, error)

	// IntrospectSchema reads the entire database schema
	IntrospectSchema(ctx context.Context, db *sql.DB, schemaName string) (*database.Schema, error)
}

// NewDriver creates a new database driver based on the driver name.
// TODO share enum with ConnectionConfig
func NewDriver(databaseType database.DatabaseType) (Driver, error) {
	switch databaseType {
	case database.DatabaseTypePostgres:
		return postgres.NewDriver(), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", databaseType)
	}
}
