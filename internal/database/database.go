package database

type ConnectionConfig struct {
	DatabaseType string // TODO make enum?
	PostgresUrl  string
}

// Driver represents a database driver with introspection and SQL generation
type Driver interface {
	// Name returns the database driver name
	Name() string

	// TestConnection attempts to connect to the database
	// TODO when to pass as pointer?
	TestConnection(cfg ConnectionConfig) error
}
