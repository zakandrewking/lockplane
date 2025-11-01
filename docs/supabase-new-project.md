# Using Lockplane with a New Supabase Project

Start your new Supabase project with declarative schema control from day one.

## Prerequisites

- Supabase project + service role key
- Lockplane CLI installed locally
- psql (for sanity checks)

## Setup

### 1. Connect to Supabase

In the Supabase dashboard, go to *Project Settings → Database* and copy the connection string.

Define a Supabase environment in `lockplane.toml`:

```toml
[environments.supabase]
description = "Supabase production"
```

Create `.env.supabase` (ignored by Git) with the credentials you copied:

```bash
cat <<'EOF' > .env.supabase
DATABASE_URL=postgres://postgres:<password>@<host>:5432/postgres
SHADOW_DATABASE_URL=postgres://postgres:<password>@<host>:6543/postgres
EOF
```

Since Supabase blocks direct database creation, point the shadow URL at a separate Supabase project or a local Postgres container. For local testing: `docker compose up supabase-shadow` using the sample `docker-compose.yml` in this repo.

### 2. Create Your Schema

Create `desired.json` with your tables, columns, and constraints. Use `examples/schemas-json` as a template.

Example:
```json
{
  "tables": [
    {
      "name": "users",
      "columns": [
        {"name": "id", "type": "uuid", "nullable": false, "default": "gen_random_uuid()"},
        {"name": "email", "type": "text", "nullable": false},
        {"name": "created_at", "type": "timestamptz", "nullable": false, "default": "now()"}
      ],
      "primaryKey": ["id"]
    }
  ]
}
```

Validate immediately:
```bash
lockplane validate schema desired.json
```

### 3. Generate Your First Migration

```bash
# Since this is a new project with no tables, you can read from the Supabase environment
lockplane plan --from-environment supabase --to desired.json --validate > migration.json
```

> **💡 Tip:** Lockplane automatically introspects your database when you provide a connection string. For a brand new project, the introspection will return an empty schema.

The validation report highlights risky operations. Review and fix before proceeding.

### 4. Apply

```bash
lockplane apply migration.json --target-environment supabase
```

This runs on the shadow database first, then applies to production.

## Team Workflow

- Store `desired.json`, `migration.json`, and `rollback.json` in `supabase/lockplane/`
- Reference them in pull requests
- Automate validation with GitHub Actions using your service-role key as a secret

## Troubleshooting

- **SSL errors:** append `?sslmode=require` to the URLs in `.env.supabase`
- **Shadow environment issues:** reset between runs with `supabase db reset` or `docker compose down -v`
