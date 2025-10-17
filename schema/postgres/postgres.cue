// PostgreSQL-specific schema definitions
// Import this for PostgreSQL databases to get type checking for Postgres-specific types

package postgres

import base "github.com/lockplane/lockplane/schema"

// Re-export base definitions
#Schema: base.#Schema
#Table:  base.#Table
#Index:  base.#Index
#Plan:   base.#Plan
#PlanStep: base.#PlanStep

// PostgreSQL column types
#ColumnType:
	"text" |
	"varchar" |
	"char" |
	"integer" |
	"bigint" |
	"smallint" |
	"serial" |
	"bigserial" |
	"smallserial" |
	"boolean" |
	"timestamp" |
	"timestamp without time zone" |
	"timestamp with time zone" |
	"timestamptz" |
	"date" |
	"time" |
	"time without time zone" |
	"time with time zone" |
	"timetz" |
	"interval" |
	"numeric" |
	"decimal" |
	"real" |
	"double precision" |
	"money" |
	"uuid" |
	"json" |
	"jsonb" |
	"xml" |
	"bytea" |
	"bit" |
	"bit varying" |
	"inet" |
	"cidr" |
	"macaddr" |
	"tsvector" |
	"tsquery" |
	"point" |
	"line" |
	"lseg" |
	"box" |
	"path" |
	"polygon" |
	"circle"

// Override the base Column to use Postgres types
#Column: base.#Column & {
	type: #ColumnType
}

// PostgreSQL-specific common column patterns

#ID: #Column & {
	name: "id"
	type: "integer"
	nullable: false
	is_primary_key: true
	default: "nextval('${_table}_id_seq'::regclass)"
}

#BigID: #Column & {
	name: "id"
	type: "bigint"
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

#Serial: #Column & {
	name: "id"
	type: "serial"
	nullable: false
	is_primary_key: true
}

#BigSerial: #Column & {
	name: "id"
	type: "bigserial"
	nullable: false
	is_primary_key: true
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
	type: "timestamp" | "timestamp without time zone" | "timestamp with time zone" | "timestamptz"
}

#BooleanColumn: #Column & {
	name: =~"^(is_|has_|can_|should_)" // booleans start with is/has/can/should
	type: "boolean"
}

#JSONColumn: #Column & {
	type: "json" | "jsonb"
}
