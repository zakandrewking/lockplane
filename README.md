# Lockplane

A Postgres-first control plane for safe, AI-friendly schema management.

## Why Lockplane?

**If you're building with AI agents**, you've probably noticed that traditional SQL migrations are a minefield. An AI can write `ALTER TABLE` statements all day, but can it guarantee those changes are safe? Can it roll back cleanly when things go wrong? Can it explain exactly what will happen before running anything? The honest answer is usually no. Lockplane makes schema changes deterministic and explainable—every migration runs in a transaction, gets validated on a shadow database first, and produces a clear audit trail. Your AI tools can finally manage databases without crossing their fingers.

**If you're a developer who's lived through migration disasters**, you know the fear of running `db:migrate` on production. Was that column actually nullable? Will this index lock the table for 10 minutes? Did someone else already apply this? Lockplane's shadow database testing catches these issues before they touch your real data. Every plan shows you exactly what SQL will run, in what order, with what outcome. No surprises, no guessing, no praying.

**If you want Firebase-level ease with Postgres-level power**, you're not alone. Firebase spoiled us with instant schema changes and zero migration headaches—until we hit the limits of a document model and needed real joins, constraints, and transactions. Lockplane brings that ease to Postgres: introspect your current schema as JSON, define your desired state, let the system figure out the migration. You keep full SQL power, you just don't have to manually write every ALTER statement.

Lockplane runs on stock Postgres (14–16), works alongside your existing tools, and doesn't lock you into anything proprietary. It's not magic—it's just what schema management should have been all along.

## Quick Start

### Prerequisites
- Go 1.24+
- Docker & Docker Compose

### Setup

1. Start Postgres:
```bash
docker compose up -d
```

2. Run the introspector:
```bash
go run main.go
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
go run main.go
```

## Migration Executor

Lockplane includes a transactional migration executor that safely applies schema changes.

### Plan Format

Migration plans are JSON files with a series of SQL steps:

```json
{
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

See example plans in `testdata/plans/`:
- `create_table.json` - Create a new table
- `add_column.json` - Add columns with constraints

### Using the Executor

The executor provides:
- **Transactional execution** - All steps succeed or all roll back
- **Shadow DB validation** - Test migrations before applying to main DB
- **Error tracking** - Detailed failure reporting

Example usage in Go:

```go
// Load migration plan
planBytes, _ := os.ReadFile("testdata/plans/create_table.json")
var plan Plan
json.Unmarshal(planBytes, &plan)

// Apply with shadow DB validation
shadowDB, _ := sql.Open("postgres", shadowConnStr)
result, err := applyPlan(ctx, mainDB, &plan, shadowDB)

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

**Completed:**
- ✅ Schema introspector with JSON output
- ✅ Shadow DB setup for dry-run validation
- ✅ Transactional migration executor
- ✅ Golden test suite with fixtures

**In Progress:**
- DSL format for defining desired schema
- Diff engine to compare DSL vs live schema
- Plan generator to auto-create migration plans
