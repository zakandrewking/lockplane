# Getting Started: Building Your First App with Lockplane

You're building an app with Claude Code and need a database. Here's how Lockplane fits in.

## The Scenario

You're asking Claude to build a note-taking app. You need:
- A `users` table for accounts
- A `notes` table for storing notes
- Indexes for fast lookups

Without Lockplane, Claude would write migration files and you'd hope they work. With Lockplane, Claude gets:
- Current schema state (what's in the database now)
- Safe testing before changes hit production
- Clear diffs showing exactly what will change

## Initial Setup

**1. Start your databases:**

```bash
docker compose up -d
```

This starts two Postgres databases:
- Main database (port 5432) - your real data
- Shadow database (port 5433) - for testing migrations

**2. Check current state:**

```bash
go run main.go
```

You'll see empty JSON:
```json
{
  "tables": null
}
```

This is your starting point. No tables yet.

## Creating Your First Schema

**Option A: Let Claude write migration plans**

Tell Claude: "Create a migration plan for a notes app with users and notes tables."

Claude will create a JSON plan:

```json
{
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

Save this as `migrations/001_initial_schema.json`.

**Option B: Write SQL directly**

If you prefer, just write SQL and have Claude convert it to a plan format.

## Applying Changes

**Test on shadow database first:**

```go
// Load your plan
plan := loadPlan("migrations/001_initial_schema.json")

// Test on shadow DB
shadowDB := connectToShadow()
result, err := applyPlan(ctx, mainDB, &plan, shadowDB)
```

Lockplane runs the migration on the shadow database first. If it fails, your real database is untouched.

**Apply to main database:**

If shadow testing passes, the same migration runs on your main database in a transaction. Either all steps succeed or none do.

**Verify it worked:**

```bash
go run main.go
```

Now you'll see your schema:
```json
{
  "tables": [
    {
      "name": "users",
      "columns": [...],
      "indexes": [...]
    },
    {
      "name": "notes",
      "columns": [...],
      "indexes": [...]
    }
  ]
}
```

## Making Changes

A week later, you need to add tags to notes.

**See what you have:**

```bash
go run main.go > current_schema.json
```

**Create the change:**

Tell Claude: "Add a tags table and a junction table for many-to-many."

Claude writes a new plan:

```json
{
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

**Test and apply:**

Same process - shadow DB test, then main DB. Your existing data is safe because:
- Shadow DB catches errors before production
- Transaction ensures all-or-nothing execution
- You can see exactly what SQL will run

## How This Helps Claude Code

**1. Claude can see current state**

Instead of guessing what's in your database, Claude runs the introspector and knows exactly what exists.

**2. Claude can diff schemas**

When you ask for changes, Claude compares current vs desired and generates only the needed migrations.

**3. Claude can validate before running**

Shadow database testing means Claude can try risky changes without breaking your app.

**4. You can review changes**

Every migration plan is a JSON file you can read. You see exactly what SQL will run before it runs.

## Common Workflow

**Daily development:**

1. Tell Claude what you need ("add user profile pictures")
2. Claude introspects current schema
3. Claude writes migration plan
4. You review the plan
5. Claude tests on shadow DB
6. Claude applies to main DB
7. You verify with another introspection

**When things go wrong:**

Because migrations run in transactions, failures roll back automatically. Your database stays consistent.

The shadow database caught the error before it touched real data.

## What's Next

Right now you write migration plans as JSON. Soon:
- Diff engine will auto-detect what changed
- Plan generator will create migrations for you
- Rollback plans will be generated automatically

But the core workflow stays the same: introspect, plan, test, apply.

## Tips for Working with Claude

**Always introspect first:**
```
"Show me the current schema, then add a comments table"
```

**Ask for migration plans, not raw SQL:**
```
"Create a migration plan to add full-text search to notes"
```

**Review plans before applying:**
```
"Show me what SQL will run before we apply this"
```

**Use shadow DB for risky changes:**
```
"Test this migration on shadow DB first - it changes column types"
```

## Getting Help

- Full design doc: `0001-design.md`
- Example plans: `testdata/plans/`
- Example schemas: `testdata/fixtures/`
- Tests: `go test -v`

Lockplane makes database changes safe for AI and humans. Your data stays consistent, changes are explainable, and you can always see what's happening.
