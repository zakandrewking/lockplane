# Getting Started: Building Your First App with Lockplane and Claude Code

You want to build an app. Here's how Lockplane fits into that journey.

## Starting with Claude Code (No Lockplane)

You ask Claude: "Build me a note-taking app with user accounts."

Claude gives you:

**docker-compose.yml:**
```yaml
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: mypassword
      POSTGRES_USER: myuser
      POSTGRES_DB: notesapp
    ports:
      - "5432:5432"
```

**migrations/001_initial.sql:**
```sql
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE notes (
  id SERIAL PRIMARY KEY,
  user_id INTEGER REFERENCES users(id),
  title TEXT NOT NULL,
  content TEXT,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_notes_user_id ON notes(user_id);
```

You run the migration:
```bash
docker compose up -d
psql -U myuser -d notesapp < migrations/001_initial.sql
```

**This works. But there are problems:**

1. No way to test migrations before running them
2. If migration fails halfway, database is in unknown state
3. Hard to see what changed between versions
4. Can't safely roll back if something breaks
5. Claude has to guess what's in the database

## Adding Lockplane to Your Setup

**The key insight: Your schema is the source of truth. Migration plans are generated on demand.**

Start by bootstrapping your schema workspace:

```bash
lockplane init --yes
```

The wizard creates a `schema/` directory (or your chosen path) that will hold the
declarative `.lp.sql` files Lockplane uses as the source of truth. You can re-run
the wizard later to scaffold additional resources as we expand it.

Bring up your application database however you normally would (Docker Compose,
Supabase, Render, etc.). For local development, we recommend running Postgres
alongside a shadow database so Lockplane can validate migrations before touching
your data.

### Configuring Database Connections

Lockplane resolves connections from named environments. Add the defaults you want to share:

```toml
default_environment = "local"

[environments.local]
description = "Local notesapp database"
schema_path = "."
```

Store the actual credentials in `.env.local`:

```bash
cat <<'EOF' > .env.local
DATABASE_URL=postgresql://lockplane:lockplane@localhost:5432/notesapp?sslmode=disable
SHADOW_DATABASE_URL=postgresql://lockplane:lockplane@localhost:5433/notesapp_shadow?sslmode=disable
EOF
```

Lockplane automatically loads `.env.<name>` when you pass `--target-environment`, `--from-environment`, or `--source-environment`. Override temporarily with `--target`, `--shadow-db`, or `--from` if you need to point at a different database (staging, production, etc.).

> **Heads up:** Keep `.env.local` out of version control. Commit `lockplane.toml` (with sanitized defaults) and share a `.env.local.example` instead.

**Your schema source of truth** - Create directory `schema/` (recommended):

```bash
# Create the recommended schema directory
mkdir -p schema
```

Then create your schema files inside:

```sql
-- schema/001_users.lp.sql
CREATE TABLE users (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX users_email_key ON users(email);
```

```sql
-- schema/002_notes.lp.sql
CREATE TABLE notes (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  title TEXT NOT NULL,
  content TEXT,
  created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_notes_user_id ON notes(user_id);
```

**Why `schema/`?**
- Clear separation from other project files
- Easy to find and maintain
- Works well with `schema_path` in `lockplane.toml`
- Lockplane processes `.lp.sql` files sorted lexicographically
- Prefix with numbers (e.g., `001_`, `002_`) to control order

**Single file alternative:**

If you prefer a single file, use `schema/schema.lp.sql`:

```bash
lockplane plan --from current.json --to schema/schema.lp.sql --validate
```

**Convert to JSON when needed:**

```bash
lockplane convert --input schema/ --output schema.json
```

**Key insight:** This describes WHAT you want, not HOW to get there. Lockplane generates the migration plans.

## The Lockplane Workflow

**The flow: Schema → Plan → Validate → Apply**

**1. Start fresh:**

```bash
docker compose up -d
```

Both databases are empty.

**2. See current state:**

```bash
lockplane introspect > current.json
cat current.json
```

Output:
```json
{
  "tables": []
}
```

Nothing yet. This is your baseline.

**3. Generate migration plan from your schema:**

```bash
# Compare current state to desired schema
lockplane plan --from current.json --to schema.lp.sql --validate > migration.json
```

> **💡 Tip:** You can skip the introspect step by using a database connection string directly:
> ```bash
> lockplane plan --from-environment local --to schema.lp.sql --validate > migration.json
> ```
> Lockplane will automatically introspect the database when it detects a connection string.

