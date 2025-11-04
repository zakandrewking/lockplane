package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/database"
	sqliteDriver "github.com/lockplane/lockplane/database/sqlite"

	_ "modernc.org/sqlite"
)

// parseSQLiteSQLSchema loads SQLite DDL by executing it against an in-memory database,
// then introspecting the resulting schema.
func parseSQLiteSQLSchema(ddl string) (*database.Schema, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open in-memory sqlite database: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Enable foreign key enforcement to match typical production settings.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	ddlStatements := strings.TrimSpace(ddl)
	if ddlStatements != "" {
		if _, err := db.ExecContext(ctx, ddlStatements); err != nil {
			return nil, fmt.Errorf("failed to execute sqlite schema: %w", err)
		}
	}

	introspector := sqliteDriver.NewIntrospector()
	schema, err := introspector.IntrospectSchema(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect sqlite schema: %w", err)
	}

	schema.Dialect = database.DialectSQLite
	return schema, nil
}
