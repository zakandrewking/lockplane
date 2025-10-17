// Lockplane schema definitions
// Import this to get type checking and validation for your database schemas

package schema

import "list"

// Schema represents a complete database schema
#Schema: {
	tables: [...#Table]
}

// Table represents a database table
#Table: {
	name: string & =~"^[a-z_][a-z0-9_]*$" // snake_case table names
	columns: [...#Column] & list.MinItems(1) // at least one column
	indexes?: [...#Index]

	// Validate that primary key columns exist
	let pkColumns = [ for col in columns if col.is_primary_key {col.name}]
	if len(pkColumns) > 0 {
		// At least one PK column must exist if any are marked
		columns: list.MinItems(1)
	}
}

// Column represents a table column
#Column: {
	name: string & =~"^[a-z_][a-z0-9_]*$" // snake_case column names
	type: #ColumnType
	nullable: bool | *true // nullable by default
	default?: string
	is_primary_key: bool | *false
}

// ColumnType defines supported Postgres types
#ColumnType:
	"text" |
	"integer" |
	"bigint" |
	"smallint" |
	"boolean" |
	"timestamp" |
	"timestamp without time zone" |
	"timestamp with time zone" |
	"date" |
	"time" |
	"numeric" |
	"real" |
	"double precision" |
	"uuid" |
	"json" |
	"jsonb" |
	"bytea"

// Index represents a database index
#Index: {
	name: string & =~"^[a-z_][a-z0-9_]*$" // snake_case index names
	columns: [...string] // column names
	unique: bool | *false
}

// Common column patterns you can use in your schemas

#ID: #Column & {
	name: "id"
	type: "integer"
	nullable: false
	is_primary_key: true
	default: "nextval('${_table}_id_seq'::regclass)"
}

#UUID_ID: #Column & {
	name: "id"
	type: "uuid"
	nullable: false
	is_primary_key: true
	default: "gen_random_uuid()"
}

#CreatedAt: #Column & {
	name: "created_at"
	type: "timestamp without time zone"
	nullable: true
	default: "now()"
}

#UpdatedAt: #Column & {
	name: "updated_at"
	type: "timestamp without time zone"
	nullable: true
	default: "now()"
}

#Email: #Column & {
	name: "email"
	type: "text"
	nullable: false
}

// Common naming conventions

#TimestampColumn: #Column & {
	name: =~"_at$" // must end in _at
	type: "timestamp" | "timestamp without time zone" | "timestamp with time zone"
}

#BooleanColumn: #Column & {
	name: =~"^(is_|has_|can_|should_)" // booleans start with is/has/can/should
	type: "boolean"
}

// Migration plan types

#Plan: {
	steps: [...#PlanStep] & list.MinItems(1)
}

#PlanStep: {
	description: string & =~".+" // non-empty description
	sql:         string & =~".+" // non-empty SQL
}