Output shows validation:
```
✓ Validation 1: PASS
  - Table creation is always safe
  - Reversible: DROP TABLE users

✓ Validation 2: PASS
  - Table creation is always safe
  - Reversible: DROP TABLE notes

✓ All operations are reversible
✓ All validations passed
```

The generated `migration.json`:
```json
{
  "steps": [
    {
      "description": "Create table users",
      "sql": "CREATE TABLE users (id integer NOT NULL, email text NOT NULL, ...)"
    },
    {
      "description": "Create table notes",
      "sql": "CREATE TABLE notes (id integer NOT NULL, user_id integer NOT NULL, ...)"
    },
    {
      "description": "Create index users_email_key",
      "sql": "CREATE UNIQUE INDEX users_email_key ON users (email)"
    },
    {
      "description": "Create index idx_notes_user_id",
      "sql": "CREATE INDEX idx_notes_user_id ON notes (user_id)"
    }
  ]
}
```

**4. Apply the migration:**

You have two options:

**Option A: Two-step (save plan first, then apply)**
```bash
# Generate and save plan (from step 3)
lockplane plan --from current.json --to schema.lp.sql --validate > migration.json

# Apply it (uses DATABASE_URL and SHADOW_DATABASE_URL from environment)
lockplane apply migration.json --target-environment local
```

**Option B: One-step (auto-approve)**
```bash
# Generate and apply in a single command (uses DATABASE_URL from environment)
lockplane apply --auto-approve --target-environment local --schema schema.lp.sql --validate
```

**What happens in both cases:**
1. Shadow DB gets the migration first (validates it works)
2. If shadow succeeds, main DB gets the same migration
3. If shadow fails, main DB is untouched
4. Everything runs in a transaction (all or nothing)

**Manual alternative** (if you prefer to see the SQL):
```bash
# Extract and run SQL manually (only works with two-step approach)
cat migration.json | jq -r '.steps[].sql' > migration.sql
psql -U lockplane -h localhost -p 5433 -d notesapp_shadow < migration.sql
psql -U lockplane -h localhost -d notesapp < migration.sql
```

**5. Verify it worked:**

```bash
lockplane introspect
```

Output:
```json
{
  "tables": [
    {
      "name": "users",
      "columns": [
        {"name": "id", "type": "integer", "nullable": false, "is_primary_key": true},
        {"name": "email", "type": "text", "nullable": false, "is_primary_key": false},
        {"name": "created_at", "type": "timestamp without time zone", "nullable": true}
      ],
      "indexes": [
        {"name": "users_email_key", "columns": ["email"], "unique": true}
      ]
    },
    {
      "name": "notes",
      "columns": [
        {"name": "id", "type": "integer", "nullable": false, "is_primary_key": true},
        {"name": "user_id", "type": "integer", "nullable": false},
        {"name": "title", "type": "text", "nullable": false},
        {"name": "content", "type": "text", "nullable": true},
        {"name": "created_at", "type": "timestamp without time zone", "nullable": true}
      ]
    }
  ]
}
```

Now you can see exactly what's in your database. Claude can see it too.

## Making Changes

A week later, you need tags.

**Without Lockplane:**
- Write new SQL file
- Hope it doesn't conflict with existing schema
- Run it, pray it works
- If it fails halfway, fix the database by hand

**With Lockplane:**

**1. See current state:**

```bash
lockplane introspect > current.json
```

Claude can now see exactly what exists.

**2. Tell Claude what you need:**

"Add a tags table and a many-to-many relationship with notes."

**3. Claude updates your schema** (`schema.lp.sql`):

Claude adds two new tables to your schema:

```sql
-- schema.lp.sql
CREATE TABLE tags (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);

CREATE TABLE note_tags (
  note_id BIGINT NOT NULL REFERENCES notes(id),
  tag_id BIGINT NOT NULL REFERENCES tags(id),
  PRIMARY KEY (note_id, tag_id)
);
```

**4. Generate and apply the migration:**

**Option A: Two-step (generate plan, then apply)**
```bash
lockplane plan --from current.json --to schema.lp.sql --validate > add_tags.json
# Review the plan
cat add_tags.json
# Apply it
lockplane apply add_tags.json --target-environment local
```

**Option B: One-step (auto-approve)**
```bash
lockplane apply --auto-approve --target-environment local --schema schema.lp.sql --validate
```

Lockplane generates:

