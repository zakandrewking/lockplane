// SQLite-specific schema definitions
// Import this for SQLite databases to get type checking for SQLite-specific types

package sqlite

import base "github.com/lockplane/lockplane/schema"

// Re-export base definitions
#Schema: base.#Schema
#Table:  base.#Table
#Index:  base.#Index
#Plan:   base.#Plan
#PlanStep: base.#PlanStep

// SQLite column types
// Note: SQLite has flexible typing but these are the canonical type affinities
#ColumnType:
	"INTEGER" |
	"TEXT" |
	"REAL" |
	"BLOB" |
	"NUMERIC" |
	// Common aliases
	"INT" |
	"VARCHAR" |
	"CHAR" |
	"BOOLEAN" |
	"DATE" |
	"DATETIME" |
	"TIMESTAMP"

// Override the base Column to use SQLite types
#Column: base.#Column & {
	type: #ColumnType
}

// SQLite-specific common column patterns

#ID: #Column & {
	name: "id"
	type: "INTEGER"
	nullable: false
	is_primary_key: true
	// SQLite AUTOINCREMENT
}

#CreatedAt: #Column & {
	name: "created_at"
	type: "DATETIME"
	nullable: true
	default: "CURRENT_TIMESTAMP"
}

#UpdatedAt: #Column & {
	name: "updated_at"
	type: "DATETIME"
	nullable: true
	default: "CURRENT_TIMESTAMP"
}

#Email: #Column & {
	name: "email"
	type: "TEXT"
	nullable: false
}

// Common naming conventions

#TimestampColumn: #Column & {
	name: =~"_at$" // must end in _at
	type: "DATETIME" | "TIMESTAMP"
}

#BooleanColumn: #Column & {
	name: =~"^(is_|has_|can_|should_)" // booleans start with is/has/can/should
	type: "BOOLEAN" | "INTEGER" // SQLite often uses INTEGER for booleans
}

#JSONColumn: #Column & {
	type: "TEXT" // SQLite stores JSON as TEXT
}
