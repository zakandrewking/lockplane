> **⚠️ Experimental Project**
> This is an experimental project for AI coding experiments and should not be expected to work yet.

# Lockplane

A Postgres-first control plane for safe, AI-friendly schema management.

## Why Lockplane?

**Shadow DB validation catches problems before production.** Most tools roll back after failure. Lockplane tests migrations on a shadow database first, so bad plans never touch your real data. *(Implemented)*

**Every change is explainable.** See exactly what SQL runs, in what order, with clear descriptions. *(Implemented)*

**Rollbacks are generated and validated, not manually written.** For every forward migration, Lockplane computes the reverse operation and validates it works. *(Implemented)*

**Long-running operations will execute durably.** Building an index on 100M rows? Backfilling a column? Lockplane will handle timeouts, retries, and progress tracking so operations complete even if connections drop. *(Planned)*

---

**New to Lockplane?** See [Getting Started](docs/getting_started.md) for a guide to building your first app with Claude Code and Lockplane.

---

## Installation

### Download Pre-built Binary (Recommended)

1. Download the latest release for your platform from [GitHub Releases](https://github.com/zakandrewking/lockplane/releases/latest)
2. Extract the archive: `tar -xzf lockplane_*.tar.gz`
3. Move to your PATH: `sudo mv lockplane /usr/local/bin/`
4. Verify: `lockplane version`

### Build from Source

```bash
git clone https://github.com/zakandrewking/lockplane.git
cd lockplane
go install .
```

### Verify Installation

```bash
lockplane
lockplane version
lockplane help
```

---

## Quick Start

### The Lockplane Workflow

Lockplane follows a simple, declarative workflow:

1. **Define your desired schema** - Single source of truth in JSON
2. **Generate migration plan** - Lockplane calculates forward/reverse SQL
3. **Validate safety** - Ensures operations are safe and reversible
4. **Apply to database** - Execute with shadow DB validation

### Prerequisites
- Lockplane CLI (see Installation above)
- Docker & Docker Compose (for local Postgres)

### Setup

1. Prepare your Docker Compose file for Lockplane:
```bash
lockplane init docker-compose
```
This finds your `docker-compose.yml`, clones your primary Postgres service, and adds a `shadow` service on port `5433`.

2. Start Postgres:
```bash
docker compose up -d
```

### Example: Your First Migration

1. **Introspect current state** (empty database):
```bash
lockplane introspect > current.json
cat current.json
# Shows: {"tables": []}
```

2. **Define your desired schema** (`desired.json`):
```json
{
  "$schema": "https://raw.githubusercontent.com/zakandrewking/lockplane/main/schema-json/schema.json",
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

3. **Generate and validate migration plan**:
```bash
lockplane plan --from current.json --to desired.json --validate
```

Output:
```
=== Validation Results ===

✓ Validation 1: PASS
  - Table creation is always safe
  - Reversible: DROP TABLE users

✓ All operations are reversible
✓ All validations passed

{
  "steps": [
    {
      "description": "Create table users",
      "sql": "CREATE TABLE users (id integer NOT NULL, email text NOT NULL)"
    }
  ]
}
```

4. **Apply the migration**:
```bash
lockplane apply --plan migration.json
```

This automatically tests on shadow DB first, then applies to main DB if successful.

That's it! Your schema is now your single source of truth. Change it, generate a new plan, validate, and apply.

## Schema Definition with JSON

Define your desired database schema using JSON with JSON Schema validation for type safety and validation.

**Create a schema:**

```json
{
  "$schema": "https://raw.githubusercontent.com/zakandrewking/lockplane/main/schema-json/schema.json",
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
# Validate schema JSON directly
lockplane validate schema desired.json

# Validate by running a diff or plan command
lockplane diff current.json desired.json
```

**Why JSON + JSON Schema?**
- **Universal format** - Works with all tools and languages
- **IDE integration** - Autocomplete and validation in VS Code, IntelliJ, etc.
- **Straightforward** - No new syntax to learn
- **JSON Schema validation** - Enforces structure and constraints
- **Ecosystem** - Massive tooling support

See [examples/schemas-json/](./examples/schemas-json/) for examples. Replace `main` in the `$schema` URL with a specific tag (for example `v0.1.0`) to pin validation to an exact release.

## How It Works

### Single Source of Truth

Your desired schema is the single source of truth. Lockplane generates everything else on demand:

```bash
# Your desired schema
cat schema.json

# Current database state
lockplane introspect > current.json

# Forward migration (current → desired)
lockplane plan --from current.json --to schema.json --validate > forward.json

# Reverse migration (desired → current)
lockplane plan --from schema.json --to current.json --validate > reverse.json
```

**No migration files to maintain.** Just update your schema and regenerate plans as needed.

## Integrations

- [Lockplane with Prisma](docs/prisma.md)
- [Lockplane with Supabase](docs/supabase.md)
- [Lockplane with Alembic](docs/alembic.md)

### Complete Workflow

```bash
# 1. Introspect current database state
lockplane introspect > current.json

# 2. Update your desired schema
vim schema.json  # Your single source of truth

# 3. Generate and validate migration plan
lockplane plan --from current.json --to schema.json --validate > migration.json

# 4. Review the generated plan
cat migration.json

# 5. Apply the migration (validates on shadow DB first)
lockplane apply --plan migration.json
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

### Migration Validation

Lockplane validates that migrations are safe and reversible **before** they run:

```bash
# Validate a migration plan
lockplane plan --from current.json --to desired.json --validate
```

**Example: Safe migration** (nullable column):
```
✓ Validation 1: PASS
  - Column 'age' is nullable - safe to add
  - Reversible: DROP COLUMN users.age

✓ All operations are reversible
✓ All validations passed
```

**Example: Unsafe migration** (NOT NULL without DEFAULT):
```
✗ Validation 1: FAIL
  Error: Cannot add NOT NULL column 'email' without a DEFAULT value
  - NOT NULL columns require a DEFAULT value when added to tables with existing data
  - Reversible: DROP COLUMN users.email

❌ Validation FAILED: Some operations are not safe
```

**What validation checks:**
- ✅ **Safety**: Can this operation be executed without breaking existing data?
- ✅ **Reversibility**: Can we generate a safe rollback?
- ✅ **NOT NULL constraints**: Requires DEFAULT values for existing rows
- 🔄 **More checks coming**: Type compatibility, data preservation, etc.

### Supported Operations

The plan generator handles:
- ✅ **Add/remove tables**
- ✅ **Add/remove columns** (with validation)
- ✅ **Modify column types, nullability, defaults**
- ✅ **Add/remove indexes**
- ✅ **Safe operation ordering** (adds before drops, tables before indexes)

### CLI Commands

```bash
# Compare two schemas (see diff)
lockplane diff before.json after.json

# Generate migration plan (with validation)
lockplane plan --from before.json --to after.json --validate

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
  "$schema": "https://raw.githubusercontent.com/zakandrewking/lockplane/main/schema-json/plan.json",
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
For reproducible validation, swap `main` in the `$schema` URL with a tagged release such as `v0.1.0`.

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

### Core Features

**Schema Management**
- ✅ Schema introspection (reads Postgres catalog) - _3 tests_
- ✅ JSON Schema definition with validation - _2 tests_
- ✅ Diff engine (compares schemas) - _7 tests_
- ✅ Foreign key support - _2 tests_

**Migration Planning**
- ✅ Automatic plan generator (generates SQL from diffs) - _11 tests_
- ✅ Automatic rollback generator (reverse migrations) - _10 tests_
- ✅ Migration validation (safety checks) - _12 tests_
  - ✅ NOT NULL without DEFAULT detection
  - ✅ Foreign key validation
  - ✅ Reversibility verification
  - ⚠️ Column type changes (partial validation)
  - ❌ Data preservation checks (planned)
  - ❌ Index operation validation (planned)

**Migration Execution**
- ✅ Transactional executor - _4 tests_
- ✅ Shadow DB validation (dry-run testing) - _tested_
- ✅ Error tracking with step-level failures
- ❌ Durable execution (timeouts, retries, progress tracking)
- ❌ Advisory locks during apply
- ❌ Two-phase confirmation for destructive operations

**CLI Commands**
- ✅ `introspect` - Export current schema to JSON
- ✅ `diff` - Compare two schema files
- ✅ `plan` - Generate migration plan with validation
- ✅ `rollback` - Generate reverse migration
- ✅ `apply` - Execute migration plan
- ✅ `validate` - Validate schema or plan files
- ✅ `init` - Setup Docker Compose with shadow DB - _3 tests_
- ✅ `version` - Show version info

**Supported Operations**
- ✅ Create/drop tables
- ✅ Add/drop columns (with validation)
- ✅ Modify column types, nullability, defaults
- ✅ Add/drop indexes (unique and non-unique)
- ✅ Add/drop foreign keys
- ❌ Sequences and serial columns (partial)
- ❌ Check constraints
- ❌ Triggers and functions
- ❌ Row-level security (RLS) policies
- ❌ Partitioned tables

**Infrastructure & Integration**
- ✅ Docker Compose setup (main + shadow DB)
- ❌ MCP server interface for AI agents
- ❌ Catalog hash computation and ledger
- ❌ pgroll integration for zero-downtime migrations
- ❌ Prisma schema converter
- ❌ Alembic migration converter

**Test Coverage:** 53 tests across all core features
