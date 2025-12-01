package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type ConnectionConfig struct {
	DatabaseType string // TODO make enum?
	PostgresUrl  string
}

// TestConnection attempts to connect to the database
// TODO pass pointer for a config? safer vs faster?
func TestConnection(cfg ConnectionConfig) error {
	// TODO define 2 variables at once?
	var driverName string
	var connectionString string

	switch cfg.DatabaseType {
	case "postgres":
		driverName = "postgres"
		connectionString = cfg.PostgresUrl
	default:
		return fmt.Errorf("unsupported database type: %s", cfg.DatabaseType)
	}

	db, err := sql.Open(driverName, connectionString)
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
