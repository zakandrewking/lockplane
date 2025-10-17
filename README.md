# Lockplane

A Postgres-first control plane for safe, AI-friendly schema management.

## Why Lockplane?

**Shadow DB validation catches problems before production.** Most tools roll back after failure. Lockplane tests migrations on a shadow database first, so bad plans never touch your real data. *(Implemented)*

**Every change is explainable.** See exactly what SQL runs, in what order, with clear descriptions. *(Implemented)*

**Rollbacks are generated and validated, not manually written.** For every forward migration, Lockplane computes the reverse operation and validates it works. *(Implemented)*

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

The introspector will connect to Postgres and output the current schema as JSON.

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
lockplane introspect > current.json
cat current.json
```

## Schema Definition with JSON

Define your desired database schema using JSON with JSON Schema validation for type safety and validation.

**Create a schema:**

```json
{
  "$schema": "./schema-json/schema.json",
  "tables": [
    {
      "name": "users",
      "columns": [
        {
          "name": "id",
          "type": "integer",
          "nullable": false,
          "default": "nextval('users_id_seq'::regclass)",
          "is_primary_key": true
        },
        {
          "name": "email",
          "type": "text",
          "nullable": false,
          "is_primary_key": false
        },
        {
          "name": "created_at",
          "type": "timestamp without time zone",
          "nullable": true,
          "default": "now()",
          "is_primary_key": false
        }
      ]
    },
    {
      "name": "posts",
      "columns": [
        {
          "name": "id",
          "type": "integer",
          "nullable": false,
          "default": "nextval('posts_id_seq'::regclass)",
          "is_primary_key": true
        },
        {
          "name": "user_id",
          "type": "integer",
          "nullable": false,
          "is_primary_key": false
        },
        {
          "name": "title",
          "type": "text",
          "nullable": false,
          "is_primary_key": false
        },
        {
          "name": "created_at",
          "type": "timestamp without time zone",
          "nullable": true,
          "default": "now()",
          "is_primary_key": false
        }
      ],
      "indexes": [
        {
          "name": "idx_posts_user_id",
          "columns": ["user_id"],
          "unique": false
        }
      ]
    }
  ]
}
```

**Validate:**

Most editors with JSON Schema support will automatically validate your schema files. You can also validate manually:

```bash
# Validate by running a diff or plan command
lockplane diff current.json desired.json
```

**Why JSON + JSON Schema?**
- **Universal format** - Works with all tools and languages
- **IDE integration** - Autocomplete and validation in VS Code, IntelliJ, etc.
- **Straightforward** - No new syntax to learn
- **JSON Schema validation** - Enforces structure and constraints
- **Ecosystem** - Massive tooling support

See [examples/schemas-json/](./examples/schemas-json/) for examples.

## Automatic Plan Generation

Lockplane can automatically generate migration plans by comparing two schemas.

### Complete Workflow

```bash
# 1. Introspect current database state
lockplane introspect > current.json

# 2. Define your desired schema
# Edit desired.json with your target schema

# 3. Generate a migration plan
lockplane plan --from current.json --to desired.json > migration.json

