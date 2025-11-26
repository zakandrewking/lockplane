# Database Version Requirements and Detection

## Progress Checklist
- [x] Phase 1: Document current version-dependent features
- [ ] Phase 2: Design version detection system
- [ ] Phase 3: Implement graceful degradation
- [ ] Phase 4: Add user-facing documentation
- [ ] Phase 5: Add runtime version checks
- [ ] Phase 6: Update error messages with version hints

## Context

Lockplane uses database-specific features that have minimum version requirements. Currently, we don't validate database versions at runtime, which can lead to cryptic errors when users run Lockplane against older database versions.

### Current Version Dependencies

**SQLite:**
- **DROP COLUMN**: Requires SQLite 3.35.0+ (released March 2021)
  - Used in: `database/sqlite/generator.go:68-72`
  - Older versions require table recreation pattern
- **RENAME COLUMN**: Requires SQLite 3.25.0+ (released September 2018)
- **Foreign key support**: Available since 3.6.19 (2009), but must be enabled with `PRAGMA foreign_keys = ON`

**PostgreSQL:**
- **DROP COLUMN IF EXISTS**: PostgreSQL 8.2+ (released December 2006)
- **IF NOT EXISTS clauses**: PostgreSQL 9.1+ for most operations (released September 2011)
- **Generated columns**: PostgreSQL 12+ (released October 2019)
- Generally, we target PostgreSQL 12+ for modern features

**libSQL/Turso:**
- Fork of SQLite 3.43.0+
- Includes all SQLite 3.35.0+ features
- Additional distributed database features

## Problem Statement

**Without version detection, users experience:**
1. **Cryptic runtime errors** - "syntax error near DROP" doesn't tell users they need SQLite 3.35+
2. **Silent feature degradation** - Older SQLite versions might silently skip DROP COLUMN
3. **Unclear compatibility** - README/docs don't specify minimum versions
4. **Migration failures** - Plans fail mid-execution on unsupported syntax
5. **No graceful fallback** - Can't automatically use table recreation for older SQLite

**Example failure:**
```
$ lockplane apply
Error: near "DROP": syntax error
```

Should be:
```
$ lockplane apply
Error: DROP COLUMN requires SQLite 3.35.0+, but you have 3.30.1
Hint: Upgrade SQLite or set LOCKPLANE_SQLITE_USE_TABLE_RECREATION=1
```

## Goals

1. **Clear minimum version requirements** in documentation
2. **Runtime version detection** for all database types
3. **Graceful degradation** where possible (e.g., table recreation for old SQLite)
4. **Helpful error messages** with upgrade paths
5. **CI/CD compatibility checks** to prevent regressions

## Non-Goals

- Supporting ancient database versions (e.g., PostgreSQL 9.x)
- Backporting modern features to old databases
- Automatic database upgrades (users must upgrade themselves)

## Implementation Phases

### Phase 1: Document Minimum Versions (CURRENT)

**Status:** Documenting existing dependencies

**Requirements:**
- SQLite: 3.35.0+ (for DROP COLUMN support)
- PostgreSQL: 12+ (for generated columns, modern DDL)
- libSQL/Turso: Latest stable (inherits SQLite 3.43.0+)

**Documentation updates needed:**
- `README.md` - Add "Database Requirements" section
- `docs/getting_started.md` - Add version check step
- Error messages - Include version hints

### Phase 2: Runtime Version Detection

**Design:**

Add version detection to each driver:

```go
// database/interface.go
type Driver interface {
    // ... existing methods ...

    // GetVersion returns the database version
    GetVersion(ctx context.Context, db *sql.DB) (string, error)

    // CheckMinimumVersion verifies the database meets minimum requirements
    CheckMinimumVersion(ctx context.Context, db *sql.DB) error
}
```

**Implementation:**

```go
// database/sqlite/driver.go
func (d *Driver) GetVersion(ctx context.Context, db *sql.DB) (string, error) {
    var version string
    err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
    return version, err
}

func (d *Driver) CheckMinimumVersion(ctx context.Context, db *sql.DB) error {
    version, err := d.GetVersion(ctx, db)
    if err != nil {
        return fmt.Errorf("failed to get SQLite version: %w", err)
    }

    // Parse version (e.g., "3.35.5" -> [3, 35, 5])
    parts := strings.Split(version, ".")
    if len(parts) < 2 {
        return fmt.Errorf("invalid SQLite version format: %s", version)
    }

    major, _ := strconv.Atoi(parts[0])
    minor, _ := strconv.Atoi(parts[1])

    if major < 3 || (major == 3 && minor < 35) {
        return fmt.Errorf("SQLite %s detected. Lockplane requires SQLite 3.35.0+ for DROP COLUMN support. "+
            "Please upgrade SQLite or set LOCKPLANE_SQLITE_USE_TABLE_RECREATION=1 to use table recreation pattern.", version)
    }

    return nil
}
```

