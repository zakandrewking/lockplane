> **⚠️ Experimental**
> This is an experiment in vibe coding so don't expect anything reliable or production-ready.

# Lockplane

[![Test](https://github.com/lockplane/lockplane/actions/workflows/test.yml/badge.svg)](https://github.com/lockplane/lockplane/actions/workflows/test.yml)
[![codecov](https://codecov.io/github/lockplane/lockplane/graph/badge.svg?token=JP0QINP1G1)](https://codecov.io/github/lockplane/lockplane)

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

### Quick Start with npx (Recommended)

No installation needed! Run lockplane directly with npx:

```bash
npx lockplane --version
```

This works on any platform (Linux, macOS, Windows) and always uses the latest version.

### Global Installation

If you prefer a permanent installation:

```bash
npm install -g lockplane
npx lockplane --version
```

### Alternative: Download Pre-built Binary

1. Download the latest release for your platform from [GitHub
   Releases](https://github.com/zakandrewking/lockplane/releases/latest)
2. Extract the archive: `tar -xzf lockplane_*.tar.gz`
3. Move to your PATH: `sudo mv lockplane /usr/local/bin/`
4. Verify: `npx lockplane --version`

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

> **Dialect selection**
>
> Lockplane parses `.lp.sql` files with PostgreSQL rules by default. For SQLite
> projects, specify `dialect = "sqlite"` in `lockplane.toml` (global) or provide
> SQLite connection strings when running commands. Inline `-- dialect:` comments
> are ignored.

> **Supabase preset**
>
> Working inside a Supabase repo? Run `lockplane init --supabase --yes` to scaffold
> `supabase/schema/`, generate `.env.supabase` with shadow-schema defaults, and set
> `schema_path = "supabase/schema"` in `lockplane.toml`. All commands (plan/apply)
> now auto-detect `supabase/schema/` before falling back to `schema/` when no
> `--schema` flag is provided.

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
npx lockplane plan --validate schema/
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

For more information on `.lp.sql` files, run `npx lockplane plan --help`.

## 4. 📜 Run your first migration

Now that we have a schema, we can generate a migration plan to apply it to our
database.

### Quick Setup: Interactive Wizard

The easiest way to get started is with the interactive wizard:

```bash
npx lockplane init
```

The wizard will guide you through:
- **Database type selection**: Choose PostgreSQL, SQLite, or libSQL/Turso
- **Connection input method (PostgreSQL only)**: Enter individual fields or paste a connection string
- **Connection details**: Enter your database credentials with smart defaults
- **Shadow strategy selection**: Pick a separate PostgreSQL database, a schema via `SHADOW_SCHEMA`, or a custom SQLite/Turso shadow file
- **Shadow configuration inputs**: Confirm port/schema/path values before Lockplane touches your environment
- **Connection testing**: Verify your database (and shadow settings) are reachable before proceeding
- **Environment setup**: Configure multiple environments (local, staging, production)
- **Summary & file generation**: Creates `lockplane.toml`, `.env.*`, and `.gitignore` updates with secure permissions

**Features:**
- Detects existing configurations and offers to add new environments
- Auto-configures SSL mode (disabled for localhost, required for remote hosts)
- Tests connections before saving to catch errors early
- Updates `.gitignore` to protect your credentials
- Provides clear next steps after setup

**Non-interactive mode** (for CI/scripts):

```bash
# Use all defaults (PostgreSQL on localhost)
npx lockplane init --yes

# Custom PostgreSQL configuration (individual fields)
npx lockplane init --yes \
  --env-name production \
  --description "Production database" \
  --host db.example.com \
  --database myapp \
  --user myuser \
  --password "$DB_PASSWORD" \
  --ssl-mode require

# PostgreSQL with connection string (easier for copying from cloud providers)
npx lockplane init --yes \
  --env-name production \
  --connection-string "postgresql://user:pass@db.example.com:5432/myapp?sslmode=require"

# SQLite configuration
npx lockplane init --yes \
  --db-type sqlite \
  --file-path ./myapp.db

# libSQL/Turso configuration
npx lockplane init --yes \
  --db-type libsql \
  --env-name production \
  --url "libsql://mydb-myorg.turso.io" \
  --auth-token "$TURSO_TOKEN"
```

All configuration flags:
- `--env-name` - Environment name (default: "local")
- `--description` - Environment description
- `--db-type` - Database type: postgres, sqlite, libsql (default: "postgres")
- `--schema-path` - Schema path relative to config (default: ".")

PostgreSQL flags:
- `--connection-string` - Full connection string (alternative to individual fields below)
- `--host` - Host (default: "localhost")
- `--port` - Port (default: "5432")
- `--database` - Database name (default: "lockplane")
- `--user` - User (default: "lockplane")
- `--password` - Password (default: "lockplane")
- `--ssl-mode` - SSL mode (auto-detected if not specified)
- `--shadow-db-port` - Shadow database port (default: "5433")
- `--shadow-schema` - Shadow schema for Postgres (reuses the primary database unless `--shadow-db` overrides)

SQLite flags:
- `--file-path` - Database file path (default: "schema/lockplane.db")

libSQL flags:
- `--url` - Database URL
- `--auth-token` - Auth token

### Adding Multiple Environments

Lockplane makes it easy to manage multiple environments (local, staging, production, etc.) in the same project. When you run `npx lockplane init` after already having a configuration, the wizard will:

1. **Detect your existing configuration** and show you what environments are already defined
2. **Guide you through adding new environments** - just like the initial setup
3. **Preserve all existing environments** - your old configurations stay intact
4. **Show a summary** of all configured environments—primary + shadow connections—before writing any files

**Example workflow:**

```bash
# Initial setup - create local environment
npx lockplane init
# ... follow the wizard to set up your local database ...

# Later, add staging environment
npx lockplane init
# The wizard will show:
# "Found existing configuration!"
# "Existing Environments: local"
# Then guide you through adding staging

# Add production environment
npx lockplane init
# The wizard will show:
# "Found existing configuration!"
# "Existing Environments: local, staging"
# Then guide you through adding production
```

**What gets created:**
- `lockplane.toml` - Updated with all environments
- `.env.local`, `.env.staging`, `.env.production` - One file per environment
- `.gitignore` - Updated to protect credentials

**Summary before saving:**
Before finalizing, the wizard shows a clear summary:

```
Configuration Summary

Total configured environments: 3

Existing environments (will be preserved):
  • local
  • staging

New environments to be added:
  • production (postgres)

Files to be created/updated:
  • lockplane.toml (will be updated)
  • .env.production (new)
  • .gitignore (update if needed)
```

**Updating an environment:**
If you add an environment with the same name as an existing one, the wizard will update it with the new configuration.

### Manual Configuration (Alternative)

If you prefer manual setup, create a `lockplane.toml` file in your project root.
You can also use the sample at `lockplane.toml.example` as a starting point.

```toml
default_environment = "local"
schema_path = "."
dialect = "postgres"
schemas = ["public"]

[environments.local]
description = "Local development database"
```

For Supabase projects, set `schema_path = "supabase/schema"` (or run
`lockplane init --supabase --yes` to scaffold it automatically) and point your
environment at the Supabase connection string plus `SHADOW_SCHEMA`.

Next, provide the actual credentials in `.env.local` (ignored by Git by default).

#### Example: PostgreSQL

```bash
cat <<'EOF' > .env.local
# PostgreSQL connection (auto-detected sslmode=disable for localhost)
POSTGRES_URL=postgresql://user:password@localhost:5432/myapp?sslmode=disable

# Option 1: Separate shadow database (default)
POSTGRES_SHADOW_URL=postgresql://user:password@localhost:5433/myapp_shadow?sslmode=disable

# Option 2: Use a schema inside the same database
# (Lockplane will use SHADOW_SCHEMA and re-use POSTGRES_URL)
# SHADOW_SCHEMA=lockplane_shadow
EOF

Prefer CLI overrides? Pass `--shadow-schema lockplane_shadow` (or `--shadow-db ...`) to `apply`, `rollback`, and `plan --validate`.
```

#### Example: SQLite

```bash
cat <<'EOF' > .env.local
# SQLite connection (file-based)
SQLITE_DB_PATH=./schema/myapp.db
# Shadow database (configured for SQLite - safe migrations)
# Auto-configured as <filename>_shadow.db
SQLITE_SHADOW_DB_PATH=./schema/myapp_shadow.db
EOF
```

#### Example: Turso/libSQL

```bash
cat <<'EOF' > .env.local
# libSQL/Turso connection (remote edge database)
LIBSQL_URL=libsql://mydb-user.turso.io
LIBSQL_AUTH_TOKEN=eyJhbGc...
# Shadow database (local SQLite for validation - safe migrations)
# Auto-configured as ./schema/turso_shadow.db (no second Turso database needed!)
LIBSQL_SHADOW_DB_PATH=./schema/turso_shadow.db
EOF
```

### Shadow Database Configuration

**What is a Shadow Database?**

A shadow database is a temporary, isolated database where Lockplane tests migrations before applying them to your production database. This catches errors early and prevents bad migrations from corrupting your data.

**Auto-Configuration by Database Type:**

Lockplane automatically configures shadow databases during `lockplane init`:

**PostgreSQL:**
- Creates a separate database on port 5433 (default)
- Uses `<database>_shadow` naming convention
- Example: `myapp` → `myapp_shadow`
- Configured via `POSTGRES_SHADOW_URL`
- Alternatively, set `SHADOW_SCHEMA=lockplane_shadow` to reuse the primary database and isolate work inside that schema (no second database required)

**SQLite:**
- Creates a shadow file alongside your main database
- Uses `<filename>_shadow.db` naming convention
- Example: `myapp.db` → `myapp_shadow.db`
- Configured via `SQLITE_SHADOW_DB_PATH`

**libSQL/Turso:**
- Uses local SQLite file for validation (no second Turso database needed)
- Auto-configured as `./schema/turso_shadow.db`
- Saves cost by validating locally before deploying to edge
- Configured via `LIBSQL_SHADOW_DB_PATH`

> 💡 During `lockplane init`, choose “Customize shadow settings” if you want to change the default PostgreSQL port or provide explicit SQLite/Turso shadow file paths.

**Customization:**

You can override shadow DB settings in your `.env.<environment>` files:
```bash
# PostgreSQL - Use different port or host
POSTGRES_SHADOW_URL=postgresql://user:pass@localhost:5434/myapp_shadow?sslmode=disable
# PostgreSQL - Use schema in same database
SHADOW_SCHEMA=lockplane_shadow

# SQLite - Use different path
SQLITE_SHADOW_DB_PATH=./test/shadow.db

# libSQL - Use different local shadow DB
LIBSQL_SHADOW_DB_PATH=./test/turso_shadow.db
```

Lockplane automatically loads `.env.<name>` for the selected environment (for the
default environment, `.env.local`). You can still override any value with CLI flags
such as `--target` or `--shadow-db` when needed.

**How Lockplane picks the shadow target (priority order):**
1. CLI `--shadow-db`
2. Environment variables / `.env` entries (`POSTGRES_SHADOW_URL`, `SQLITE_SHADOW_DB_PATH`, etc.)
3. Schema overrides (`SHADOW_SCHEMA` / `--shadow-schema`) – reuses the primary connection and isolates work inside that schema
4. Built-in defaults (`:memory:` for SQLite/libSQL, `<database>_shadow` on port 5433 for PostgreSQL)

This means you can keep your `.env` minimal: choose schema mode once (via wizard, `.env`, or `--shadow-schema`) and Lockplane automatically reuses the correct database unless you explicitly point it elsewhere with `--shadow-db`.

### Apply the migration

Now, we can generate a migration plan to apply our schema to our database with the following command:

```bash
npx lockplane apply --auto-approve --target-environment local --schema schema/
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
npx lockplane apply --auto-approve --target-environment local --schema schema/
```

And that's it! You've successfully made a change to your schema and applied it to your database.

## 6. ✅ Final environment check

Before handing the project to teammates or automations:

- Commit `lockplane.toml` with the environments you expect everyone to use.
- Keep sensitive credentials in `.env.<name>` files (add them to `.gitignore`) and share sanitized samples such as `.env.local.example`.
- Record which environment maps to each deployed database (local, staging, production) so the `--*-environment` flags stay meaningful.
- When automating (CI, release pipelines), copy the appropriate `.env.<name>` file onto the runner or provide explicit `--target` / `--from` flags.

Lockplane always prefers explicit CLI values, so you can temporarily override connections without touching the shared environment files.

## 7. 🔄 Multi-Phase Migrations (Zero-Downtime Changes)

Some schema changes can't be done safely in a single step. Renaming a column, changing data types, or removing columns requires coordination between database changes and code deployments.

Lockplane provides **multi-phase migration plans** that break complex changes into safe, backward-compatible steps using the expand/contract pattern.

### When to Use Multi-Phase Migrations

Use multi-phase migrations for:
- **Column renames** - `email` → `email_address`
- **Type changes** - `TEXT` → `INTEGER`
- **Adding NOT NULL constraints** - Need backfill first
- **Dropping columns** - Need code deploy before schema change
- **Dropping tables** - Need deprecation period and optional archiving

### Example: Renaming a Column

Let's safely rename `users.email` to `users.email_address`:

```bash
npx lockplane plan-multiphase \
  --pattern expand_contract \
  --table users \
  --old-column email \
  --new-column email_address \
  --type TEXT > rename-email.json
```

This generates a **3-phase plan**:

**Phase 1: Expand** - Add new column, enable dual-write
```sql
ALTER TABLE users ADD COLUMN email_address TEXT;
UPDATE users SET email_address = email WHERE email_address IS NULL;
```
*Then deploy code that writes to BOTH columns*

**Phase 2: Migrate Reads** - Switch to reading from new column
*Deploy code that reads from `email_address`, keeps writing to both*

**Phase 3: Contract** - Remove old column
```sql
ALTER TABLE users DROP COLUMN email;
```

### Supported Patterns

**Expand/Contract** - Column rename or compatible type change
```bash
npx lockplane plan-multiphase --pattern expand_contract \
  --table users --old-column name --new-column full_name --type TEXT
```

**Deprecation** - Safe column removal with deprecation period
```bash
npx lockplane plan-multiphase --pattern deprecation \
  --table users --column last_login --type TIMESTAMP
```

**Drop Table** - Safe table removal with optional archiving
```bash
# With data archiving
npx lockplane plan-multiphase --pattern drop_table \
  --table old_logs --archive-data

# Without archiving
npx lockplane plan-multiphase --pattern drop_table \
  --table temp_table
```

**Validation** - Add constraints with backfill
```bash
npx lockplane plan-multiphase --pattern validation \
  --table posts --column status --type TEXT \
  --constraint "CHECK (status IN ('draft', 'published'))"
```

**Type Change** - Incompatible type changes with dual-write
```bash
npx lockplane plan-multiphase --pattern type_change \
  --table users --column age \
  --old-type TEXT --new-type INTEGER
```

### Multi-Phase Plan Structure

Multi-phase plans are JSON files that contain:
- Multiple coordinated migration phases
- Code deployment requirements between phases
- Verification steps for each phase
- Phase-specific rollback instructions

Example structure:
```json
{
  "multi_phase": true,
  "operation": "rename_column",
  "pattern": "expand_contract",
  "total_phases": 3,
  "phases": [
    {
      "phase_number": 1,
      "name": "expand",
      "requires_code_deploy": true,
      "code_changes_required": [
        "Update app to write to both columns"
      ],
      "plan": {
        "steps": [...]
      },
      "verification": [...],
      "rollback": {...}
    }
  ]
}
```

### Executing Multi-Phase Plans

Once you have a multi-phase plan, execute it phase by phase:

**Phase 1: Execute first database changes**
```bash
npx lockplane apply-phase rename-email.json --phase 1
```

Lockplane tracks progress in `.lockplane-state.json` (git-ignored) to ensure phases are executed in order.

**Phase 2: Deploy code, then continue**
After deploying code changes for phase 1:
```bash
npx lockplane apply-phase rename-email.json --next
# or explicitly: --phase 2
```

**Phase 3: Complete the migration**
```bash
npx lockplane apply-phase rename-email.json --next
```

**Check status at any time:**
```bash
npx lockplane phase-status
```

Output:
```
📋 Active Multi-Phase Migration

Operation:   rename_column
Pattern:     expand_contract
Progress:    2/3 phases complete

Phase Status:
  ✅ Phase 1: Complete
  ✅ Phase 2: Complete
  ▶️  Phase 3: Ready to execute

Next: Execute phase 3
  lockplane apply-phase rename-email.json --phase 3
```

**Rollback if needed:**
```bash
# Rollback current phase
npx lockplane rollback-phase rename-email.json

# Rollback specific phase
npx lockplane rollback-phase rename-email.json --phase 2
```

See [Multi-Phase Migration Guide](devdocs/projects/multi-phase-migrations.md) for detailed documentation.

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

From `lockplane.toml` you can reference `.env.<name>` files located in the project root.
Lockplane searches for `lockplane.toml` starting from the current directory and walks up
until it finds the file or reaches a project boundary (.git, go.mod, package.json).
CLI flags still take precedence, so you can override connections temporarily.

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
npx lockplane apply plan.json \
  --target-environment staging \
  --target "postgresql://override@host/db" \
  --shadow-db "postgresql://override@host/db_shadow"

# Reuse the primary database but isolate work inside a schema
npx lockplane apply plan.json \
  --target-environment staging \
  --shadow-schema lockplane_shadow
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
npx lockplane convert --input schema.lp.sql --output schema.json
npx lockplane convert --input schema.json --output schema.lp.sql --to sql
```

Editors that support JSON Schema validation can point at `schema-json/schema.json` for autocomplete when working in JSON. See [examples/schemas-json/](./examples/schemas-json/) for reference files.

### Organizing Multiple Files

Prefer keeping related DDL in separate `.lp.sql` files? Point Lockplane at the directory:

```bash
# Combine all .lp.sql files in a directory into a single schema (non-recursive)
npx lockplane plan --from current.json --to schema/ --validate
npx lockplane convert --input schema/ --output schema.json
```

Files are read in lexicographic order, so you can prefix them with numbers (for example `001_tables.lp.sql`, `010_indexes.lp.sql`) to make the order explicit. Only top-level files are considered—subdirectories and symlinks are skipped to avoid accidental recursion.

## Schema Validation

Lockplane provides comprehensive validation for schema and plan files to catch errors early.

### Validating SQL Schemas (`.lp.sql`)

```bash
# Validate SQL schema directory
npx lockplane plan --validate schema/

# Validate with JSON output (for IDE integration)
npx lockplane plan --validate schema/ --output json

# Specify shadow DB
npx lockplane plan --validate schema/ --shadow-db "postgresql://..."

# Reuse the main database with a dedicated schema
npx lockplane plan --validate schema/ --shadow-schema lockplane_shadow
```

**What's validated:**

1. **SQL Syntax & Semantics**
   - Executes schema against shadow database to catch both syntax and semantic errors
   - Detects missing tables (e.g., index on non-existent table)
   - Detects missing columns (e.g., view referencing non-existent column)
   - Detects type mismatches, constraint violations, etc.
   - Reports exact error messages from the database

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
npx lockplane validate schema schema.json
```

**What's validated:**
- JSON syntax (must be valid JSON)
- Structure matches Lockplane JSON Schema (`schema-json/schema.json`)
- All required fields are present
- Data types are correct

### Validating Migration Plans

```bash
# Validate migration plan file
npx lockplane validate plan migration.json

# Validate with JSON output
npx lockplane validate plan --format json migration.json
```

**What's validated:**
- JSON syntax
- Structure matches Lockplane plan schema (`schema-json/plan.json`)
- All migration steps are well-formed
- SQL statements in steps are present and non-empty

### IDE Integration

The `--output-format json` flag outputs structured validation results for IDE integration. The [VSCode Lockplane extension](vscode-lockplane/) uses this to show real-time validation errors as you type.

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
npx lockplane introspect > current.json

# Forward migration (current → desired)
npx lockplane plan --from current.json --to schema.lp.sql --validate > forward.json

# Reverse migration (desired → current)
npx lockplane plan --from schema.lp.sql --to current.json --validate > reverse.json
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
npx lockplane plan \
  --from postgres://user:pass@localhost:5432/production \
  --to postgres://user:pass@localhost:5433/staging \
  --validate > migration.json

# Compare live database to schema file
npx lockplane plan \
  --from-environment local \
  --to schema.lp.sql \
  --validate > migration.json

# Auto-approve: plan and apply directly to database
npx lockplane apply \
  --auto-approve \
  --target-environment local \
  --schema schema/

# Generate rollback plan (two-step workflow)
npx lockplane plan-rollback \
  --plan migration.json \
  --from-environment local > rollback.json

# Or apply rollback directly (one-step workflow)
npx lockplane rollback \
  --plan migration.json \
  --target-environment local
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
npx lockplane introspect > current.json

# 2. Update your desired schema
vim schema.lp.sql  # Your single source of truth

# 3. Generate and validate migration plan
npx lockplane plan --from current.json --to schema.lp.sql --validate > migration.json

# 4. Review the generated plan
cat migration.json

# 5. Apply the migration (validates on shadow DB first)
npx lockplane apply migration.json
```

**One-step approach (auto-approve):**
```bash
# 1. Update your desired schema
vim schema.lp.sql  # Your single source of truth

# 2. Plan and apply in one command (validates on shadow DB first)
npx lockplane apply --auto-approve --target $DATABASE_URL --schema schema.lp.sql
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
npx lockplane plan --from current.json --to schema.lp.sql
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
npx lockplane plan --from current.json --to schema.lp.sql
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
npx lockplane plan --from current.json --to schema.lp.sql --validate
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
npx lockplane apply migration.json --target $DATABASE_URL --shadow-db $SHADOW_DB_URL
# or reuse the same database with a schema override
npx lockplane apply migration.json --target $DATABASE_URL --shadow-schema lockplane_shadow

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
npx lockplane plan --from current.json --to schema.lp.sql > migration.json
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
npx lockplane apply migration.json
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
