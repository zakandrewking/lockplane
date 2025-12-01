package database

// Schema represents a database schema
type Schema struct {
	Tables []Table `json:"tables"`
	// Dialect Dialect `json:"dialect,omitempty"`
}

// Table represents a database table
type Table struct {
	Name    string   `json:"name"`
	Schema  string   `json:"schema,omitempty"` // Schema name (e.g., "public", "storage")
	Columns []Column `json:"columns"`
	// Indexes     []Index      `json:"indexes"`
	// ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
	RLSEnabled bool `json:"rls_enabled,omitempty"`
	// Policies    []Policy     `json:"policies,omitempty"` // Row Level Security policies
}

// Column represents a table column
type Column struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default,omitempty"`
	IsPrimaryKey bool    `json:"is_primary_key"`
}

type ConnectionConfig struct {
	DatabaseType string // TODO make enum?
	PostgresUrl  string
}
