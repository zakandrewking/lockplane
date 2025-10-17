// Base schema definitions common to all database dialects
// This file contains the core structure for tables, columns, and indexes
// Dialect-specific type definitions are in separate files (postgres.cue, sqlite.cue, etc.)

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
// The type field will be constrained by dialect-specific schemas
#Column: {
	name: string & =~"^[a-z_][a-z0-9_]*$" // snake_case column names
	type: string // Dialect-specific schemas will constrain this
	nullable: bool | *true // nullable by default
	default?: string
	is_primary_key: bool | *false
}

// Index represents a database index
#Index: {
	name: string & =~"^[a-z_][a-z0-9_]*$" // snake_case index names
	columns: [...string] // column names
	unique: bool | *false
}

// Migration plan types

#Plan: {
	steps: [...#PlanStep] & list.MinItems(1)
}

#PlanStep: {
	description: string & =~".+" // non-empty description
	sql:         string & =~".+" // non-empty SQL
}
