// Example: Comparison of PostgreSQL vs SQLite types
// This file shows how the same schema might differ between dialects

package comparison

// PostgreSQL version
import postgres "github.com/lockplane/lockplane/schema/postgres"

postgres_products: postgres.#Table & {
	name: "products"
	columns: [
		{
			name:           "id"
			type:           "serial"
			nullable:       false
			is_primary_key: true
		},
		{
			name:     "name"
			type:     "varchar"
			nullable: false
		},
		{
			name:     "price"
			type:     "numeric"
			nullable: false
		},
		{
			name:     "metadata"
			type:     "jsonb"
			nullable: true
		},
		{
			name:     "created_at"
			type:     "timestamptz"
			nullable: false
			default:  "now()"
		},
	]
}

// SQLite version (same logical schema, different types)
import sqlite "github.com/lockplane/lockplane/schema/sqlite"

sqlite_products: sqlite.#Table & {
	name: "products"
	columns: [
		{
			name:           "id"
			type:           "INTEGER"
			nullable:       false
			is_primary_key: true
		},
		{
			name:     "name"
			type:     "TEXT"
			nullable: false
		},
		{
			name:     "price"
			type:     "NUMERIC"
			nullable: false
		},
		{
			name:     "metadata"
			type:     "TEXT" // SQLite stores JSON as TEXT
			nullable: true
		},
		{
			name:     "created_at"
			type:     "DATETIME"
			nullable: false
			default:  "CURRENT_TIMESTAMP"
		},
	]
}
