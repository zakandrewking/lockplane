# Lockplane

A Postgres-first control plane for safe, AI-friendly schema management.

## Why Lockplane?

**Shadow DB validation catches problems before production.** Most tools roll back after failure. Lockplane tests migrations on a shadow database first, so bad plans never touch your real data. *(Implemented)*

**Every change is explainable.** See exactly what SQL runs, in what order, with clear descriptions. *(Implemented)*

**Rollbacks will be generated and validated, not manually written.** For every forward migration, Lockplane will compute the reverse operation and validate it works. *(Planned)*

**Long-running operations will execute durably.** Building an index on 100M rows? Backfilling a column? Lockplane will handle timeouts, retries, and progress tracking so operations complete even if connections drop. *(Planned)*

---

**New to Lockplane?** See [GETTING_STARTED.md](./GETTING_STARTED.md) for a guide to building your first app with Claude Code and Lockplane.

---

## Installation

### Binary Releases (Recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/lockplane/lockplane/releases):

**Linux / macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/lockplane/lockplane/main/install.sh | bash
```

**Manual Installation:**
1. Download the appropriate binary for your OS from [releases](https://github.com/lockplane/lockplane/releases/latest)
2. Extract the archive: `tar -xzf lockplane_*.tar.gz`
3. Move to your PATH: `sudo mv lockplane /usr/local/bin/`
4. Verify: `lockplane version`

**Homebrew (macOS/Linux):**
```bash
brew install lockplane/tap/lockplane
```

**From Source:**
```bash
git clone https://github.com/lockplane/lockplane.git
cd lockplane
go install .
```

### Verify Installation

```bash
lockplane version
lockplane help
```

---

## Quick Start

### Prerequisites
- Lockplane CLI (see Installation above)
- Docker & Docker Compose (for local Postgres)

### Setup

1. Start Postgres:
```bash
docker compose up -d
```

2. Run the introspector:
```bash
lockplane introspect
```

The introspector will connect to Postgres and output the current schema as CUE. To output JSON instead, use:
```bash
lockplane introspect --format json
```

### Example: Create a test schema

```bash
docker compose exec pg psql -U lockplane -d lockplane -c "
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT NOW()
);
"
```

Then run the introspector again to see the schema:
```bash
lockplane introspect
```

## Schema Definition with CUE

Define your desired database schema using CUE for type safety, validation, and IDE support.

**Create a schema:**

```cue
// schema/myapp.cue
package myapp

import "github.com/lockplane/lockplane/schema"

schema.#Schema & {
	tables: [users, posts]
}

users: schema.#Table & {
	name: "users"
	columns: [
		schema.#ID,           // Reusable patterns
		schema.#Email,
		schema.#CreatedAt,
	]
}

posts: schema.#Table & {
	name: "posts"
	columns: [
		schema.#ID,
		{name: "user_id", type: "integer", nullable: false},
		{name: "title", type: "text", nullable: false},
		schema.#CreatedAt,
	]
	indexes: [
		{name: "idx_posts_user_id", columns: ["user_id"], unique: false},
	]
}
```

**Validate:**

```bash
# Validate (IDE does this automatically)
cue vet schema/myapp.cue

# Export to JSON (optional, for legacy tools)
go run cmd/cue-export/main.go -cue schema/myapp.cue -json desired_schema.json
```

**Why CUE?**
- **IDE integration** - Autocomplete, inline errors, type checking
- **Reusable components** - Define column patterns once
- **Built-in validation** - Snake_case names, valid types, constraints
- **Type safety** - Catch errors before runtime

See [schema/README.md](./schema/README.md) for full CUE documentation and [examples/schemas/](./examples/schemas/) for examples.

## Automatic Plan Generation

Lockplane can automatically generate migration plans by comparing two schemas.

### Complete Workflow

```bash
# 1. Define your desired schema in CUE
# schema/myapp.cue

# 2. Introspect current database state
lockplane introspect > current.cue

# 3. Generate a migration plan
lockplane plan --from current.cue --to schema/myapp.cue > migration.cue

# 4. Review the generated plan
cat migration.cue
```

### Example

Given two schemas:

**Before** (`current.cue`):
```cue
schema.#Schema & {
	tables: [{
		name: "users"
		columns: [
			{name: "id", type: "integer", nullable: false, is_primary_key: true},
			{name: "email", type: "text", nullable: false},
		]
	}]
}
```

**After** (`desired.cue`):
```cue
schema.#Schema & {
	tables: [
		{
			name: "users"
			columns: [
				{name: "id", type: "integer", nullable: false, is_primary_key: true},
				{name: "email", type: "text", nullable: false},
				{name: "age", type: "integer", nullable: true},
			]
		},
		{
			name: "posts"
			columns: [
				{name: "id", type: "integer", nullable: false, is_primary_key: true},
				{name: "title", type: "text", nullable: false},
			]
		},
	]
}
```

**Generated plan**:
```bash
lockplane plan --from current.cue --to desired.cue
```

```cue
schema.#Plan & {
	steps: [
		{
			description: "Create table posts"
			sql:         "CREATE TABLE posts (...)"
		},
		{
			description: "Add column age to table users"
			sql:         "ALTER TABLE users ADD COLUMN age integer"
		},
	]
}
```

### Supported Operations

The plan generator handles:
- ✅ **Add/remove tables**
- ✅ **Add/remove columns**
- ✅ **Modify column types, nullability, defaults**
- ✅ **Add/remove indexes**
- ✅ **Safe operation ordering** (adds before drops, tables before indexes)

### CLI Commands

```bash
# Compare two schemas (see diff)
lockplane diff before.cue after.cue