**PostgreSQL version detection:**

```go
// database/postgres/driver.go
func (d *Driver) GetVersion(ctx context.Context, db *sql.DB) (string, error) {
    var version string
    err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
    // Returns: "PostgreSQL 15.3 on x86_64-pc-linux-gnu..."
    return version, err
}

func (d *Driver) CheckMinimumVersion(ctx context.Context, db *sql.DB) error {
    version, err := d.GetVersion(ctx, db)
    if err != nil {
        return fmt.Errorf("failed to get PostgreSQL version: %w", err)
    }

    // Extract major version (e.g., "PostgreSQL 15.3..." -> 15)
    re := regexp.MustCompile(`PostgreSQL (\d+)\.(\d+)`)
    matches := re.FindStringSubmatch(version)
    if len(matches) < 2 {
        return fmt.Errorf("failed to parse PostgreSQL version: %s", version)
    }

    major, _ := strconv.Atoi(matches[1])
    if major < 12 {
        return fmt.Errorf("PostgreSQL %d detected. Lockplane requires PostgreSQL 12+ for modern DDL features. "+
            "Current version: %s", major, version)
    }

    return nil
}
```

**When to check:**
- On `lockplane introspect` (first command users run)
- On `lockplane apply` before executing migrations
- On `lockplane plan` if generating SQL
- Optional: Add `lockplane check-version` command

### Phase 3: Graceful Degradation for SQLite

**Problem:** SQLite <3.35.0 doesn't support DROP COLUMN

**Solution:** Automatically use table recreation pattern

```go
// database/sqlite/generator.go
func (g *Generator) DropColumn(tableName string, col database.Column) (string, string) {
    // Check if we should use table recreation (set via env var or detected version)
    if g.shouldUseTableRecreation() {
        return g.dropColumnViaTableRecreation(tableName, col)
    }

    // Modern SQLite: use DROP COLUMN directly
    sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, col.Name)
    description := fmt.Sprintf("Drop column %s from table %s", col.Name, tableName)
    return sql, description
}

func (g *Generator) shouldUseTableRecreation() bool {
    // Check environment variable override
    if os.Getenv("LOCKPLANE_SQLITE_USE_TABLE_RECREATION") == "1" {
        return true
    }

    // Check detected version (requires passing version info to generator)
    if g.version != nil && g.version.IsOlderThan(3, 35, 0) {
        return true
    }

    return false
}
```

**Table recreation pattern:**
```sql
-- Drop column from users table (compatible with SQLite <3.35.0)
CREATE TABLE users_new (id INTEGER PRIMARY KEY, email TEXT NOT NULL);
INSERT INTO users_new SELECT id, email FROM users;
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;
```

**Implementation approach:**
1. Add `Version` field to `Generator` struct
2. Pass version during driver initialization
3. Automatically choose appropriate strategy
4. Add `LOCKPLANE_SQLITE_USE_TABLE_RECREATION=1` env var for override

### Phase 4: User-Facing Documentation

**README.md updates:**

```markdown
## Database Requirements

### Minimum Versions

- **PostgreSQL**: 12.0 or later
- **SQLite**: 3.35.0 or later (for DROP COLUMN support)
  - Older versions (3.25.0+) supported with `LOCKPLANE_SQLITE_USE_TABLE_RECREATION=1`
- **libSQL/Turso**: Latest stable version

### Checking Your Database Version

PostgreSQL:
```bash
psql -c "SELECT version();"
```

SQLite:
```bash
sqlite3 --version
```

### Troubleshooting Version Issues

If you encounter errors like "syntax error near DROP", check your database version:

```bash
# SQLite
lockplane check-version

# Or manually
sqlite3 mydb.db "SELECT sqlite_version();"
```

For older SQLite versions, use table recreation mode:
```bash
export LOCKPLANE_SQLITE_USE_TABLE_RECREATION=1
lockplane apply
```
```

**docs/getting_started.md updates:**

Add version check step after database connection setup.

### Phase 5: Enhanced Error Messages

**Current error:**
```
Error: near "DROP": syntax error
```

