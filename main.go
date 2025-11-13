package main

import (
	"context"
	"database/sql"

	_ "github.com/lib/pq"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"

	"github.com/lockplane/lockplane/cmd"
	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/planner"
)

// Type aliases for backward compatibility with tests
type Schema = database.Schema
type Table = database.Table
type Column = database.Column
type Index = database.Index
type ForeignKey = database.ForeignKey

func main() {
	cmd.Execute()
}

// Wrapper functions for backward compatibility with integration tests
// These delegate to the executor package

func applyPlan(ctx context.Context, db *sql.DB, plan *planner.Plan, shadowDB *sql.DB, currentSchema *Schema, driver database.Driver, verbose bool) (*planner.ExecutionResult, error) {
	return executor.ApplyPlan(ctx, db, plan, shadowDB, currentSchema, driver, verbose)
}

func detectDriver(connString string) string {
	return executor.DetectDriver(connString)
}

func newDriver(driverName string) (database.Driver, error) {
	return executor.NewDriver(driverName)
}

func getSQLDriverName(driverType string) string {
	return executor.GetSQLDriverName(driverType)
}
