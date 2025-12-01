package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/lockplane/lockplane/internal/database/connection"
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

// Open a connection to the database, and run a ping to test it
func (d *Driver) OpenConnection(cfg connection.ConnectionConfig) (*sql.DB, error) {
	// TODO enable ssl
	var finalUrl = cfg.PostgresUrl + "?sslmode=disable"
	db, err := sql.Open("postgres", finalUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		// TODO defer necessary?
		defer func() { _ = db.Close() }()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
