> **⚠️ Experimental**
> This is an experiment in vibe coding so don't expect anything reliable or production-ready.

# Lockplane

A control plane for safe, AI-friendly schema management. Works with PostgreSQL and SQLite.

## Why Lockplane?

**Shadow DB validation catches problems before production.** Lockplane tests
migrations on a shadow database first, so bad plans never touch your real data.

**Every change is explainable.** See exactly what SQL runs, in what order, with
clear descriptions.

**Rollbacks are generated and validated, not manually written.** For every
forward migration, Lockplane computes the reverse operation and validates it
works.

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

NOTE: If you do not have a database yet, you can use the `lockplane init` command to
create a new database for you. For more information, run `lockplane init --help`.

### Choose your database

Lockplane supports both PostgreSQL and SQLite. Choose the option that matches your setup:

#### Option A: PostgreSQL

If you have a PostgreSQL database running on localhost:5432 called `myapp`, set the connection string environment variable. Replace `user` and `password` with your actual database credentials:

```bash
export DATABASE_URL="postgresql://user:password@localhost:5432/myapp?sslmode=disable"
```

NOTE: You must add `?sslmode=disable` to the connection string for local development.

#### Option B: SQLite

If you're using SQLite, you can point to a file or use an in-memory database:

```bash
# Use a file-based SQLite database
export DATABASE_URL="myapp.db"

# Or use an in-memory database (useful for testing)
export DATABASE_URL=":memory:"
```

### Apply the migration

Now, we can generate a migration plan to apply our schema to our database with the following command:

```bash
lockplane apply --auto-approve --from $DATABASE_URL --to schema/
```

This will apply the schema to your database, validating it on a shadow database
first.

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
lockplane apply --auto-approve --from $DATABASE_URL --to schema/
```

And that's it! You've successfully made a change to your schema and applied it to your database.

## Configuration

Lockplane can be configured via environment variables or command-line flags. Flags take precedence over environment variables.

| Setting | Environment Variable | CLI Flag | Used By | Default |
|---------|---------------------|----------|---------|---------|
| Main database URL | `DATABASE_URL` | `--db` | `apply` | `postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable` |
| Shadow database URL | `SHADOW_DATABASE_URL` | `--shadow-db` | `apply` | `postgres://lockplane:lockplane@localhost:5433/lockplane?sslmode=disable` |

### Database Connection Strings

Lockplane uses database connections in two ways:

1. **Reading schemas** - Commands like `plan`, `diff`, `introspect`, and `rollback` accept connection strings as arguments to `--from` and `--to`:
   ```bash
   # Read current schema directly from database
   lockplane plan --from postgresql://localhost:5432/myapp --to schema.json
   ```

2. **Applying migrations** - The `apply` command uses connection strings to know where to execute:
   ```bash
   # Via environment variables (recommended for safety)
   export DATABASE_URL="postgresql://localhost:5432/myapp"
   export SHADOW_DATABASE_URL="postgresql://localhost:5433/myapp_shadow"
   lockplane apply --plan migration.json

   # Or via command-line flags
   lockplane apply --plan migration.json \
     --db "postgresql://localhost:5432/myapp" \
     --shadow-db "postgresql://localhost:5433/myapp_shadow"
   ```

**Supported database formats:**
- PostgreSQL: `postgres://` or `postgresql://`
- SQLite: `file:path/to/db.sqlite`, `path/to/db.db`, or `:memory:`

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
  --from $DATABASE_URL \
  --to schema.lp.sql \
  --validate > migration.json

# Auto-approve with database connection string
lockplane apply \
  --auto-approve \
  --from $DATABASE_URL \
  --to schema/ \
  --validate

# Generate rollback using live database state
lockplane rollback \
  --plan migration.json \
  --from $DATABASE_URL > rollback.json
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
lockplane apply --plan migration.json
```

**One-step approach (auto-approve):**
```bash
# 1. Introspect current database state
lockplane introspect > current.json

# 2. Update your desired schema
vim schema.lp.sql  # Your single source of truth

# 3. Generate and apply in one command (validates on shadow DB first)
lockplane apply --auto-approve --from current.json --to schema.lp.sql --validate
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
- 🔄 **More checks coming**: Type compatibility, data preservation, etc.

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
lockplane apply --plan migration.json
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
  3. Apply the new plan: lockplane apply --plan migration.json
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