```json
{
  "steps": [
    {
      "description": "Create table tags",
      "sql": "CREATE TABLE tags (id integer NOT NULL, name text NOT NULL)"
    },
    {
      "description": "Create table note_tags",
      "sql": "CREATE TABLE note_tags (note_id integer NOT NULL, tag_id integer NOT NULL, PRIMARY KEY (note_id, tag_id))"
    },
    {
      "description": "Create index tags_name_key",
      "sql": "CREATE UNIQUE INDEX tags_name_key ON tags (name)"
    }
  ]
}
```

**What happens:**

Both workflows test on shadow DB first, then apply to main DB. Shadow DB catches errors. Main DB stays safe.

## Working with a Frontend

**Your typical setup:**

```
project/
├── frontend/        # React, Vue, etc
├── backend/         # API server
├── schema/          # .lp.sql files (source of truth)
│   ├── 001_tables.lp.sql
│   └── 010_indexes.lp.sql
├── docker-compose.yml
└── main.go         # Lockplane integration
```

**Frontend needs to know the schema:**

Your `schema/` directory (or single `.lp.sql` file) is both:
1. Your desired database schema
2. The source for frontend type generation (convert to JSON when needed)

```bash
# Convert your schema to JSON for the frontend
lockplane convert --input schema/ --output frontend/schema.json

# Or introspect current state if you need it
lockplane introspect > frontend/current-schema.json
```

Your frontend can now:
- Generate TypeScript types from `frontend/schema.json`
- Know what fields exist on each table
- Validate data before sending to API

**When schema changes:**

1. Update files in `schema/` (your source of truth)
2. Generate migration plan: `lockplane plan --from current.json --to schema/ --validate`
3. Test with shadow DB
4. Apply to main DB
5. Frontend already has the new schema (regenerate from `schema/`)
6. Regenerate TypeScript types
7. Deploy together

**Key insight:** Your schema file serves both database migrations AND frontend types. One source of truth for everything.

## Deployment

### Development

You're already doing this:
- Main DB for real data
- Shadow DB for testing
- `schema/` (or a single `.lp.sql`) in git (your source of truth)
- Migration plans generated on demand

### Staging

**Your staging setup:**

```yaml
# docker-compose.staging.yml
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_USER: lockplane
      POSTGRES_DB: notesapp_staging
    volumes:
      - staging_data:/var/lib/postgresql/data

  shadow:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_USER: lockplane
      POSTGRES_DB: notesapp_staging_shadow
```

**Deploy flow:**

1. Git push to staging branch
2. CI runs: `go test` (verifies migrations work)
3. Generate migration plan: `lockplane plan --from current.json --to schema/ --validate`
4. Apply migrations to staging DB:
   ```bash
   lockplane apply migration.json --target-environment local
   ```
5. Deploy new app code
6. Verify with smoke tests

### Production

**Key differences:**

1. **No shadow DB in production** (too expensive, not needed)
2. **Migrations run without shadow validation** (already tested in staging)
3. **Backups before migrations** (can restore if needed)

**docker-compose.production.yml:**

```yaml
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_USER: lockplane
      POSTGRES_DB: notesapp
    volumes:
      - /var/lib/postgresql/data:/var/lib/postgresql/data
```

**Production deploy flow:**

```bash
# 1. Backup database
pg_dump notesapp > backup_$(date +%Y%m%d).sql

# 2. Apply migrations (skip shadow DB in production)
lockplane apply migration.json --target-environment local --skip-shadow

# 3. Deploy new app code
docker compose up -d app

# 4. Verify
lockplane introspect  # Confirm schema is correct
curl /health          # Confirm app works
```

**If something breaks:**

```bash
# Restore from backup
psql notesapp < backup_20250101.sql

# Deploy previous app version
git checkout previous-version
docker compose up -d app
```

### Production Best Practices

**1. Separate migration deployments from app deployments**

Deploy migrations first, verify they work, then deploy app code. This way:
- Database changes can't break the current app
- Rollback is simpler (just the app, not migrations)

**2. Use read-only shadow DB in staging**

Instead of a full shadow DB in production, keep one in staging:
```bash
# Daily: copy production to staging shadow
pg_dump production | psql staging_shadow
```

This catches issues that only appear with real data volumes.

**3. Schedule risky migrations**

Long-running migrations (adding indexes, changing column types) should:
- Run during low-traffic windows
- Use Lockplane's durable execution (coming soon)
- Have tested rollback plans

