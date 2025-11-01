# Using Lockplane with an Existing Supabase Project

Add declarative schema control, safety checks, and reversible migrations to your existing Supabase project.

## Prerequisites

- Existing Supabase project + service role key
- Supabase CLI (`npm install -g supabase`)
- Lockplane CLI installed locally
- psql (for sanity checks)

## Setup

### 1. Connect to Supabase

In the Supabase dashboard, go to *Project Settings → Database* and copy the connection string.

Add a Supabase environment to `lockplane.toml` and store credentials in `.env.supabase`:

```toml
[environments.supabase]
description = "Supabase production"
```

```bash
cat <<'EOF' > .env.supabase
DATABASE_URL=postgres://postgres:<password>@<host>:5432/postgres
SHADOW_DATABASE_URL=postgres://postgres:<password>@<host>:6543/postgres
EOF
```

Since Supabase blocks direct database creation, point the shadow URL at a separate Supabase project or a local Postgres container. For local testing: `docker compose up supabase-shadow` using the sample `docker-compose.yml` in this repo.

### 2. Author Your Changes

Edit `desired.json` to define your target schema. You can use `examples/schemas-json` as a reference, or introspect your current database and modify it:

```bash
# Optional: capture current state to use as a template
lockplane introspect > desired.json
# Edit desired.json with your changes
```

Validate your changes:
```bash
lockplane validate schema desired.json
```

### 3. Plan and Review

Generate a migration plan directly from your database:

```bash
# Lockplane will automatically introspect your current state
lockplane plan --from-environment supabase --to desired.json --validate > migration.json
```

> **💡 Tip:** You don't need to run `lockplane introspect` first—Lockplane automatically introspects your database when you provide a connection string!

The validation report highlights risky operations (e.g., adding NOT NULL columns without defaults). Fix `desired.json` or add backfill steps before proceeding.

### 4. Test on Shadow Database

Dry-run the migration on your shadow database:

```bash
lockplane apply migration.json --target-environment supabase
```

Lockplane applies to the shadow database first for safety.

### 5. Deploy to Production

**Option A: Direct apply**
```bash
lockplane apply migration.json --target-environment supabase
```

**Option B: Via Supabase CLI**

Convert the generated SQL into Supabase migrations. Each `PlanStep` contains SQL—copy it into `supabase/migrations/<timestamp>_lockplane.sql`.

```bash
supabase db push
```

This keeps Supabase migration history aligned with Lockplane's declarative schema.

## Team Workflow

- Store `desired.json`, `migration.json`, and `rollback.json` in `supabase/lockplane/`
- Reference them in pull requests
- Automate validation with GitHub Actions using your service-role key as a secret
- When Supabase adds new extensions or triggers, introspect again to sync before making changes

## Troubleshooting

- **SSL errors:** add `?sslmode=require` to the URLs in `.env.supabase`
- **Function/trigger differences:** Supabase migrations create additional objects. Ignore them in Lockplane by scoping JSON to the tables you manage, or extend the schema definition
- **Shadow environment mismatches:** reset between runs with `supabase db reset` or `docker compose down -v` for local mirrors
