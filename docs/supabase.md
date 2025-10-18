# Using Lockplane with Supabase

Supabase projects run on managed PostgreSQL. Lockplane gives you declarative schema control, safety checks, and reversible plans that complement Supabase's migration tooling.

## Prerequisites

- Supabase project + service role key (for migrations)
- Supabase CLI (`npm install -g supabase`)
- Lockplane CLI installed locally
- psql (for sanity checks)

## Connect Lockplane to Supabase

1. **Fetch credentials**
   - In the Supabase dashboard, go to *Project Settings → Database*.
   - Copy the connection string for the primary database (typically `postgres://postgres:<password>@<host>:5432/postgres`).
   - Export it so Lockplane can see it:
     ```bash
     export DATABASE_URL='postgres://postgres:<password>@<host>:5432/postgres'
     export SHADOW_DATABASE_URL='postgres://postgres:<password>@<host>:6543/postgres'
     ```
   - Supabase blocks direct creation of new databases, so point the shadow URL at a separate Supabase project or a local Postgres container. For local testing you can run `docker compose up supabase-shadow` via the sample `docker-compose.yml` in this repo.

2. **Capture current state**
   ```bash
   lockplane introspect > current.json
   ```
   Commit `current.json` if you want a baseline snapshot.

3. **Author desired schema**
   - Edit `desired.json` manually or use the `examples/schemas-json` layout as a template.
   - Validate immediately:
     ```bash
     lockplane validate schema desired.json
     ```

4. **Plan and review**
   ```bash
   lockplane plan --from current.json --to desired.json --validate > migration.json
   ```
   - The validation report will highlight risky operations (e.g., adding NOT NULL columns without defaults). Fix `desired.json` or add backfill steps before proceeding.

5. **Dry-run on a Supabase shadow**
   - If you have a staging Supabase project, set `SHADOW_DATABASE_URL` to that project's connection string.
   - Lockplane's `apply` command will run against the shadow first:
     ```bash
     lockplane apply --plan migration.json
     ```

6. **Deploy with Supabase CLI**
   - Optional: convert the generated SQL into Supabase migrations. Each `PlanStep` contains SQL; copy it into `supabase/migrations/<timestamp>_lockplane.sql`.
   - Then run:
     ```bash
     supabase db push
     ```
   - This keeps Supabase migration history aligned with Lockplane's declarative schema.

## Team Workflow Tips

- Store `desired.json`, `migration.json`, and `rollback.json` in `supabase/lockplane/` and reference them in pull requests.
- Automate validation with GitHub Actions using the service-role connection string saved as a secret.
- When Supabase adds new extensions or triggers, introspect again to sync Lockplane's schema before modifying tables.

## Troubleshooting

- **SSL errors:** add `?sslmode=require` to `DATABASE_URL`.
- **Function/trigger differences:** Supabase migrations create additional objects. Ignore them in Lockplane by scoping JSON to the tables you manage, or extend the schema definition.
- **Shadow environment mismatches:** always reset or rebuild the shadow DB between runs using `supabase db reset` or `docker compose down -v` for local mirrors.

