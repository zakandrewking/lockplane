> **⚠️ Experimental**
> This is an experiment in vibe coding so don't expect anything reliable or production-ready.

# Lockplane

[![Test](https://github.com/zakandrewking/lockplane/actions/workflows/test.yml/badge.svg)](https://github.com/zakandrewking/lockplane/actions/workflows/test.yml)
[![codecov](https://codecov.io/github/zakandrewking/lockplane/graph/badge.svg?token=JP0QINP1G1)](https://codecov.io/github/zakandrewking/lockplane)

A control plane for safe, AI-friendly schema management. Works with PostgreSQL, SQLite, and Turso.

## Why Lockplane?

**Shadow DB validation catches problems before production.** Lockplane tests
migrations on a shadow database first, so bad plans never touch your real data.

**Breaking change detection prevents data loss.** Lockplane automatically
identifies dangerous operations (dropping columns, type narrowing, etc.) and
suggests safer alternatives before you deploy.

**Every change is explainable.** See exactly what SQL runs, in what order, with
clear descriptions and safety classifications (Safe, Review, Lossy, Dangerous).

**Rollbacks are generated and validated, not manually written.** For every
forward migration, Lockplane computes the reverse operation, validates it
works, and warns if rollback will lose data.

**Guarantees safety.** Lockplane validates migrations, only runs migrations
against expected database state, safely rolls back every time.

**Long-running operations are executed durably.** Building an index on 100M
rows? Backfilling a column? Lockplane will handle timeouts, retries, and
progress tracking so operations complete even if connections drop.

---

Get started by following these steps:

## 1. 📦 Installation

### Download Pre-built Binary

1. Download the latest release for your platform from [GitHub
   Releases](https://github.com/zakandrewking/lockplane/releases/latest)
2. Extract the archive: `tar -xzf lockplane_*.tar.gz`
3. Move to your PATH: `sudo mv lockplane /usr/local/bin/`
4. Verify: `lockplane version`

For more options, see the [Installation Guide](docs/installation.md).

## 2. 🤖 Optional: Use with AI Assistants

**Claude Code Users**: There's a Lockplane plugin that provides expert knowledge
about Lockplane commands, workflows, and best practices!

```bash
/plugin install lockplane@lockplane-tools
```

The plugin automatically helps with:
- Schema migration planning
- Safety validation
- SQL generation
- Rollback strategies
- Best practices

[Learn more about the plugin →](claude-plugin/README.md)

**Other AI Assistants**: See
[llms.txt](https://github.com/zakandrewking/lockplane/blob/main/llms.txt) for
comprehensive Lockplane context.

## 3. 🚀 Create your first schema

With Lockplane, you describe your desired database schema with special SQL files
that end in `.lp.sql`. These files contain normal valid SQL DDL (supporting
either PostgreSQL or SQLite dialects), but with an extra level of strictness to
guarantee that your schema is safe to use.

> **Dialect hint**
>
> Lockplane parses `.lp.sql` files with PostgreSQL rules by default.
> To treat a file as SQLite DDL, add a comment at the top:
>
> ```sql
> -- dialect: sqlite
> CREATE TABLE todos (
>   id INTEGER PRIMARY KEY,
>   completed INTEGER NOT NULL DEFAULT 0,
>   created_at TEXT NOT NULL DEFAULT (datetime('now'))
> );
> ```
>
> When you supply SQLite connection strings (e.g., `sqlite://…`) Lockplane
> automatically uses SQLite parsing for introspection and planning.

Let's get started with an example. Create a new directory called `schema/` at
the root of your project, and create a new file called `users.lp.sql`. Add the
following SQL to the file:

```sql
CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

This file defines a simple `users` table with an `id`, `email`, and `created_at`
column.

Now, let's validate our schema:

```bash
lockplane validate sql schema/users.lp.sql
```

This will output a report of any issues with your schema. If there are no issues,
you'll see a message like this:

```
✓ Validation 1: PASS
```

NOTE: We also provide a VSCode extension that will validate your schema files for you when you save them! See [vscode-lockplane/README.md](vscode-lockplane/README.md) for more information.

Lockplane schema files cannot contain any dangerous SQL statements, or any
statements that are not "declarative". For example, you cannot use `DROP TABLE`
or `DROP COLUMN` statements -- statements like this make sense when you are
interacting with a live database, but for schema definition we want to focus on
creating a structure. Later, we'll see how lockplane can drop tables and columns
when needed.

For more information on `.lp.sql` files, run `lockplane validate sql --help`.

## 4. 📜 Run your first migration

Now that we have a schema, we can generate a migration plan to apply it to our
database.

### Quick Setup: Interactive Wizard

The easiest way to get started is with the interactive wizard:

```bash
lockplane init
```

The wizard will guide you through:
- **Database type selection**: Choose PostgreSQL, SQLite, or libSQL/Turso
- **Connection details**: Enter your database credentials with smart defaults
- **Connection testing**: Verify your database is reachable before proceeding
- **Environment setup**: Configure multiple environments (local, staging, production)
- **Shadow DB configuration**: Automatically set up shadow databases for safe migrations (PostgreSQL only)
- **File generation**: Creates `schema/lockplane.toml` and `.env.*` files with secure permissions

**Features:**
- Detects existing configurations and offers to add new environments
- Auto-configures SSL mode (disabled for localhost, required for remote hosts)
- Tests connections before saving to catch errors early
- Updates `.gitignore` to protect your credentials
- Provides clear next steps after setup

**Non-interactive mode** (for CI/scripts):
```bash
lockplane init --yes  # Use all defaults
```

### Manual Configuration (Alternative)

If you prefer manual setup, create a `schema/lockplane.toml` file.
You can also use the sample at `lockplane.toml.example` as a starting point.

```toml
default_environment = "local"

[environments.local]
description = "Local development database"
schema_path = "."
```

Next, provide the actual credentials in `.env.local` (ignored by Git by default).

#### Example: PostgreSQL

```bash
cat <<'EOF' > .env.local
DATABASE_URL=postgresql://user:password@localhost:5432/myapp?sslmode=disable
SHADOW_DATABASE_URL=postgresql://user:password@localhost:5433/myapp_shadow?sslmode=disable
EOF
```

#### Example: SQLite

```bash
cat <<'EOF' > .env.local
# Use a file-based SQLite database
DATABASE_URL=sqlite://./myapp.db
SHADOW_DATABASE_URL=sqlite://./myapp_shadow.db
EOF
```

#### Example: Turso

```bash
cat <<'EOF' > .env.local
# Use Turso remote SQLite databases
DATABASE_URL=libsql://mydb-user.turso.io?authToken=eyJhbGc...
SHADOW_DATABASE_URL=libsql://mydb-shadow-user.turso.io?authToken=eyJhbGc...
EOF
```

Lockplane automatically loads `.env.<name>` for the selected environment (for the
default environment, `.env.local`). You can still override any value with CLI flags
such as `--target` or `--shadow-db` when needed.

### Apply the migration

Now, we can generate a migration plan to apply our schema to our database with the following command:

```bash
lockplane apply --auto-approve --target-environment local --schema schema/
```

This will introspect the target database, generate a migration plan, and apply it immediately (with shadow database validation).

## 5. 🔍 Making a change

Now, let's make a change to our schema. Let's add a new column to the `users`
table called `age`. Update your `users.lp.sql` file to add the new column:

```sql
CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  age INTEGER NOT NULL DEFAULT 0
);
```

Now to apply the change, we can run the following command:

```bash
lockplane apply --auto-approve --target-environment local --schema schema/
```

And that's it! You've successfully made a change to your schema and applied it to your database.

## 6. ✅ Final environment check

Before handing the project to teammates or automations:

- Commit `lockplane.toml` with the environments you expect everyone to use.
- Keep sensitive credentials in `.env.<name>` files (add them to `.gitignore`) and share sanitized samples such as `.env.local.example`.
- Record which environment maps to each deployed database (local, staging, production) so the `--*-environment` flags stay meaningful.
- When automating (CI, release pipelines), copy the appropriate `.env.<name>` file onto the runner or provide explicit `--target` / `--from` flags.

Lockplane always prefers explicit CLI values, so you can temporarily override connections without touching the shared environment files.

## Configuration

Lockplane resolves configuration in this order:
1. Explicit CLI flags (`--target`, `--target-environment`, `--from`, `--from-environment`, etc.)
2. Named environments from `lockplane.toml` (plus `.env.<name>` overlays)
3. Built-in defaults for local development

### Configuration discovery

When you run a Lockplane command, it searches for `lockplane.toml` starting from the
current working directory and walking up parent directories. For convenience the
search also checks each directory's `schema/` subdirectory, so the default project
layout looks like this:

```
my-app/
├── schema/
│   ├── lockplane.toml    # configuration discovered here
│   ├── 001_users.lp.sql
│   └── 002_notes.lp.sql
└── cmd/
    └── main.go
```

From `schema/lockplane.toml` you can reference `.env.<name>` files located either next
to the config file or in the project root. CLI flags still take precedence, so you can
override connections temporarily without moving the config file.

### `lockplane.toml`

Use this file to define environments shared across your team:

```toml
default_environment = "local"
schema_path = "schema/"

[environments.local]
description = "Local development"

[environments.staging]
description = "Managed staging database"
# Values pulled from .env.staging
```

### `.env.<environment>` files

Store credentials in `.env.local`, `.env.staging`, etc. Lockplane reads these files
automatically based on the selected environment. Each file should define:

```
DATABASE_URL=postgresql://user:password@host:5432/db?sslmode=disable
SHADOW_DATABASE_URL=postgresql://user:password@host:5433/db_shadow?sslmode=disable
```

### CLI overrides

Flags override any configured environment:

```bash
# Override target connection once-off
lockplane apply plan.json \
  --target-environment staging \
  --target "postgresql://override@host/db" \
  --shadow-db "postgresql://override@host/db_shadow"
```

**Supported database formats:**
- PostgreSQL: `postgres://` or `postgresql://`
- SQLite: `file:path/to/db.sqlite`, `path/to/db.db`, or `:memory:`
- Turso/libSQL: `libsql://[DATABASE].turso.io?authToken=[TOKEN]`

## Schema Definition Formats

Lockplane accepts both SQL DDL (`.lp.sql`) and JSON schema files. Authoring `.lp.sql` is the preferred workflow—it's easy to review, copy/paste into PRs, and matches the SQL your database understands. JSON remains fully supported for tooling and automation.

### Preferred: `.lp.sql`

```sql
-- schema.lp.sql
CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX users_email_key ON users(email);
```

### Alternate: JSON

If you need JSON (for example, to integrate with existing tooling), convert on demand:

```bash
lockplane convert --input schema.lp.sql --output schema.json
lockplane convert --input schema.json --output schema.lp.sql --to sql
```

Editors that support JSON Schema validation can point at `schema-json/schema.json` for autocomplete when working in JSON. See [examples/schemas-json/](./examples/schemas-json/) for reference files.

### Organizing Multiple Files

Prefer keeping related DDL in separate `.lp.sql` files? Point Lockplane at the directory:

```bash
# Combine all .lp.sql files in a directory into a single schema (non-recursive)
lockplane plan --from current.json --to schema/ --validate
lockplane convert --input schema/ --output schema.json
```

Files are read in lexicographic order, so you can prefix them with numbers (for example `001_tables.lp.sql`, `010_indexes.lp.sql`) to make the order explicit. Only top-level files are considered—subdirectories and symlinks are skipped to avoid accidental recursion.

## Schema Validation

Lockplane provides comprehensive validation for schema and plan files to catch errors early.

### Validating SQL Schemas (`.lp.sql`)

```bash
# Validate SQL schema file
lockplane validate sql schema.lp.sql

# Validate with JSON output (for IDE integration)
lockplane validate sql --format json schema.lp.sql

# Validate directory of SQL files
lockplane validate sql lockplane/schema/
```

**What's validated:**

1. **SQL Syntax** (statement-by-statement)
   - Uses the same PostgreSQL parser as the database itself (via libpg_query)
   - Detects multiple syntax errors in a single pass
   - Reports exact line numbers for each error

2. **Schema Structure**
   - Duplicate column names
   - Missing data types
   - Missing primary keys (warning)
   - Invalid foreign key references (non-existent tables or columns)
   - Duplicate index names
   - Invalid index column references

3. **Dangerous Patterns** (data loss risks)
   - `DROP TABLE` - Permanently deletes data
   - `DROP COLUMN` - Irreversible data loss
   - `TRUNCATE TABLE` - Deletes all rows
   - `DELETE` without `WHERE` clause - Unintentional data deletion

4. **Non-Declarative Patterns** (imperative SQL not allowed in schema files)
   - `IF NOT EXISTS` clauses - Makes schema non-deterministic
   - Transaction control (`BEGIN`, `COMMIT`, `ROLLBACK`) - Lockplane manages transactions
   - `CREATE OR REPLACE` - Use plain `CREATE` statements instead

**Example output:**
```
✗ SQL syntax errors in schema.lp.sql:

  Line 5: syntax error at or near "SERIAL"
  Line 19: syntax error at or near "TEXT"

Found 2 syntax error(s). Please fix these before running validation.
```

### Validating JSON Schemas (`.json`)

```bash
# Validate JSON schema file
lockplane validate schema schema.json
```

**What's validated:**
- JSON syntax (must be valid JSON)
- Structure matches Lockplane JSON Schema (`schema-json/schema.json`)
- All required fields are present
- Data types are correct

### Validating Migration Plans

```bash
# Validate migration plan file
lockplane validate plan migration.json

# Validate with JSON output
lockplane validate plan --format json migration.json
```

**What's validated:**
- JSON syntax
- Structure matches Lockplane plan schema (`schema-json/plan.json`)
- All migration steps are well-formed
- SQL statements in steps are present and non-empty

### IDE Integration

The `--format json` flag outputs structured validation results for IDE integration. The [VSCode Lockplane extension](vscode-lockplane/) uses this to show real-time validation errors as you type.

```json
{
  "valid": false,
  "issues": [
    {
      "file": "schema.lp.sql",
      "line": 5,
      "column": 1,
      "severity": "error",
      "message": "syntax error at or near \"SERIAL\"",
      "code": "syntax_error"
    }
  ]
}
```

## How It Works

### Single Source of Truth

Your desired schema is the single source of truth. Lockplane generates everything else on demand:

```bash
# Your desired schema
cat schema.lp.sql

# Current database state
lockplane introspect > current.json

# Forward migration (current → desired)
lockplane plan --from current.json --to schema.lp.sql --validate > forward.json

# Reverse migration (desired → current)
lockplane plan --from schema.lp.sql --to current.json --validate > reverse.json
```

**No migration files to maintain.** Just update your schema and regenerate plans as needed.

### Using Database Connection Strings

Instead of introspecting to a file, you can use database connection strings directly with `plan`, `apply`, and `rollback` commands. Lockplane will automatically introspect the database when it detects a connection string.

**Supported connection string formats:**
- PostgreSQL: `postgres://user:pass@host:port/dbname` or `postgresql://...`
- SQLite: `sqlite://path/to/db.db`, `file:path/to/db.db`, or `:memory:`

**Examples:**

```bash
# Compare two live databases
lockplane plan \
  --from postgres://user:pass@localhost:5432/production \
  --to postgres://user:pass@localhost:5433/staging \
  --validate > migration.json

# Compare live database to schema file
lockplane plan \
  --from-environment local \
  --to schema.lp.sql \
  --validate > migration.json

# Auto-approve: plan and apply directly to database
lockplane apply \
  --auto-approve \
  --target-environment local \
  --schema schema/

# Generate rollback using live database state
lockplane rollback \
  --plan migration.json \
  --from-environment local > rollback.json
```

This is especially useful for:
- **CI/CD pipelines**: Compare production state directly without intermediate files
- **Multi-environment workflows**: Diff staging vs production databases
- **Quick checks**: Skip the introspect step when working with live databases

**Note:** When using connection strings, Lockplane uses the same credentials and permissions as your application. Make sure the database user has appropriate read permissions for introspection.

## Integrations

- [Lockplane with SQLAlchemy](docs/sqlalchemy.md) - Python ORM integration
- [Lockplane with Prisma](docs/prisma.md) - TypeScript/JavaScript ORM integration
- [Lockplane with Supabase](docs/supabase.md) - Supabase project integration
- [Lockplane with Alembic](docs/alembic.md) - Migrating from Alembic to Lockplane

### Complete Workflow

**Two-step approach (traditional):**
```bash
# 1. Introspect current database state
lockplane introspect > current.json

# 2. Update your desired schema
vim schema.lp.sql  # Your single source of truth

# 3. Generate and validate migration plan
lockplane plan --from current.json --to schema.lp.sql --validate > migration.json

# 4. Review the generated plan
cat migration.json

# 5. Apply the migration (validates on shadow DB first)
lockplane apply migration.json
```

**One-step approach (auto-approve):**
```bash
# 1. Update your desired schema
vim schema.lp.sql  # Your single source of truth

# 2. Plan and apply in one command (validates on shadow DB first)
lockplane apply --auto-approve --target $DATABASE_URL --schema schema.lp.sql
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

**After** (`schema.lp.sql`, shown here in JSON form):
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
lockplane plan --from current.json --to schema.lp.sql
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
lockplane plan --from current.json --to schema.lp.sql
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
- ✅ **Breaking changes**: Identifies operations that will affect running applications
- ✅ **Data loss detection**: Warns about permanent data loss from dropping columns/tables
- ✅ **Type conversion safety**: Analyzes whether type changes preserve data

### Migration Safety Levels

Lockplane automatically classifies every migration operation by its safety level and provides detailed analysis of potential risks:

**Safety Levels:**
- **✅ Safe** - Fully reversible with no risk of data loss (e.g., adding a nullable column)
- **⚠️ Review** - May need review for performance or application compatibility (e.g., adding an index on large table)
- **🔶 Lossy** - Forward migration is safe, but rollback may lose data (e.g., widening type from INTEGER to BIGINT)
- **❌ Dangerous** - Permanent data loss or breaking change (e.g., dropping a column, narrowing type from BIGINT to INTEGER)
- **🔄 Multi-Phase** - Requires coordinated application changes (e.g., renaming a column requires expand/contract pattern)

**Example: Dangerous operation detected**
```bash
lockplane plan --from current.json --to schema.lp.sql --validate
```

```
=== Migration Safety Report ===

❌ Dangerous - Operation 1
  💥 Permanent data loss
  ⚠️  Breaking change - will affect running applications
  ↩️  Rollback: Cannot rollback - column data is permanently lost

  💡 Safer alternatives:
     • Use deprecation period: stop writes → archive data → stop reads → drop column
     • Use expand/contract if renaming: add new column → dual-write → migrate reads → drop old

=== Summary ===

  ❌ 1 dangerous operation(s)

⚠️  WARNING: This migration contains dangerous operations.
   Review safer alternatives above before proceeding.
```

**What's detected:**

1. **Data Loss Operations**
   - **Dropping columns** - Permanently loses all data in that column
   - **Dropping tables** - Permanently loses all rows and structure
   - **Type narrowing** - Converting BIGINT → INTEGER may truncate values
   - **Making columns NOT NULL** - May fail if existing rows have NULL values

2. **Rollback Risks**
   - **Type widening** (INTEGER → BIGINT) - Forward migration is safe, but rollback may lose precision
   - **Dropped objects** - Rollback can recreate structure but not restore data
   - **Irreversible operations** - Some operations cannot be safely reversed

3. **Breaking Changes**
   - Operations that require application code changes to deploy safely
   - Suggests multi-phase deployment patterns (expand/contract)
   - Identifies operations that will cause downtime if not coordinated

**Example: Type conversion analysis**

Safe widening (data preserved):
```
✅ Safe - Operation 1: Alter column users.account_balance type
  • Type change: INTEGER → BIGINT (safe widening)
  • Forward migration: safe (all values fit in larger type)
  ⚠️  Rollback: May lose precision when converting back to INTEGER
```

Dangerous narrowing (potential data loss):
```
❌ Dangerous - Operation 1: Alter column users.account_balance type
  💥 Potential data loss
  • Type change: BIGINT → INTEGER (dangerous narrowing)
  • Risk: Values outside INTEGER range will cause migration to fail

  💡 Safer alternatives:
     • Use multi-phase: add new column → backfill → dual-write → migrate reads → drop old
     • Test conversion on shadow DB first to verify data compatibility
```

**Testing dangerous operations safely:**

Always use shadow DB validation to test dangerous migrations before production:

```bash
# Test on shadow DB first (automatic with apply command)
lockplane apply migration.json --target $DATABASE_URL --shadow-db $SHADOW_DB_URL

# Shadow DB validation will:
# 1. Apply migration to shadow DB
# 2. Run validation checks
# 3. Only proceed to production if shadow DB succeeds
```

### Supported Operations

The plan generator handles:
- ✅ **Add/remove tables**
- ✅ **Add/remove columns** (with validation)
- ✅ **Modify column types, nullability, defaults**
- ✅ **Add/remove indexes**
- ✅ **Safe operation ordering** (adds before drops, tables before indexes)

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
  "source_hash": "a3f2...8b1c",
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

### Source Hash Verification

Every migration plan includes a `source_hash` field - a SHA-256 hash of the source database schema. This prevents applying plans to the wrong database state.

**Why this matters:**
- Prevents applying the wrong plan to a database
- Detects if the database was modified since the plan was generated
- Ensures plans are applied in the correct order

**How it works:**

When you generate a plan:
```bash
lockplane plan --from current.json --to schema.lp.sql > migration.json
```

The plan includes the hash of `current.json`:
```json
{
  "source_hash": "a3f2db8c1e4f9b7a5d6e2c8f1a4b9e7c3d5f8a1b2c4e6f8a9b1c3d5e7f9a2b4c6",
  "steps": [...]
}
```

When you apply the plan:
```bash
lockplane apply migration.json
```

Lockplane:
1. Introspects the current database state
2. Computes the hash of the current state
3. Compares it to `source_hash` in the plan
4. **Rejects the plan if hashes don't match**

**Example error:**
```
❌ Source schema mismatch!

The migration plan was generated for a different database state.
This usually happens when:
  - The plan is being applied to the wrong database
  - The database has been modified since the plan was generated
  - The plan is being applied out of order

Expected source hash: a3f2...2b4c6
Current database hash: b7c8...5e9f1

To fix this:
  1. Introspect the current database: lockplane introspect > current.json
  2. Generate a new plan: lockplane plan --from current.json --to desired.lp.sql
  3. Apply the new plan: lockplane apply migration.json
```

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
