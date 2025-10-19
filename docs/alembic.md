# Using Lockplane with Alembic

Alembic (SQLAlchemy's migration tool) excels at Python-based migrations. Lockplane adds declarative diffs, JSON validation, and reversible plans. Use them together to keep your migrations safe while retaining Alembic's scripting flexibility.

## Prerequisites

- Python 3.11+
- SQLAlchemy + Alembic (`pip install alembic sqlalchemy psycopg2-binary`)
- Lockplane CLI installed
- Access to your Postgres database for both tools

## Hybrid Workflow Overview

1. **Track schema declaratively with Lockplane**
   - `current.json` represents the state currently deployed.
   - `schema.lp.sql` (preferred, or a directory of `.lp.sql` files) describes the target state. Convert from JSON if needed with `lockplane convert`.

2. **Use Lockplane for safety analysis**
   - `lockplane plan` highlights unsafe operations and generates SQL/rollback steps.
   - Feed those steps into Alembic migration scripts when you need custom logic.

3. **Apply with Alembic**
   - Convert Lockplane plan steps into Alembic `op.execute` or `op.create_table` calls.
   - Keep Lockplane in the loop by introspecting after Alembic runs to confirm convergence.

## Step-by-Step

### 1. Initialize Alembic
```bash
alembic init migrations
```
Configure `sqlalchemy.url` in `alembic.ini` to match the `DATABASE_URL` you use for Lockplane.

### 2. Capture current schema with Lockplane
```bash
lockplane introspect > current.json
```

### 3. Define desired schema
Update `schema.lp.sql` to express the new state. Validate immediately:
```bash
lockplane validate schema schema.lp.sql
```

### 4. Generate a plan and review
```bash
lockplane plan --from current.json --to schema.lp.sql --validate > migration.json
```
- Read the stderr validation summary for warnings.
- `migration.json` contains ordered `steps` with SQL.
- Optionally generate a rollback:
  ```bash
  lockplane rollback --plan migration.json --from current.json > rollback.json
  ```

### 5. Author Alembic revision using Lockplane SQL
```bash
alembic revision -m "sync with lockplane"
```
Edit the new file in `migrations/versions/`:
```python
from alembic import op


def upgrade():
    op.execute("""
    -- paste SQL from migration.json step 1
    """)
    op.execute("""
    -- step 2, etc.
    """)


def downgrade():
    op.execute("""
    -- paste SQL from rollback.json (reverse order)
    """)
```
Use `op.create_table`/`op.add_column` where convenient; Lockplane SQL serves as a verified template.

### 6. Test against a shadow database
Set `SHADOW_DATABASE_URL` to your staging database or an ephemeral container and run:
```bash
lockplane apply --plan migration.json --skip-shadow
```
- Or, run `lockplane apply` without `--skip-shadow` to let it dry-run on the shadow, then perform `alembic upgrade head` once confident.

### 7. Keep artifacts in sync
- After Alembic upgrade succeeds, run `lockplane introspect > current.json` and commit alongside the Alembic revision to document the new state.
- If Alembic includes custom Python logic (data migrations), add notes alongside `schema.lp.sql` (or in README) to ensure future maintainers rerun those scripts.

## Automation Tips

- Add a CI step that calls `lockplane plan --validate` and `alembic upgrade --sql head` to ensure both toolchains agree.
- Use Makefile targets:
  ```makefile
  lockplane-plan:
	lockplane plan --from current.json --to schema.lp.sql --validate > migration.json

  alembic-upgrade:
  	alembic upgrade head
  ```
- Consider storing `migration.json`/`rollback.json` next to the Alembic revision to document the generated SQL.

## Troubleshooting

- **Autogenerate vs. Lockplane:** Alembic autogenerate may produce different SQL than Lockplane. Prefer Lockplane’s SQL for consistency; adjust Alembic autogen output or disable it for the revision.
- **Transactional differences:** Lockplane assumes transactional DDL; if your Alembic revision requires non-transactional steps, split them into separate revisions and note it in the Lockplane plan.
- **Data migrations:** Run Python data migrations within the Alembic revision; Lockplane focuses on structural changes. Document any required data steps in the PR description alongside `schema.lp.sql` updates.