## How Lockplane Changes Your Development

**Before Lockplane:**

1. Claude writes SQL migration files
2. You run them manually
3. Hope for the best
4. Database state is a mystery
5. No idea if you can roll back
6. Rollback is manual and scary

**With Lockplane:**

1. Introspect current state → Claude sees exactly what exists
2. Update files in `schema/` → Your desired state (source of truth)
3. Generate plan → Lockplane calculates SQL operations
4. Validate → Ensures safety and reversibility
5. Test on shadow DB → Catches errors before production
6. Apply with confidence → Transactional, validated, safe
7. Rollback available → Automatically generated from schema

**The key insight: Schema → Plan → Validate → Apply**

Your schema file is the single source of truth. Everything else is generated on demand.

**The big wins:**

- **For Claude:** `lockplane introspect` shows exact current state - no guessing
- **For you:** Validate migrations before they run - catch errors early
- **For your team:** Schema is readable SQL (and convertible to JSON) - everyone understands it
- **For production:** Shadow DB testing - safe migrations every time
- **For rollbacks:** Automatically generated - always know you can undo

## Common Workflows

**Starting a new feature:**

**Two-step approach:**
```bash
# 1. See current state
lockplane introspect > current.json

# 2. Tell Claude what you need
# "Add user profiles with avatar URLs"

# 3. Claude updates schema/
# (adds columns to users table)

# 4. Generate and validate migration
lockplane plan --from current.json --to schema/ --validate > add_profiles.json

# 5. Review the plan
cat add_profiles.json

# 6. Apply it
lockplane apply add_profiles.json --target-environment local
```

**One-step auto-approve approach:**
```bash
# 1. See current state
lockplane introspect > current.json

# 2. Tell Claude what you need
# "Add user profiles with avatar URLs"

# 3. Claude updates schema/
# (adds columns to users table)

# 4. Generate and apply in one command
lockplane apply --auto-approve --target-environment local --schema schema/ --validate
```

**Reviewing a pull request:**

```bash
# Check migration plans
cat migrations/003_add_comments.json

# Test locally with shadow DB
go test

# Approve if migrations are safe
```

**Production deployment:**

```bash
# Staging first
./deploy-staging.sh
# Verify staging works
# Then production
./deploy-production.sh
```

## What's Next

**Current state (works today):**
- ✅ JSON Schema-validated plan and schema definitions
- ✅ Introspection (see what's in the database)
- ✅ Diff engine (compare two schemas)
- ✅ Plan generator (auto-create migrations from desired state)
- ✅ Rollback generator (automatic reverse migrations)
- ✅ Migration executor (safe, transactional)
- ✅ Shadow DB validation (test before applying)

**Coming soon:**
- Durable execution (long-running operations with retries)
- MCP server (AI agents can use Lockplane as a tool)
- Catalog hash computation and ledger

## Tips for Working with Claude

**Always start with introspection:**
```
"Show me the current schema, then add a comments feature"
```

**Ask for migration plans:**
```
"Create a migration plan to add full-text search"
```
(Not: "Write SQL for full-text search")

**Review before applying:**
```
"Show me exactly what SQL will run"
```

**Test risky changes:**
```
"This changes column types - test on shadow DB first"
```

**Think in states, not scripts:**
```
"The database should have these tables: [list]"
```
(Let Claude figure out the migrations)

## Using Lockplane with Your ORM

If you're using an ORM like SQLAlchemy, Prisma, or another tool, you can integrate Lockplane into your workflow:

**SQLAlchemy (Python):**
- Generate desired schema from your models using `create_all()`
- Use Lockplane to diff and migrate safely
- See: [Lockplane with SQLAlchemy](sqlalchemy.md)

**Prisma (TypeScript/JavaScript):**
- Export schema using `prisma db pull` or from `schema.prisma`
- Use Lockplane for production migrations
- See: [Lockplane with Prisma](prisma.md)

**Alembic (Python):**
- Migrate from Alembic to Lockplane for shadow DB validation
- See: [Lockplane with Alembic](alembic.md)

These integrations let you keep your ORM as the source of truth while using Lockplane's safety features for production migrations.

## Getting Help

- Full design: `0001-design.md`
- Example migrations: `testdata/plans/`
- Example schemas: `testdata/fixtures/`
- Tests: `go test -v`

Lockplane makes database changes safe for AI and humans. Your data stays consistent, changes are explainable, and you always know what's happening.
