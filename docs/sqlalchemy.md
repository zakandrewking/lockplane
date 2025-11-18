# Using Lockplane with SQLAlchemy

Lockplane fits into an SQLAlchemy workflow by treating your models as the source
of truth and letting you reuse Lockplane's shadow database as a scratch
workspace. Here's the minimal flow:

1. Initialize Lockplane (`lockplane init`).
2. Prepare a clean shadow database (`lockplane shadow prepare`).
3. Point SQLAlchemy's `Base.metadata.create_all()` at the provided shadow URL.
4. Generate and apply a migration plan (`lockplane shadow diff` → `lockplane apply`).

Below you'll find detailed steps plus guidance for production rollouts.

## Prerequisites
- SQLAlchemy project targeting PostgreSQL (SQLite/libSQL also work).
- Lockplane CLI (`npx lockplane` or `npm install -g lockplane`).
- Access to the target database and a shadow database (Lockplane can scaffold the latter).

## Part 1: Development Flow (models → database)

### Step 1: Initialize Lockplane
Run the interactive wizard to configure environments and shadow settings:

```bash
lockplane init
```

This creates `lockplane.toml`, `.env.local`, and default schema directories
(`schema/`). The wizard also configures `SHADOW_DATABASE_URL` so Lockplane can
validate migrations safely.

### Step 2: Prepare the Shadow Database
Lockplane exposes a helper to clean the shadow DB and hand you a connection URL:

```bash
lockplane shadow prepare --target-environment local > shadow.json
```

`shadow.json` contains the `shadow_url` (and optional `shadow_schema`). Treat
this as your "scratch" database for the next step.

### Step 3: Run `Base.metadata.create_all()` Against the Shadow URL
Use the connection string from `shadow.json` when constructing your SQLAlchemy
engine. Example (`generate_schema.py`):

```python
import json
from sqlalchemy import create_engine
from models import Base

with open("shadow.json") as f:
    meta = json.load(f)

shadow_engine = create_engine(meta["shadow_url"])
Base.metadata.create_all(shadow_engine)
```

This populates the shadow database with your desired schema, using SQLAlchemy's
normal `create_all()` semantics.

### Step 4: Diff and Apply
Generate a migration plan by comparing the shadow DB (desired schema) to your
real environment:

```bash
# Produce plan.json by diffing shadow → target environment
lockplane shadow diff --target-environment local > plan.json

# Review plan.json if desired, then apply
lockplane apply plan.json --target-environment local
```

Lockplane still performs shadow validation during `apply`, giving you the usual
safety checkpoints. Once you’re finished, release the reservation:

```bash
lockplane shadow release
```

### Optional: Automate With a Makefile
```makefile
.PHONY: shadow migrate
shadow:
	lockplane shadow prepare --target-environment local > shadow.json
	python generate_schema.py
	lockplane shadow diff --target-environment local > plan.json

migrate: shadow
	lockplane apply plan.json --target-environment local --auto-approve
	lockplane shadow release
```

## Part 2: Productionizing the Flow
Once the development loop feels comfortable, formalize deployments:

### 1. Initialize Additional Environments
Use `lockplane init --env-name production --yes` (or the interactive wizard) to
add production settings to `lockplane.toml`. Populate `.env.production` with
`DATABASE_URL`, `SHADOW_DATABASE_URL`, and credentials for your production
shadow database.

### 2. Generate Reviewable Plans
Keep migrations auditable by checking plan files into version control:

```bash
# Inside CI or your local machine
lockplane shadow prepare --target-environment production > shadow.json
python generate_schema.py                # populates the shadow DB
lockplane shadow diff --target-environment production > migration.json
lockplane shadow release

# Optional: rollback plan
lockplane plan-rollback --plan migration.json \
  --from-environment production > rollback.json

# Review + commit
git add migration.json rollback.json
```

Developers familiar with Alembic can think of `migration.json` as the reviewed
DDL script, except Lockplane can regenerate it deterministically from the
current schema state. You can require code review on plan files just as you
would for manually written migrations.

### 3. Deploy
Apply plans during release with the target environment specified:

```bash
lockplane apply migration.json --target-environment production
```

Because Lockplane already diffed against the prepared shadow DB, you can skip
`shadow prepare` during rollout or rerun it to double-check. If needed, roll back
using the stored `rollback.json`.

## Mental Model for SQLAlchemy/Alembic Users
- **Models stay authoritative.** Continue editing ORM classes and calling
  `Base.metadata.create_all()`. Lockplane replaces hand-written Alembic scripts
  with deterministic diffing between “what you created in shadow” and “what’s in
  production.”
- **Plan files replace migration scripts.** `migration.json` serves the same
  purpose as a checked-in Alembic revision, but Lockplane computes the SQL and
  validates it on a clean shadow DB.
- **Shadow helpers remove friction.** Instead of provisioning extra scratch
  databases, `lockplane shadow prepare` resets the existing shadow DB and hands
  you a connection string for ORMs or schema-code generators.
- **Production confidence.** Shadow validation plus optional rollback plans mean
  you retain the safety nets familiar from Alembic’s migration history.

## Summary
1. `lockplane init`
2. `lockplane shadow prepare`
3. `Base.metadata.create_all(shadow_engine)`
4. `lockplane shadow diff ...` → `lockplane apply ...`

This keeps SQLAlchemy models “the source of truth” while letting Lockplane handle
migration planning, validation, and rollback.
