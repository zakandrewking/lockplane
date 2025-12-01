package executor

import (
	"fmt"

	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/database/postgres"
)

// NewDriver creates a new database driver based on the driver name.
// TODO share enum with ConnectionConfig
func NewDriver(databaseType string) (database.Driver, error) {
	switch databaseType {
	case "postgres":
		return postgres.NewDriver(), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", databaseType)
	}
}
