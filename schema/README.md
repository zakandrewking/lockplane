# Lockplane CUE Schemas

This directory contains CUE schema definitions for Lockplane. Import these in your own schemas to get type checking, validation, and IDE support.

## Quick Start

**1. Create your schema:**

```cue
// myapp.cue
package myapp

import "github.com/lockplane/lockplane/schema"

schema.#Schema & {
	tables: [users]
}

users: schema.#Table & {
	name: "users"
	columns: [
		schema.#ID,
		schema.#Email,
		schema.#CreatedAt,
	]
}
```

**2. Validate it:**

```bash
cue vet myapp.cue
```

**3. Export to JSON:**

```bash
go run cmd/cue-export/main.go -cue myapp.cue -json desired_schema.json
```

**4. Use with Lockplane:**

```bash
# Compare with current schema
go run main.go > current_schema.json
# (Diff engine will compare these in the future)
```

## IDE Support

**VS Code:** Install the [CUE extension](https://marketplace.visualstudio.com/items?itemName=cuelang.vscode-cue)
- Autocomplete on field names
- Inline validation errors
- Type checking as you type
- Hover for documentation

## Available Definitions

### Core Types

- `#Schema` - Complete database schema
- `#Table` - Database table
- `#Column` - Table column
- `#Index` - Table index
- `#ColumnType` - Supported Postgres types

### Common Patterns

Pre-defined column patterns you can use:

- `#ID` - Standard integer ID with sequence
- `#UUID_ID` - UUID primary key
- `#Email` - Email text column
- `#CreatedAt` - Created timestamp
- `#UpdatedAt` - Updated timestamp

### Validation Rules

Built-in validations:
- Snake_case naming (`users`, `created_at`)
- At least one column per table
- Valid Postgres types
- Timestamp columns must end in `_at`
- Boolean columns start with `is_`, `has_`, `can_`, or `should_`

## Examples

See `examples/schemas/` for:
- `simple.cue` - Minimal schema
- `notes_app.cue` - Full application with relationships

## Advanced Usage

**Reusable column definitions:**

```cue
#UserReference: schema.#Column & {
	name: "user_id"
	type: "integer"
	nullable: false
}

posts: schema.#Table & {
	name: "posts"
	columns: [
		schema.#ID,
		#UserReference,  // Reuse your definition
		{name: "title", type: "text", nullable: false},
	]
}
```

**Conditional fields:**

```cue
#AuditTable: schema.#Table & {
	columns: [
		...
		schema.#CreatedAt,
		schema.#UpdatedAt,
		if audited {
			{name: "created_by", type: "integer", nullable: false}
		},
	]
}
```

**Table inheritance:**

```cue
#BaseTable: schema.#Table & {
	columns: [
		schema.#ID,
		schema.#CreatedAt,
	]
}

users: #BaseTable & {
	name: "users"
	columns: [
		...#BaseTable.columns,
		schema.#Email,
	]
}
```

## Why CUE?

**Traditional approach:**
- Write schema
- Validate with separate tool
- Hope for the best

**With CUE:**
- Types ARE validation
- Errors in your IDE immediately
- Compose and reuse definitions
- Mathematical guarantees of correctness
