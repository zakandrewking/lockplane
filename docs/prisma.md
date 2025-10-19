# Using Lockplane with Prisma

Prisma and Lockplane both target PostgreSQL, so you can combine them to keep your application models and database schema aligned. This guide shows a workflow where Prisma stays the source of truth for application models while Lockplane generates, validates, and executes SQL changes.

## Prerequisites

- Node.js 18+
- `npm` or `pnpm`
- Prisma CLI (`npm install prisma --save-dev`)
- Lockplane CLI (`go install ./...` from this repo or download a release)
- Access to your target Postgres instance

## Recommended Workflow

1. **Model changes in Prisma**
   - Update `schema.prisma` with the new models or field changes.
   - Run `npx prisma format` to keep the schema tidy.

2. **Use Prisma to generate SQL**
   - Run `npx prisma migrate diff --from-empty --to-schema-datamodel schema.prisma --script > prisma.sql` for a SQL draft, or use `prisma migrate dev` to update a dev database.

3. **Introspect with Lockplane**
   - Capture the current database state:
     ```bash
     lockplane introspect --database-url "$DATABASE_URL" > current.json
     ```
   - If you used `prisma migrate dev`, run it against the same database Prisma updated so the diff is accurate.

4. **Capture the desired schema (`.lp.sql` preferred)**
   - Introspect the updated database to JSON, then convert it to SQL DDL:
     ```bash
     lockplane introspect --database-url "$DATABASE_URL" > desired.json
     lockplane convert --input desired.json --output schema.lp.sql --to sql
     ```
   - Commit `schema.lp.sql` as the declarative source of truth that mirrors your Prisma models. Keep `desired.json` only if other tooling still expects JSON.

5. **Validate and plan**
   - Generate and validate the migration plan:
     ```bash
     lockplane plan --from current.json --to schema.lp.sql --validate > migration.json
     ```
   - Review the validation report for safety/reversibility notes.

6. **Apply via Lockplane**
   - After validation, apply with shadow testing:
     ```bash
     lockplane apply --plan migration.json
     ```
   - Prisma stays responsible for generating client types (`npx prisma generate`).

## Tips

- Add `schema-json/schema.json` to VS Code settings so Prisma schema edits surface JSON schema warnings when you sync into Lockplane (after converting to JSON with `lockplane convert`).
- Run `lockplane validate schema schema.lp.sql` (Lockplane will auto-detect the format) in CI right after `npx prisma format` to catch drift early.
- If Prisma introduces functions or extensions, ensure they appear in `schema.lp.sql` before running Lockplane validation.

## CI Example

```yaml
name: prisma-lockplane
on: [push]
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: npm ci
      - run: npx prisma generate
      - run: go install ./...
      - run: lockplane validate schema schema.lp.sql
      - run: lockplane plan --from current.json --to schema.lp.sql --validate
```
