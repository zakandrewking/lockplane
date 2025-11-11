# Lockplane Roadmap

**Mission:** Make database schema changes safe, understandable, and boring in production.

## Philosophy

Database schema management in production is hard. Teams face:
- Fear of breaking changes
- Unclear rollback paths
- Confusing disaster recovery
- Drift between environments
- Coordination across teams
- Lock contention and downtime

**Lockplane's approach:**
- **Declarative schemas** - Define desired state, not migration scripts
- **Safety-first** - Validate on shadow DB before production
- **Explainable** - Every change has a clear description and rollback plan
- **Disaster recovery patterns** - Standard procedures for backups and point-in-time recovery
- **Multi-database** - PostgreSQL, SQLite, libSQL support

**Integration over invention:** When best-in-class tools exist (like [pgroll](https://github.com/xataio/pgroll) for zero-downtime Postgres migrations), Lockplane helps you adopt them instead of recreating them.

---

## Roadmap Organized by Production Pain Points

### 1. Disaster Recovery and Backup Management

**The Problem:**
When something goes wrong in production, teams panic. Common scenarios:
- Need to restore from backup after a bad migration
- Accidentally dropped data, need to recover specific rows
- Want to query old data without affecting production
- Unclear which backup to use or how to restore safely

Teams waste hours figuring out the right `pg_restore` flags, creating temporary databases, and manually extracting data.

**How Lockplane Solves This:**

- **Today:**
  - Schema introspection captures exact state for recovery planning
  - Automatic rollback plans for migrations
  - Shadow DB pattern establishes best practices for isolated testing

**Roadmap:**
- [ ] **Standard backup recovery patterns**
  - `lockplane backup restore --from <backup> --to <temp-db>` - Restore backup to temporary database
  - `lockplane backup query --from <backup> --read-only` - Mount backup for read-only queries
  - `lockplane backup compare <prod> <backup>` - Diff current state vs. backup to understand what changed
  - Clear documentation: "When to restore vs. query backups"

- [ ] **Point-in-time recovery helpers**
  - `lockplane recover --to <timestamp>` - Find and restore nearest backup
  - Timeline visualization: "Backup A (2 hours ago) → Migration X → Current state"
  - Estimate data loss: "Restoring will lose 1,243 rows inserted after backup"

- [ ] **Data extraction from backups**
  - `lockplane backup extract --table users --where "created_at > '2024-01-01'" --from <backup> --output recovery.sql`
  - Generate INSERT statements to merge backup data into current production
  - Handle schema mismatches: "Backup has old schema, generating compatible inserts"

- [ ] **Backup validation**
  - `lockplane backup validate <backup>` - Test restore on shadow DB
  - Verify backup is complete and restorable
  - Check schema compatibility with current codebase
  - Report: "Backup is 3 migrations behind, will need to migrate after restore"

- [ ] **Recovery runbooks**
  - Generate step-by-step recovery procedures
  - "If migration X fails, here's how to rollback using backup Y"
  - Include verification steps: "Check row counts, verify foreign keys, test queries"

### 2. Backward-Compatible Changes (Expand/Contract Pattern)

**The Problem:**
You can't do "one big DDL + code deploy" in prod. You need multi-step, backward-compatible changes: add new column, backfill, dual-write, flip reads, then drop old stuff. This is evolutionary database design.

**How Lockplane Solves This:**

- **Today:**
  - Declarative schema naturally expresses target state
  - Shadow DB validation catches breaking changes before prod
  - Automatic rollback generation provides safety net
  - Explicit migration plans show each step

**Roadmap:**
- [ ] **Detect breaking changes**
  - Flag operations requiring multi-phase deployments
  - ALTER COLUMN TYPE → needs expand/contract
  - ADD NOT NULL → needs backfill first
  - DROP COLUMN → needs code deploy before schema change
  - Warn: "This change will break running applications"

- [ ] **Multi-phase migration plans**
  - Generate 3-step plans: expand, dual-write phase, contract
  - Step 1: Add new column (nullable)
  - Step 2: Backfill + dual-write (code deployment)
  - Step 3: Make NOT NULL + drop old column

- [ ] **Integration with zero-downtime tools**
  - Generate pgroll migrations for Postgres when needed
  - Generate gh-ost migrations for MySQL when needed
  - Traditional migrations for simple, non-breaking changes
  - Let users choose: "pgroll, traditional DDL, or manual multi-phase"

### 3. Avoiding Locks and Downtime During DDL

**The Problem:**
Many DDL operations take heavyweight locks that block reads/writes. A "simple" ALTER TABLE can stall your whole app. Users get timeouts, requests queue up, and you're scrambling to kill the migration.

**How Lockplane Solves This:**

- **Today:**
  - Validation reports warn about potentially dangerous operations
  - Shadow DB testing lets you measure lock duration before prod
  - Shows exactly what SQL will run

**Roadmap:**
- [ ] **Lock impact analysis**
  - Measure DDL duration on shadow DB with realistic data
  - Report: "ALTER TABLE will hold AccessExclusive lock for ~30 seconds"
  - Estimate: "Will block ~1,500 queries based on current load"
  - Show lock type: AccessExclusive, ShareUpdateExclusive, etc.

- [ ] **Lock-safe rewrites**
  - `CREATE INDEX` → `CREATE INDEX CONCURRENTLY`
  - `ADD CONSTRAINT` → `ADD CONSTRAINT NOT VALID` + `VALIDATE CONSTRAINT`
  - `ALTER COLUMN TYPE` → suggest multi-phase or zero-downtime tool
  - Inject `SET lock_timeout = '2s'` to fail fast instead of blocking

- [ ] **Zero-downtime execution options**
  - For Postgres: offer pgroll integration for lock-heavy operations
  - For MySQL: offer gh-ost integration
  - Traditional DDL for simple, fast operations
  - Let user decide based on lock analysis

### 4. Large Data Migrations & Backfills

**The Problem:**
Schema changes require moving or transforming data: backfilling new columns, splitting/merging tables, migrating to new types. Naive `UPDATE` statements lock tables, time out, or blow transaction logs.

**How Lockplane Solves This:**

- **Today:**
  - Shadow DB validation tests backfill performance on realistic data
  - Explicit migration plans show exactly what SQL runs
  - Detects when operations will require data movement

**Roadmap:**
- [ ] **Backfill performance analysis**
  - Test on shadow DB to estimate duration
  - Report: "Backfill will process ~1M rows, estimated time: 3 minutes"
  - Measure transaction log impact
  - Warn: "This backfill will generate 2GB of WAL logs"

- [ ] **Safe backfill patterns**
  - Generate batched UPDATE scripts for large tables
  - Add progress tracking and resume capability
  - Use: `UPDATE ... WHERE id BETWEEN batch_start AND batch_end`
  - Report: "Processed 100,000/1,000,000 rows (10%)"

- [ ] **Backfill expression generation**
  - For type conversions: `UPDATE SET new_col = CAST(old_col AS newtype)`
  - For computed columns: `UPDATE SET full_name = first_name || ' ' || last_name`
  - For data cleanup: `UPDATE SET status = COALESCE(status, 'pending')`
  - Validate expressions on shadow DB before production

### 5. Rollbacks Are Hard (Stateful System, Irreversible Changes)

**The Problem:**
Rolling back database changes is scary. Once new data has been written in the "new" shape, reverting the schema can cause:
- Data loss (dropped columns with new data)
- Constraint violations (new data doesn't fit old constraints)
- Application errors (code expects old schema but data has new shape)

Unlike code deploys, you can't just "git revert" the database.

**How Lockplane Solves This:**

- **Today:**
  - Automatic rollback plan generation
  - Shadow DB testing validates both forward and rollback paths
  - Explicit SQL shows exactly what rollback will do
  - Source hash validation prevents applying migrations to wrong state

**Roadmap:**
- [ ] **Rollback safety analysis**
  - Classify migrations by rollback safety:
    - ✅ Safe: Add nullable column → DROP COLUMN loses no data
    - ⚠️ Lossy: Add NOT NULL with backfill → rollback loses new data
    - ❌ Dangerous: DROP COLUMN → can't rollback, data is gone
  - Show impact: "Rollback will lose 1,243 rows written to new column"

- [ ] **Rollback testing on shadow DB**
  - Apply forward migration to shadow DB
  - Write test data in new schema shape
  - Apply rollback migration
  - Verify: no errors, expected data preserved/lost as documented

- [ ] **Backup-based rollback**
  - When rollback SQL isn't safe, suggest backup restore
  - `lockplane rollback --via-backup` - restore from pre-migration backup
  - Compare: "SQL rollback: loses 1,243 rows. Backup restore: loses 5 minutes of writes"
  - Generate recovery procedure with both options

### 6. Schema Drift Between Environments

**The Problem:**
Hotfixes in prod, manual SQL in psql, inconsistent migration runs. Dev, staging, and prod have subtly different schemas. Which one is correct? No one knows. Debugging becomes a nightmare.

**How Lockplane Solves This:**

- **Today:**
  - Introspection + diff detects drift: compare desired schema to any environment
  - Declarative source of truth (`desired.json`) eliminates "which migration ran where?" confusion
  - Environment-specific configs in `lockplane.toml`
  - One command to check any environment: `lockplane plan --from-environment prod --to schema/`

**Roadmap:**
- [ ] **Drift detection and reporting**
  - `lockplane drift check --all-environments` - check all environments at once
  - Visual diff: show exactly what's different in each environment
  - Report: "Production has extra column 'emergency_fix_2024_01', not in desired schema"
  - Flag: "Staging missing 3 migrations that prod has"

- [ ] **Drift reconciliation**
  - Generate fix-up migrations to bring drifted environments into alignment
  - `lockplane drift fix --environment staging` - generate migration to match desired
  - Handle manual changes: "Column 'admin_notes' not in schema. Add to schema.json or drop?"

- [ ] **Drift prevention in CI**
  - GitHub Action that checks each environment daily
  - Block deployments if production has drifted from desired state
  - Comment on PRs: "⚠️ This migration will be applied to a drifted database"
  - Require manual review when drift detected

### 7. Catching Dangerous Migrations Before They Hit Prod

**The Problem:**
It's easy to write dangerous migrations:
- DROP COLUMN destroys data permanently
- ALTER COLUMN TYPE without validation breaks apps
- ADD NOT NULL to a table with existing rows fails at runtime
- Missing foreign key cascades cause orphaned data

These errors are caught in production at 2 AM.

**How Lockplane Solves This:**

- **Today:**
  - Validation reports flag risky operations before execution
  - Shadow DB testing catches runtime errors with real data
  - Explicit migration plans force review (every SQL statement is visible)
  - Won't apply migration to wrong database state (source hash check)

**Roadmap:**
- [ ] **Configurable safety levels**
  - `strict`: Block dangerous operations, require multi-phase migrations
  - `moderate`: Warn about risky operations, require confirmation
  - `permissive`: Allow with clear documentation of risks

- [ ] **Dangerous operation catalog**
  - DROP COLUMN → "⚠️ Permanent data loss. Consider deprecation period first."
  - ALTER TYPE without expression → "❌ Will fail if data doesn't fit new type"
  - ADD NOT NULL without DEFAULT → "❌ Will fail on tables with existing rows"
  - DROP CONSTRAINT → "⚠️ Application may assume this constraint exists"
  - Suggest safer alternatives for each

- [ ] **Impact estimation**
  - Test on shadow DB with realistic data
  - Report: "This migration will lock table for 30 seconds"
  - Show affected queries: "15 slow queries will be blocked during migration"
  - Compare options: "Traditional DDL: 30s lock. Multi-phase: no lock. pgroll: zero downtime"

### 8. Testing Migrations Realistically (Prod-Like Data + CI)

**The Problem:**
Migrations pass on your laptop's empty test database, then fail in production:
- "This takes 2ms locally" → "This locks the table for 5 minutes in prod"
- "No errors in tests" → "Data doesn't fit new constraint, migration fails"
- "Works in CI" → "Production has pathological data that breaks the migration"

**How Lockplane Solves This:**

- **Today:**
  - Shadow DB testing on prod-like data before applying to production
  - Environment isolation (dev/stage/prod configs keep environments separate)
  - Validation reports surface issues early
  - Explicit SQL lets you review exact operations

**Roadmap:**
- [ ] **Enhanced shadow DB workflows**
  - `lockplane test --on-shadow-db --with-data` - clone prod data to shadow, test migration
  - Measure actual performance: duration, lock times, transaction log size
  - Compare schemas after: "Shadow DB schema matches expected? ✓"
  - Test rollback on shadow DB with prod-like data

- [ ] **CI integration**
  - GitHub Actions workflow template: test migrations on shadow DB in CI
  - Comment on PRs with test results:
    - Migration duration: "3.2 seconds on 1M rows"
    - Lock analysis: "Holds AccessExclusive lock for 0.8s"
    - Rollback tested: "✓ Rollback successful"
  - Block merge if shadow DB tests fail

- [ ] **Production data sampling**
  - `lockplane shadow prepare --sample-from production` - copy representative sample
  - Preserve data distribution: edge cases, nulls, max lengths
  - Anonymize sensitive data automatically
  - Keep shadow DB fresh: "Sample was taken 3 days ago, may be stale"

- [ ] **Performance regression detection**
  - Compare query plans before/after migration
  - Test common queries on shadow DB: "SELECT * FROM users WHERE email = ?"
  - Report: "Query plan changed, 20% slower after migration"
  - Suggest indexes if needed

### 9. Cross-Team / Cross-Service Coordination

**The Problem:**
Many services, ETL jobs, and reporting tools depend on the same database schema. Coordinating breaking changes is a nightmare:
- API team changes a column, analytics breaks
- Backend adds NOT NULL, mobile app crashes
- Data warehouse assumes old schema, ETL fails
- No one knows who's using what

**How Lockplane Solves This:**

- **Today:**
  - Declarative schema serves as shared source of truth (everyone can see desired state)
  - Migration plans are reviewable artifacts in PRs
  - Explicit descriptions explain *why* changes are made
  - Git history tracks who changed what

**Roadmap:**
- [ ] **Schema ownership tracking**
  - Annotate tables/columns with ownership metadata in schema.json:
    ```json
    {
      "name": "users",
      "owner": "auth-team",
      "consumers": ["api-service", "analytics-etl", "reporting-dashboard"]
    }
    ```
  - Generate CODEOWNERS-style rules for schema changes

- [ ] **Breaking change notifications**
  - Detect breaking changes: DROP COLUMN, ALTER TYPE, ADD NOT NULL
  - Notify affected teams: "#data-platform: 'last_login' column will be removed"
  - Require approval from consumers before merging
  - GitHub/Slack integration

- [ ] **Deprecation workflow**
  - Mark columns as deprecated with removal dates
  - `@deprecated(remove_after="2025-03-01", reason="Use email_verified_at instead")`
  - Track usage: "3 services still query deprecated column 'is_verified'"
  - Auto-generate migration when safe to remove

- [ ] **Schema changelog**
  - Human-readable summary of changes for each environment
  - "Users table: Added email_verified_at column, deprecated is_verified"
  - Link to migration plans and rollback procedures
  - Subscribe to changes: "Notify #analytics when users table changes"

---

## Additional Roadmap Items

### Zero-Downtime Migration Tool Integration

When teams need zero-downtime migrations, integrate with best-in-class tools instead of rebuilding them:

- [ ] **pgroll (Postgres)**
  - Generate pgroll YAML from Lockplane schema diffs
  - Map Lockplane JSON operations to pgroll operations
  - Test pgroll migrations on shadow DB before production
  - `lockplane export --to pgroll` command

- [ ] **gh-ost / pt-online-schema-change (MySQL)**
  - Generate gh-ost command-line args from Lockplane diffs
  - Similar philosophy: delegate to proven tools
  - `lockplane export --to gh-ost` command

- [ ] **Choose your own adventure**
  - Let users decide: traditional DDL, pgroll, manual multi-phase, or other tools
  - Provide safety analysis to inform the decision
  - Don't force a specific approach

### Database Support

**Current:** PostgreSQL, SQLite, libSQL

**Planned:**
- [ ] MySQL/MariaDB - Introspection, diff generation, migration planning
- [ ] CockroachDB - Postgres-compatible, should work with existing driver
- [ ] Edge databases - Turso, Cloudflare D1 (SQLite-compatible)

**Strategy:** For each database, provide introspection, validation, and migration planning. Integrate with database-specific zero-downtime tools when they exist.

### Migration Management

- [ ] **Migration history tracking**
  - Store applied migrations in `lockplane.migrations` table
  - Track: timestamp, schema hash before/after, migration plan, result
  - Query: "What migrations have been applied to this database?"
  - Detect out-of-order applications

- [ ] **Idempotent migrations** - Safe to run multiple times without errors

- [ ] **Partial migrations** - Apply only specific tables or operations

- [ ] **Migration squashing** - Combine many small migrations into one optimized migration

### Developer Experience

- [ ] **Interactive migration builder (TUI)**
  - Step-by-step: define table, add columns, set constraints
  - Generate Lockplane JSON schema
  - Preview migration plan before committing

- [ ] **Schema visualization**
  - Generate ERD diagrams from desired.json
  - Show table relationships, foreign keys
  - Annotate with ownership metadata

- [ ] **Migration templates**
  - Common patterns: add column, split table, normalize data, add soft deletes
  - Generate Lockplane JSON from template
  - Example: `lockplane template add-column --table users --column email_verified`

- [ ] **Watch mode**
  - Auto-regenerate plans when desired.json changes
  - Show diff in terminal
  - Instant feedback loop for schema development

### Database Version Upgrades

Help teams safely upgrade database versions (e.g., Postgres 12 → 16):

- [ ] **Version compatibility checker**
  - Analyze schema for deprecated features in new version
  - Suggest rewrites for deprecated SQL patterns
  - Test schema compatibility on shadow DB running new version

- [ ] **Upgrade validation workflow**
  1. Introspect current schema on old version
  2. Restore backup on new version in shadow DB
  3. Test migrations work on new version
  4. Compare query plans and performance
  5. Generate compatibility report

- [ ] **Extension compatibility**
  - Track extension versions (PostGIS, pg_trgm, pgvector, timescaledb)
  - Flag extensions needing updates before database upgrade
  - Generate upgrade sequence: "Update postgis before Postgres, verify schema after"

### Integration & Ecosystem

- [ ] **Prisma integration**
  - Import Prisma schema → Lockplane JSON
  - Export Lockplane JSON → Prisma schema
  - Bidirectional sync workflow

- [ ] **Alembic integration** (Python)
  - Import Alembic schema models → Lockplane JSON
  - Use Lockplane for validation and testing

- [ ] **Supabase helpers**
  - First-class support for Supabase managed Postgres
  - Handle Supabase-specific tables (auth, storage, realtime)
  - Documentation: "Using Lockplane with Supabase"

- [ ] **GitHub Actions**
  - Pre-built workflow templates
  - Test migrations on shadow DB in CI
  - Comment on PR with validation results
  - Block merge if safety checks fail

- [ ] **Terraform provider** - Manage schemas as infrastructure

### Observability

- [ ] **Migration metrics**
  - Track migration success/failure rates, duration, rollback frequency
  - Performance impact measurements
  - Help teams understand migration patterns

- [ ] **Schema health dashboard**
  - Visualize drift across environments
  - Show migration status and history
  - Track which environments are out of sync

- [ ] **Alerting integrations**
  - Slack/PagerDuty notifications for dangerous operations
  - Alert: "Production schema drifted from desired state"
  - Alert: "Migration failed on shadow DB test"

---

## Documentation Strategy

### Core Guides

- [ ] **"Getting Started with Lockplane"** - Basic workflow, introspect → diff → plan → apply
- [ ] **"Disaster Recovery Playbook"** - Backup restoration, point-in-time recovery, data extraction
- [ ] **"Safe Migration Patterns"** - Expand/contract, multi-phase deploys, zero-downtime strategies
- [ ] **"Migrating from Prisma/Alembic/Flyway"** - Import existing schemas, adopt Lockplane
- [ ] **"Production Checklist"** - Everything to verify before applying migrations

### Reference

- [ ] **Lockplane JSON Schema Reference** - All fields, types, constraints
- [ ] **Safety Checker Rules** - What Lockplane validates, why it matters
- [ ] **Command Reference** - All CLI commands with examples
- [ ] **Environment Configuration** - lockplane.toml and .env files

---

## Success Metrics

**We succeed when teams:**
1. Stop fearing database migrations
2. Catch dangerous operations before production
3. Recover from failures quickly with clear procedures
4. Manage schema drift across environments
5. Coordinate breaking changes across teams

**Indicators of success:**
- Reduced migration-related production incidents
- Faster time to resolve schema issues
- Teams confidently make schema changes
- Clear recovery procedures when things go wrong
- Multi-database teams use Lockplane consistently

---

## Future: lockplane-auth

Define authentication rules once and target them to multiple data stores. `lockplane-auth` compiles a unified policy specification into row-level security policies for relational databases, Firestore security rules, or the closest equivalent that each supported database offers.

This extends Lockplane's philosophy: **provide safety, explainability, and integration with best tools**.

---

## Contributing

See issues tagged with `roadmap` in the GitHub issue tracker. If you have ideas or want to tackle any of these items, open an issue to discuss the approach first.

**Prioritization is driven by:**
1. **Production pain points** - The 9 areas above
2. **Safety and disaster recovery** - Make failures less scary
3. User requests and feedback
4. Multi-database support
5. Developer experience improvements

**Our mission:** Make schema changes safe, understandable, and boring in production.