# 4. Review the generated plan
cat migration.json
```

### Example

Given two schemas:

**Before** (`current.json`):
```json
{
  "tables": [
    {
      "name": "users",
      "columns": [
        {
          "name": "id",
          "type": "integer",
          "nullable": false,
          "is_primary_key": true
        },
        {
          "name": "email",
          "type": "text",
          "nullable": false,
          "is_primary_key": false
        }
      ]
    }
  ]
}
```

**After** (`desired.json`):
```json
{
  "tables": [
    {
      "name": "users",
      "columns": [
        {
          "name": "id",
          "type": "integer",
          "nullable": false,
          "is_primary_key": true
        },
        {
          "name": "email",
          "type": "text",
          "nullable": false,
          "is_primary_key": false
        },
        {
          "name": "age",
          "type": "integer",
          "nullable": true,
          "is_primary_key": false
        }
      ]
    },
    {
      "name": "posts",
      "columns": [
        {
          "name": "id",
          "type": "integer",
          "nullable": false,
          "is_primary_key": true
        },
        {
          "name": "title",
          "type": "text",
          "nullable": false,
          "is_primary_key": false
        }
      ]
    }
  ]
}
```

**Generated plan**:
```bash
lockplane plan --from current.json --to desired.json
```

```json
{
  "steps": [
    {
      "description": "Create table posts",
      "sql": "CREATE TABLE posts (id integer NOT NULL, title text NOT NULL)"
    },
    {
      "description": "Add column age to table users",
      "sql": "ALTER TABLE users ADD COLUMN age integer"
    }
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
lockplane diff before.json after.json

# Generate migration plan
lockplane plan --from before.json --to after.json

# Generate rollback plan
lockplane rollback --plan forward.json --from before.json
```

## Automatic Rollback Generation

Lockplane can automatically generate rollback plans that reverse forward migrations.

### How It Works

Given a forward migration plan and the original schema state, Lockplane generates the exact reverse operations needed to undo the migration:

```bash
# 1. Generate forward migration
lockplane plan --from current.json --to desired.json > forward.json

# 2. Generate rollback migration
lockplane rollback --plan forward.json --from current.json > rollback.json
```

### Example

**Forward migration** (before → after):
```json
{
  "steps": [
    {
      "description": "Create table posts",
      "sql": "CREATE TABLE posts (id integer NOT NULL, title text NOT NULL)"
    },
    {
      "description": "Add column age to table users",
      "sql": "ALTER TABLE users ADD COLUMN age integer"
    },
    {
      "description": "Create index idx_users_email",
      "sql": "CREATE UNIQUE INDEX idx_users_email ON users (email)"
    }
  ]
}
```

**Generated rollback** (after → before):
```json
{
  "steps": [
    {
      "description": "Rollback: Drop index idx_users_email",
      "sql": "DROP INDEX idx_users_email"
    },
    {
      "description": "Rollback: Drop column age from table users",
      "sql": "ALTER TABLE users DROP COLUMN age"
    },
    {
      "description": "Rollback: Drop table posts",
      "sql": "DROP TABLE posts CASCADE"
    }
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

Migration plans are JSON files with a series of SQL steps:

```json
{
  "$schema": "./schema-json/plan.json",
  "steps": [
    {
      "description": "Create posts table",
      "sql": "CREATE TABLE posts (id SERIAL PRIMARY KEY, title TEXT NOT NULL)"
    },
    {
      "description": "Add index on title",
      "sql": "CREATE INDEX idx_posts_title ON posts(title)"
    }
  ]
}
```

See example plans in `examples/schemas-json/` and `testdata/plans-json/`.

### Using the Executor

The executor provides:
- **Transactional execution** - All steps succeed or all roll back
- **Shadow DB validation** - Test migrations before applying to main DB
- **Error tracking** - Detailed failure reporting

Example usage in Go:

```go
// Load migration plan from JSON
plan, _ := LoadJSONPlan("testdata/plans-json/create_table.json")

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
- ✅ Schema introspector with JSON output
- ✅ JSON Schema for defining desired schemas with validation
- ✅ Diff engine to compare schemas
- ✅ **Automatic plan generator** - generates SQL migrations from schema diffs
- ✅ **Automatic rollback generator** - generates reverse migrations automatically
- ✅ Shadow DB setup for dry-run validation
- ✅ Transactional migration executor with JSON plan format
- ✅ Golden test suite with JSON fixtures
- ✅ CLI commands: `introspect`, `diff`, `plan`, `rollback`

**Planned:**
- Durable execution for long-running operations
- MCP server interface for AI agents
- Catalog hash computation and ledger
- pgroll integration for zero-downtime migrations

## Current Limitations

**Introspector gaps:** Currently captures tables, columns, and indexes. Foreign keys, check constraints, and full index column parsing are not yet implemented.

**Development-only migrations:** All DDL runs in a transaction. Zero-downtime migrations with pgroll are planned for production use.
