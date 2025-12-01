package database

// Schema represents a database schema
type Schema struct {
	// Tables  []Table `json:"tables"`
	// Dialect Dialect `json:"dialect,omitempty"`
}

type ConnectionConfig struct {
	DatabaseType string // TODO make enum?
	PostgresUrl  string
}
