# Lockplane Roadmap

**Mission:** Help teams safely manage database schemas in production—and help them adopt best-in-class tools like [pgroll](https://github.com/xataio/pgroll) when they need zero-downtime migrations.

## Philosophy: Don't Recreate, Integrate

**pgroll** is the state-of-the-art for zero-downtime Postgres migrations. It uses schema versioning (via views) and expand/contract patterns to keep applications running during schema changes.

**Lockplane's role:**
- **Schema authoring** - Define schemas declaratively, not as migration scripts
- **Migration planning** - Generate migration plans (including pgroll migrations) from schema diffs
- **Validation & testing** - Test migrations on shadow databases before production
- **Multi-database support** - SQLite, libSQL, eventually MySQL (pgroll is Postgres-only)
- **Adoption path** - Start simple, graduate to pgroll when you need zero-downtime

---

## Core Strategy: Lockplane + pgroll Integration

### Current State (Lockplane Today)

Lockplane provides:
- Declarative JSON schema format (`desired.json`)
- Schema introspection from live databases
- Diff generation (compare desired vs. actual)
- Migration plan generation
- Automatic rollback plans
- Shadow DB validation
- Multi-database support (PostgreSQL, SQLite, libSQL)

### Target State (Lockplane + pgroll)

```
┌─────────────────────────────────────────────────────────────┐
│  1. Author schema in Lockplane JSON                        │
│     (easier than writing migration files by hand)          │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  2. Lockplane generates diff & validates safety             │
│     - Shadow DB testing                                     │
│     - Detect dangerous operations                           │
│     - Estimate impact                                       │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  3. Choose execution path:                                  │
│                                                             │
│     Simple change (add nullable column, create table)?      │
│     → Use traditional Lockplane apply                       │
│                                                             │
│     Need zero-downtime (alter column, add NOT NULL)?       │
│     → Export to pgroll format                               │
│     → pgroll start migration.yaml                           │
│     → Deploy new app version                                │
│     → pgroll complete                                       │
└─────────────────────────────────────────────────────────────┘
```

**Benefits:**
- Better authoring experience than writing pgroll YAML by hand
- Validation before execution
- Flexibility to use simple migrations when zero-downtime isn't needed
- Clear upgrade path: start with Lockplane, add pgroll when needed

---

## Roadmap Organized by Production Pain Points

Each section shows how Lockplane + pgroll address real production challenges.

### 1. Backward-Compatible Changes (Expand/Contract Pattern)

**The Problem:**
You can't do "one big DDL + code deploy" in prod. You need multi-step, backward-compatible changes: add new column, backfill, dual-write, flip reads, then drop old stuff. This is evolutionary database design.

**How pgroll Solves This:**
- Expand/contract pattern is pgroll's core strength
- Schema versioning via views allows old and new versions to coexist
- Automatic dual-write triggers keep old and new columns in sync
- `pgroll start` → expand, `pgroll complete` → contract

**How Lockplane Helps:**
- **Today:**
  - Declarative schema naturally expresses target state without manual expand/contract steps
  - Shadow DB validation catches breaking changes before prod
  - Automatic rollback generation provides safety net

**Roadmap:**
- [ ] **Generate pgroll migrations from Lockplane diffs**
  - Detect operations that require expand/contract (alter column type, add NOT NULL, etc.)
  - Generate pgroll YAML with correct `up`/`down` expressions
  - Map Lockplane JSON types to pgroll operations

- [ ] **Smart execution mode selection**
  - Analyze migration: "This can be done with simple DDL" vs. "This needs pgroll"
  - Flag operations that benefit from zero-downtime:
    - ALTER COLUMN TYPE
    - ADD CONSTRAINT NOT NULL
    - DROP COLUMN with data
    - Rename operations
  - Recommend: "Use pgroll for this migration"

- [ ] **Validate pgroll migrations before execution**
  - Parse pgroll YAML and test on shadow DB
  - Verify `up`/`down` expressions work correctly
  - Measure impact of triggers and view overhead

### 2. Avoiding Locks and Downtime During DDL

**The Problem:**
Many Postgres DDL operations take heavyweight locks that block reads/writes. A "simple" ALTER TABLE can stall your whole app.

**How pgroll Solves This:**
- Zero-downtime migrations are pgroll's primary goal
- Schema versioning via views eliminates locking on ALTER COLUMN
- Expand/contract pattern means no blocking DDL on critical path

**How Lockplane Helps:**
- **Today:**
  - Validation reports warn about potentially dangerous operations
  - Shadow DB testing lets you measure lock duration before prod

**Roadmap:**
- [ ] **Automatic pgroll routing for lock-heavy operations**
  - Detect DDL that would lock tables for significant time
  - Auto-suggest pgroll execution path
  - Show comparison: "Traditional: 30s lock. pgroll: zero downtime"

- [ ] **Lock-safe rewrites for non-pgroll migrations**
  - For simple migrations that don't need full pgroll:
    - `CREATE INDEX` → `CREATE INDEX CONCURRENTLY`
    - `ADD CONSTRAINT` → `ADD CONSTRAINT NOT VALID` + `VALIDATE CONSTRAINT`
  - Inject `SET lock_timeout` for safety

- [ ] **Operation timing estimates from shadow DB**
  - Measure DDL duration on shadow DB
  - Report expected lock times
  - Help decide: "Is pgroll worth the complexity for this change?"

### 3. Large Data Migrations & Backfills

**The Problem:**
Schema changes require moving or transforming data: backfilling new columns, splitting/merging tables. Doing this online requires chunked updates and careful transaction management.

**How pgroll Solves This:**
- Automatic column backfilling with `up` expressions
- Dual-write triggers handle ongoing writes during backfill
- No manual batching needed for most cases

**How Lockplane Helps:**
- **Today:**
  - Shadow DB validation tests backfill performance on realistic data
  - Explicit migration plans show exactly what SQL runs

**Roadmap:**
- [ ] **Generate pgroll `up`/`down` expressions**
  - For computed columns: `up: "expression"`, `down: "reverse"`
  - For type conversions: `up: "CAST(col AS newtype)"`
  - For splits: generate expressions to populate new columns from old

- [ ] **Backfill performance estimation**
  - Test on shadow DB to estimate duration
  - Report: "Backfill will process ~1M rows, estimated time: 3 minutes"
  - Warn if backfill expression is expensive (full table scan)

- [ ] **Batched backfill mode for massive tables**
  - When pgroll's automatic backfill would be too slow
  - Generate chunked UPDATE scripts with progress tracking
  - Coordinate with pgroll: backfill before `pgroll complete`

### 4. Rollbacks Are Hard (Stateful System, Irreversible Changes)

**The Problem:**
DB changes aren't trivially reversible once new data has been written in the "new" shape (dropped columns, destructive transforms).

**How pgroll Solves This:**
- Instant rollback with `pgroll rollback`
- Old schema version remains active until `pgroll complete`
- No data loss during rollback before completion

**How Lockplane Helps:**
- **Today:**
  - Automatic rollback generation for traditional migrations
  - Shadow DB testing validates both forward and rollback

**Roadmap:**
- [ ] **Rollback safety analysis**
  - Flag migrations where rollback would be lossy:
    - "⚠️ After pgroll complete, rollback requires new migration"
    - "✅ Safe to rollback: pgroll maintains old schema until complete"
  - Show data loss risk: "Rollback drops column with N rows of new data"

- [ ] **Generate traditional rollback when pgroll isn't available**
  - For non-Postgres databases
  - For simple migrations that don't use pgroll
  - Document point-in-time recovery as alternative

- [ ] **pgroll completion safety checks**
  - Before recommending `pgroll complete`, verify old schema is unused
  - Check application deployments: "3 pods still using old schema version"
  - Suggest wait time: "Safe to complete in 1 hour when old pods terminate"

### 5. Schema Drift Between Environments

**The Problem:**
Hotfixes in prod, manual psql changes, and inconsistent migration runs across dev/stage/prod cause drift. Debugging becomes painful.

**How pgroll Helps:**
- Version tracking in `pgroll.internal_schema` table
- Clear migration history

**How Lockplane Helps:**
- **Today:**
  - Introspection + diff detects drift: compare desired schema to any environment
  - Declarative source of truth (`desired.json`) eliminates "which migration ran where?" confusion
  - Environment-specific configs in `lockplane.toml`

**Roadmap:**
- [ ] **Drift detection across Lockplane + pgroll**
  - Read `pgroll.internal_schema` to understand pgroll state
  - Introspect actual schema (views + physical tables)
  - Compare to desired state
  - Report: "Stage has 3 uncommitted pgroll migrations"

- [ ] **Drift reconciliation**
  - Generate fix-up migrations to bring drifted environments back
  - Detect manual schema changes outside pgroll
  - Suggest: "Convert manual changes to Lockplane JSON + generate pgroll migration"

- [ ] **CI workflow for drift detection**
  - GitHub Action that checks each environment daily
  - Flag: "Production schema doesn't match desired.json"
  - Block deployments if critical drift detected

### 6. Catching Dangerous Migrations Before They Hit Prod

**The Problem:**
Dropping columns, changing types, adding NOT NULL can all be dangerous in production. Easy to shoot yourself in the foot.

**How pgroll Helps:**
- Enforces safe patterns through its operation model
- Can't drop columns in unsafe ways (expand/contract prevents this)

**How Lockplane Helps:**
- **Today:**
  - Validation reports flag risky operations
  - Shadow DB testing catches issues before prod
  - Explicit migration plans force review

**Roadmap:**
- [ ] **Safety levels with pgroll recommendations**
  - `strict`: Require pgroll for all breaking changes
  - `moderate`: Warn "This could use pgroll for zero-downtime"
  - `permissive`: Allow traditional migrations with confirmation

- [ ] **Dangerous operation detection + pgroll suggestions**
  - DROP COLUMN → "Use pgroll to safely deprecate this column first"
  - ALTER COLUMN TYPE → "pgroll can do this with zero downtime"
  - ADD NOT NULL → "pgroll handles backfill + constraint safely"
  - Show side-by-side: traditional risk vs. pgroll safety

- [ ] **Impact estimation**
  - Test on shadow DB
  - Report: "Traditional: locks table for 30s. pgroll: zero downtime"
  - Help users make informed decisions

### 7. Testing Migrations Realistically (Prod-Like Data + CI)

**The Problem:**
Migrations pass in CI but fail on real prod datasets due to data volume, skew, or pathological rows.

**How pgroll Helps:**
- Can test `pgroll start` on staging before production
- Shadow schema versions don't affect production

**How Lockplane Helps:**
- **Today:**
  - Shadow DB testing on prod-like data before applying anywhere
  - Environment isolation (dev/stage/prod configs)
  - Validation reports surface issues early

**Roadmap:**
- [ ] **Test pgroll migrations on shadow DB**
  - Run `pgroll start` on shadow DB first
  - Verify `up`/`down` expressions work with real data
  - Measure trigger overhead and view query performance
  - Test rollback path

- [ ] **CI integration for Lockplane + pgroll workflow**
  - Documented patterns for GitHub Actions, GitLab CI
  - Auto-generate pgroll migrations in CI
  - Test on shadow DB
  - Comment on PR with safety report and pgroll YAML

- [ ] **Performance regression detection**
  - Compare query plans before/after migration
  - Test queries through pgroll views vs. direct tables
  - Warn: "View adds 10ms overhead to SELECT queries"

### 8. Cross-Team / Cross-Service Coordination

**The Problem:**
Many apps/ETL jobs/reporting systems depend on the same Postgres schema. Coordinating breaking changes across teams is hard.

**How pgroll Helps:**
- Schema versioning allows teams to migrate independently
- Old and new versions coexist during transition
- Teams can test against new schema before committing

**How Lockplane Helps:**
- **Today:**
  - Declarative schema serves as shared source of truth
  - Migration plans are reviewable artifacts in PRs
  - Explicit descriptions explain *why* changes are made

**Roadmap:**
- [ ] **Schema ownership annotations in Lockplane JSON**
  ```json
  {
    "tables": [
      {
        "name": "users",
        "owner": "auth-team",
        "consumers": ["api-service", "analytics-etl"],
        "breaking_change_notifications": ["#eng-auth", "#data-platform"]
      }
    ]
  }
  ```

- [ ] **Deprecation timelines with pgroll coordination**
  - Mark columns as deprecated with removal dates
  - Generate pgroll migration that maintains both versions
  - Notify consumers: "You have 30 days to migrate to new schema version"
  - Auto-check: "3 services still using old schema version"

- [ ] **Changelog generation**
  - Human-readable summary of schema changes
  - Explain pgroll migration phases
  - Show which teams need to act: "API team: update to use new column"

---

## Additional Roadmap Items

### pgroll Integration (Core Focus)

- [ ] **Lockplane → pgroll migration generator**
  - Parse Lockplane JSON schema diff
  - Generate pgroll YAML/JSON with operations
  - Map types, constraints, indexes to pgroll format
  - Generate `up`/`down` SQL expressions for backfills

- [ ] **pgroll → Lockplane schema importer**
  - Read pgroll migration history
  - Reconstruct desired schema state
  - Import as Lockplane JSON
  - Use case: "I'm using pgroll, want to add Lockplane tooling"

- [ ] **Bidirectional sync**
  - Keep Lockplane JSON in sync with pgroll migrations
  - Watch pgroll migrations, update desired.json
  - Detect manual pgroll migrations, incorporate into Lockplane

- [ ] **pgroll operation coverage**
  - Support all pgroll operations:
    - create_table, drop_table, rename_table
    - add_column, drop_column, alter_column, rename_column
    - create_index, drop_index
    - add_constraint, drop_constraint
    - sql (raw SQL operations)
  - Map each to Lockplane schema primitives

- [ ] **pgroll execution helpers**
  - `lockplane pgroll start` - wrap pgroll CLI
  - `lockplane pgroll status` - show migration state
  - `lockplane pgroll complete` - finalize with safety checks
  - `lockplane pgroll rollback` - revert with analysis

### Database Support

**Current:** PostgreSQL, SQLite, libSQL

**Planned:**
- [ ] **Postgres (with pgroll)** - First-class integration, generate pgroll migrations
- [ ] **Postgres (without pgroll)** - Traditional migrations for simple changes
- [ ] SQLite / libSQL - Traditional migrations (no zero-downtime option)
- [ ] MySQL/MariaDB - Explore gh-ost or pt-online-schema-change integration
- [ ] CockroachDB - Research zero-downtime migration tools in ecosystem

**Strategy:** For each database, integrate with its best-in-class migration tool rather than reinventing.

### Migration Management

- [ ] **Migration history tracking**
  - For traditional migrations: store in `lockplane.migrations` table
  - For pgroll: read from `pgroll.internal_schema`
  - Unified view across both

- [ ] **Idempotent migrations** - Safe to run multiple times

- [ ] **Partial migrations** - Apply only specific tables or operations

- [ ] **Migration squashing** - Combine many small migrations into one optimized migration (useful before pgroll adoption)

### Developer Experience

- [ ] **Interactive migration builder (TUI)**
  - Step-by-step: define table, add columns, set constraints
  - Generate both Lockplane JSON and pgroll YAML
  - Preview migration plan before committing

- [ ] **Schema visualization**
  - Generate ERD diagrams from desired.json
  - Show table relationships, foreign keys
  - Annotate with ownership and pgroll migration status

- [ ] **Migration templates**
  - Common patterns: add column, split table, normalize data
  - Generate Lockplane JSON + pgroll YAML from template
  - Example: "Add soft delete column to table"

- [ ] **Watch mode**
  - Auto-regenerate plans when desired.json changes
  - Show diff in terminal
  - Instant feedback loop

### Database Version Upgrades

Help teams safely upgrade Postgres versions (12 → 13 → 14 → 15 → 16):

- [ ] **Version compatibility checker**
  - Analyze schema for deprecated features
  - Check pgroll compatibility with new Postgres version
  - Suggest rewrites for deprecated patterns

- [ ] **Upgrade validation workflow**
  1. Introspect current schema + pgroll state on old version
  2. Test restore on new Postgres version in shadow DB
  3. Verify pgroll operations work correctly
  4. Compare query plans and performance
  5. Generate compatibility report

- [ ] **Extension compatibility**
  - Track extension versions (PostGIS, pg_trgm, pgvector, timescaledb)
  - Flag extensions needing updates before Postgres upgrade
  - Generate upgrade sequence

### Integration & Ecosystem

- [ ] **Prisma integration**
  - Import Prisma schema → Lockplane JSON
  - Export Lockplane migrations → Prisma migrations
  - Or export Lockplane → pgroll for zero-downtime

- [ ] **Alembic integration** (Python)
  - Import Alembic schema
  - Export to pgroll for zero-downtime Postgres

- [ ] **Supabase helpers**
  - First-class support for Supabase managed Postgres
  - Guide: "Using pgroll with Supabase"
  - Handle Supabase-specific tables (auth, storage)

- [ ] **GitHub Actions**
  - Pre-built workflow: generate pgroll migrations in CI
  - Test on shadow DB
  - Comment on PR with validation results
  - Auto-merge if safety checks pass

- [ ] **Terraform provider** - Manage schemas as infrastructure

### Observability

- [ ] **Migration metrics**
  - Track pgroll vs. traditional migration usage
  - Success/failure rates, duration, rollback frequency
  - Cost analysis: "pgroll adds 10ms query overhead but zero downtime"

- [ ] **Schema health dashboard**
  - Visualize drift, migration status, pgroll version status
  - Show which environments have uncommitted migrations
  - Track: "3 tables still on old pgroll schema version"

- [ ] **Alerting integrations**
  - Slack/PagerDuty notifications for dangerous operations
  - Alert: "Production schema drifted from desired state"
  - Alert: "pgroll migration stuck in incomplete state"

---

## Documentation Strategy

### Core Guides

- [ ] **"Getting Started with Lockplane"** - Basic workflow, introspect → diff → plan → apply
- [ ] **"When to Use pgroll"** - Decision matrix, examples of migrations that benefit
- [ ] **"Lockplane + pgroll Tutorial"** - End-to-end workflow, schema change → pgroll generation → execution
- [ ] **"Migrating from Prisma/Alembic/Flyway"** - Import existing schemas, adopt Lockplane + pgroll
- [ ] **"Zero-Downtime Migration Patterns"** - Expand/contract recipes, pgroll best practices

### Reference

- [ ] **Lockplane JSON Schema Reference** - All fields, types, constraints
- [ ] **pgroll Operation Mapping** - How Lockplane operations map to pgroll
- [ ] **Safety Checker Rules** - What Lockplane validates, why it matters
- [ ] **Command Reference** - All CLI commands with examples

---

## Success Metrics

**Adoption goals:**
1. Users start with Lockplane for schema authoring and validation
2. Users graduate to pgroll when they need zero-downtime (with Lockplane generating migrations)
3. Users continue using Lockplane for multi-database support (SQLite, MySQL)
4. Users advocate for pgroll because Lockplane made it accessible

**We succeed when:**
- Teams say "Lockplane makes pgroll easy to adopt"
- pgroll usage increases because Lockplane lowers the barrier
- Users choose Lockplane + pgroll over proprietary migration tools
- The Postgres ecosystem has better zero-downtime migration practices

---

## Future: lockplane-auth

Define authentication rules once and target them to multiple data stores. `lockplane-auth` compiles a unified policy specification into row-level security policies for relational databases, Firestore security rules, or the closest equivalent that each supported database offers.

This extends Lockplane's philosophy: **don't recreate best-in-class tools, help users adopt them**.

---

## Contributing

See issues tagged with `roadmap` in the GitHub issue tracker. If you have ideas or want to tackle any of these items, open an issue to discuss the approach first.

**Prioritization is driven by:**
1. **pgroll integration** - Core focus, highest priority
2. Production pain points (the 8 areas above)
3. User requests and feedback
4. Multi-database support
5. Developer experience improvements

**Our mission:** Make schema changes safe and boring. Help teams adopt pgroll. Don't reinvent wheels.
