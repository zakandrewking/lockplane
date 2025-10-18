# Getting Started: Building Your First App with Lockplane

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

Start by letting Lockplane configure your Compose stack:

```bash
lockplane init docker-compose
```

The command searches for an existing `docker-compose.yml`, duplicates your primary Postgres service, and creates a `shadow` database on the next available port (defaults to `5433`).

**Your new docker-compose.yml:**
```yaml
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: lockplane
      POSTGRES_USER: lockplane
      POSTGRES_DB: notesapp
    ports:
      - "5432:5432"
    volumes:
      - dbdata:/var/lib/postgresql/data

  shadow:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: lockplane
      POSTGRES_USER: lockplane
      POSTGRES_DB: notesapp_shadow
    ports:
      - "5433:5432"

volumes:
  dbdata:
```

**Key difference:** You now have two databases. Main for real data, shadow for testing.

**Your new migrations/001_initial.json:**
```json
{
  "$schema": "../schema-json/plan.json",
  "steps": [
    {
      "description": "Create users table",
      "sql": "CREATE TABLE users (id SERIAL PRIMARY KEY, email TEXT NOT NULL UNIQUE, created_at TIMESTAMP DEFAULT NOW())"
    },
    {
      "description": "Create notes table",
      "sql": "CREATE TABLE notes (id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES users(id), title TEXT NOT NULL, content TEXT, created_at TIMESTAMP DEFAULT NOW())"
    },
    {
      "description": "Add index on notes user_id",
      "sql": "CREATE INDEX idx_notes_user_id ON notes(user_id)"
    }
  ]
}
```

**Key differences:**
- Migration is a structured JSON plan validated by the bundled JSON Schema
- Each step has a description explaining what it does
- Editors can auto-complete and lint the plan because it is plain JSON

## The Lockplane Workflow

**1. Start fresh:**

```bash
docker compose up -d
```

Both databases are empty.

**2. See what you have:**

```bash
lockplane introspect
```

Output:
```json
{
  "tables": []
}
```

Nothing yet. This is your baseline.

**3. Apply your migration:**

In your Go code:
```go
// Load the plan from JSON
plan, err := LoadJSONPlan("migrations/001_initial.json")

// Connect to both databases
mainDB := connect("localhost:5432/notesapp")
shadowDB := connect("localhost:5433/notesapp_shadow")

// Apply with shadow DB validation
result, err := applyPlan(ctx, mainDB, plan, shadowDB)
```

**What happens:**
1. Shadow DB gets the migration first
2. If shadow succeeds, main DB gets the same migration
3. If shadow fails, main DB is untouched
4. Everything runs in a transaction (all or nothing)

**4. Verify it worked:**

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
lockplane introspect > current_schema.json
```

Claude can now see exactly what exists.

**2. Tell Claude what you need:**

"Add a tags table and a many-to-many relationship with notes."

Claude knows:
- What tables already exist
- What columns are already there
- What would conflict

**3. Claude creates migrations/002_add_tags.json:**

```json
{
  "$schema": "../schema-json/plan.json",
  "steps": [
    {
      "description": "Create tags table",
      "sql": "CREATE TABLE tags (id SERIAL PRIMARY KEY, name TEXT NOT NULL UNIQUE)"
    },
    {
      "description": "Create note_tags junction table",
      "sql": "CREATE TABLE note_tags (note_id INTEGER REFERENCES notes(id), tag_id INTEGER REFERENCES tags(id), PRIMARY KEY (note_id, tag_id))"
    }
  ]
}
```

**4. Test on shadow, then apply:**

Same workflow. Shadow DB catches errors. Main DB stays safe.

## Working with a Frontend

**Your typical setup:**

```
project/
├── frontend/        # React, Vue, etc
├── backend/         # API server
├── migrations/      # Lockplane migration plans
├── docker-compose.yml
└── main.go         # Lockplane introspector
```

**Frontend needs to know the schema:**

```bash
# Generate current schema for the frontend (JSON format)
lockplane introspect > frontend/schema.json
```

Your frontend can now:
- Generate TypeScript types from the schema
- Know what fields exist on each table
- Validate data before sending to API

**When schema changes:**

1. Create migration plan (JSON plan file)
2. Test with shadow DB
3. Apply to main DB
4. Regenerate schema file
5. Update frontend types
6. Deploy together

No surprises. Frontend and backend stay in sync.

## Deployment

### Development

You're already doing this:
- Main DB for real data
- Shadow DB for testing
- Migration plans in git

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
3. Apply migrations to staging DB (with shadow validation)
4. Deploy new app code
5. Verify with smoke tests

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

# 2. Apply migrations (no shadow DB)
# (Use your apply function with LoadJSONPlan)

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

1. Claude writes SQL
2. You run it
3. Hope for the best
4. Database state is a mystery
5. Rollback is manual and scary

**With Lockplane:**

1. Claude introspects (sees exact current state)
2. Claude writes a migration plan in JSON
3. JSON Schema validation ensures the structure is correct
4. Shadow DB tests it first
5. Transaction ensures atomicity
6. You always know what's in the database
7. Rollback plans are automatically generated

**The big wins:**

- **For Claude:** No more guessing what's in the database
- **For you:** Clear plans you can review before execution, with type safety
- **For your team:** Migration history is readable JSON with validation
- **For production:** Shadow testing catches issues early

## Common Workflows

**Starting a new feature:**

```bash
# See current state
lockplane introspect > schema_before.json

# Build feature (Claude writes migrations)
# ...

# See what changed
lockplane introspect > schema_after.json
lockplane diff schema_before.json schema_after.json
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

## Getting Help

- Full design: `0001-design.md`
- Example migrations: `testdata/plans/`
- Example schemas: `testdata/fixtures/`
- Tests: `go test -v`

Lockplane makes database changes safe for AI and humans. Your data stays consistent, changes are explainable, and you always know what's happening.