**Enhanced error:**
```
Error: Failed to execute migration step "Drop column deprecated_field from table users"

SQL: ALTER TABLE users DROP COLUMN deprecated_field
Database error: near "DROP": syntax error

Possible causes:
  • SQLite version 3.30.1 detected, but DROP COLUMN requires 3.35.0+

Solutions:
  1. Upgrade SQLite to 3.35.0 or later:
     - Ubuntu/Debian: sudo apt install sqlite3
     - macOS: brew upgrade sqlite3

  2. Use table recreation mode (compatible with older SQLite):
     export LOCKPLANE_SQLITE_USE_TABLE_RECREATION=1
     lockplane apply

  3. Check your SQLite version:
     sqlite3 --version
```

### Phase 6: CI/CD Integration

**GitHub Actions matrix testing:**

```yaml
jobs:
  test:
    strategy:
      matrix:
        db:
          - postgres: 12
          - postgres: 13
          - postgres: 14
          - postgres: 15
          - sqlite: 3.35.0
          - sqlite: 3.40.0
          - sqlite: 3.45.0
```

**Version detection in tests:**

```go
func TestRequiresDatabaseVersion(t *testing.T) {
    // Skip test if database version doesn't meet requirements
    db := setupTestDB(t)
    driver := postgres.NewDriver()

    if err := driver.CheckMinimumVersion(context.Background(), db); err != nil {
        t.Skipf("Test requires newer database version: %v", err)
    }

    // Test code...
}
```

## Migration Path for Existing Users

**For users on old SQLite:**
1. Run `lockplane check-version` (new command)
2. See clear error with upgrade instructions
3. Choose: upgrade SQLite OR use `LOCKPLANE_SQLITE_USE_TABLE_RECREATION=1`

**For users on old PostgreSQL (<12):**
1. Clear error message on first command
2. No graceful fallback (PostgreSQL 12+ is reasonable requirement)
3. Must upgrade database

## Testing Strategy

**Unit tests:**
- Version parsing (SQLite "3.35.5" -> [3, 35, 5])
- Version comparison (3.30 < 3.35)
- Error message formatting

**Integration tests:**
- SQLite 3.35+ with DROP COLUMN
- SQLite <3.35 with table recreation mode
- PostgreSQL 12+ with modern DDL
- Version detection queries

**Manual testing:**
- Docker containers with specific versions
- Test on Ubuntu 20.04 (SQLite 3.31.1, old)
- Test on Ubuntu 22.04 (SQLite 3.37.2, modern)

## Security Considerations

- Version queries are read-only (`SELECT version()`)
- No SQL injection risk (no user input)
- Version strings parsed safely (regex, bounds checking)

## Performance Impact

- **Minimal**: Version check is one SELECT query
- Run once per connection, cache result
- No impact on migration execution

## Rollout Plan

1. **Phase 1** (this doc): Document requirements
2. **Phase 2** (week 1): Add version detection to drivers
3. **Phase 3** (week 2): Implement graceful degradation for SQLite
4. **Phase 4** (week 2): Update all documentation
5. **Phase 5** (week 3): Enhance error messages
6. **Phase 6** (week 3): Add CI matrix testing

## Open Questions

1. **Should we support SQLite <3.35?**
   - Pro: Wider compatibility (Ubuntu 20.04 has 3.31.1)
   - Con: Table recreation is complex and error-prone
   - Decision: YES, via `LOCKPLANE_SQLITE_USE_TABLE_RECREATION=1`

2. **Should we support PostgreSQL <12?**
   - Pro: PostgreSQL 11 is still supported until November 2023
   - Con: Significant feature differences, extra complexity
   - Decision: NO, PostgreSQL 12+ is reasonable (released October 2019)

3. **Should version check be opt-out?**
   - Pro: Safety by default
   - Con: Might break existing setups
   - Decision: Always check, provide clear env var override

4. **Should we cache version between commands?**
   - Pro: Avoid repeated queries
   - Con: Users might upgrade database mid-session
   - Decision: Cache per-connection, re-check on new connection

## Success Metrics

- Zero "syntax error near DROP" issues reported without version hint
- Clear error messages with actionable next steps
- Documentation clearly states minimum versions
- CI tests run against minimum supported versions

## References

- SQLite DROP COLUMN: https://www.sqlite.org/lang_altertable.html#altertabdropcol
- SQLite version history: https://www.sqlite.org/changes.html
- PostgreSQL feature matrix: https://www.postgresql.org/docs/current/features.html
- Current implementation: `database/sqlite/generator.go:68-72`
