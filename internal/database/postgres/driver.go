package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lockplane/lockplane/internal/database"
)

// Driver implements database.Driver for PostgreSQL
type Driver struct {
}

// NewDriver creates a new PostgreSQL driver
func NewDriver() *Driver {
	return &Driver{}
}

// Name returns the database driver name
func (d *Driver) Name() string {
	return "postgres"
}

// TestConnection attempts to connect to the database
func (d *Driver) TestConnection(cfg database.ConnectionConfig) error {
	db, err := sql.Open("postgres", cfg.PostgresUrl)
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
