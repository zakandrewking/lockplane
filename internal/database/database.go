package database

import (
	"fmt"

	"github.com/lockplane/lockplane/internal/database/connection"
	"github.com/lockplane/lockplane/internal/database/postgres"
)

// Driver represents a database driver with introspection and SQL generation
type Driver interface {
	// Name returns the database driver name
	Name() string

	// TestConnection attempts to connect to the database
	// TODO when to pass as pointer?
	TestConnection(cfg connection.ConnectionConfig) error
}

// NewDriver creates a new database driver based on the driver name.
// TODO share enum with ConnectionConfig
func NewDriver(databaseType string) (Driver, error) {
	switch databaseType {
	case "postgres":
		return postgres.NewDriver(), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", databaseType)
	}
}