# Generate migration plan
lockplane plan --from before.cue --to after.cue

# Generate rollback plan
lockplane rollback --plan forward.cue --from before.cue

# Output formats (--format flag)
--format cue   # CUE output (default)
--format json  # JSON output
```

## Automatic Rollback Generation

Lockplane can automatically generate rollback plans that reverse forward migrations.

### How It Works

Given a forward migration plan and the original schema state, Lockplane generates the exact reverse operations needed to undo the migration:

```bash
# 1. Generate forward migration
lockplane plan --from current.cue --to desired.cue > forward.cue

# 2. Generate rollback migration
lockplane rollback --plan forward.cue --from current.cue > rollback.cue
```

### Example

**Forward migration** (before → after):
```cue
schema.#Plan & {
  steps: [
    {
      description: "Create table posts"
      sql:         "CREATE TABLE posts (...)"
    },
    {
      description: "Add column age to table users"
      sql:         "ALTER TABLE users ADD COLUMN age integer"
    },
    {
      description: "Create index idx_users_email"
      sql:         "CREATE UNIQUE INDEX idx_users_email ON users (email)"
    },
  ]
}
```

**Generated rollback** (after → before):
```cue
schema.#Plan & {
  steps: [
    {
      description: "Rollback: Drop index idx_users_email"
      sql:         "DROP INDEX idx_users_email"
    },
    {
      description: "Rollback: Drop column age from table users"
      sql:         "ALTER TABLE users DROP COLUMN age"
    },
    {
      description: "Rollback: Drop table posts"
      sql:         "DROP TABLE posts CASCADE"
    },
  ]
}
```

### Supported Rollback Operations

All forward operations have corresponding rollbacks:
- ✅ **CREATE TABLE** → DROP TABLE CASCADE
- ✅ **DROP TABLE** → CREATE TABLE (reconstructed from schema)
- ✅ **ADD COLUMN** → DROP COLUMN
- ✅ **DROP COLUMN** → ADD COLUMN (restored with original definition)
- ✅ **ALTER TYPE** → ALTER TYPE (back to original)
- ✅ **SET/DROP NOT NULL** → Reversed
- ✅ **SET/DROP DEFAULT** → Restored to original value
- ✅ **CREATE INDEX** → DROP INDEX
- ✅ **DROP INDEX** → CREATE INDEX (reconstructed)

### Rollback Safety

- Operations are reversed in the correct order (last-in, first-out)
- Requires the original "before" schema to reconstruct dropped objects
- Each rollback step is validated for correctness
- Rollbacks can be tested on shadow DB before production use

## Migration Executor

Lockplane includes a transactional migration executor that safely applies schema changes.

### Plan Format

Migration plans are CUE files with a series of SQL steps:

```cue
// plan.cue
package myplan

import "github.com/lockplane/lockplane/schema"

schema.#Plan & {
	steps: [
		{
			description: "Create posts table"
			sql:         "CREATE TABLE posts (id SERIAL PRIMARY KEY, title TEXT NOT NULL)"
		},
		{
			description: "Add index on title"
			sql:         "CREATE INDEX idx_posts_title ON posts(title)"
		},
	]
}
```

See example plans in `testdata/plans/`:
- `create_table.cue` - Create a new table
- `add_column.cue` - Add columns with constraints

### Using the Executor

The executor provides:
- **Transactional execution** - All steps succeed or all roll back
- **Shadow DB validation** - Test migrations before applying to main DB
- **Error tracking** - Detailed failure reporting

Example usage in Go:

```go
// Load migration plan from CUE
plan, _ := LoadCUEPlan("testdata/plans/create_table.cue")

// Apply with shadow DB validation
shadowDB, _ := sql.Open("postgres", shadowConnStr)
result, err := applyPlan(ctx, mainDB, plan, shadowDB)

if result.Success {
    fmt.Printf("Applied %d steps successfully\n", result.StepsApplied)
} else {
    fmt.Printf("Failed: %v\n", result.Errors)
}
```

### Testing Migrations

Run the test suite to see the executor in action:

```bash
go test -v -run TestApplyPlan
```

### Environment Variables

- `DATABASE_URL` - Main Postgres connection string (default: `postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable`)
- `SHADOW_DATABASE_URL` - Shadow DB for dry-run validation (default: `postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable`)

## Project Status

Currently implementing M1 (DSL & Planner). See `0001-design.md` for full design.

**Completed (M1 - DSL & Planner):**
- ✅ Schema introspector with CUE output (JSON also available)
- ✅ CUE DSL for defining desired schemas with validation
- ✅ Diff engine to compare schemas
- ✅ **Automatic plan generator** - generates SQL migrations from schema diffs
- ✅ **Automatic rollback generator** - generates reverse migrations automatically
- ✅ Shadow DB setup for dry-run validation
- ✅ Transactional migration executor with CUE plan format
- ✅ Golden test suite with CUE fixtures
- ✅ CLI commands: `introspect`, `diff`, `plan`, `rollback`

**Planned:**
- Durable execution for long-running operations
- MCP server interface for AI agents
- Catalog hash computation and ledger
- pgroll integration for zero-downtime migrations

## Current Limitations

**Introspector gaps:** Currently captures tables, columns, and indexes. Foreign keys, check constraints, and full index column parsing are not yet implemented.

**Development-only migrations:** All DDL runs in a transaction. Zero-downtime migrations with pgroll are planned for production use.
