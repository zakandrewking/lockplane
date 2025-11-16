# Validate SQL via Plan (Shadow DB Validation)

## Progress Checklist
- [ ] Phase 1: Design and planning
  - [x] Initial design document
  - [ ] Gather feedback and refine approach
  - [ ] Decide on API/flags for IDE integration
- [ ] Phase 2: Implementation
  - [ ] Add `plan --validate` mode that outputs diagnostics
  - [ ] Implement shadow DB caching for performance
  - [ ] Add incremental validation support
  - [ ] Map execution errors back to source files/lines
- [ ] Phase 3: Integration
  - [ ] Update VSCode extension to use new validation
  - [ ] Remove old `validate sql` command
  - [ ] Simplify `validate` to just JSON schema checking
- [ ] Phase 4: Documentation and testing
  - [ ] Add tests for new validation mode
  - [ ] Update docs and examples
  - [ ] Performance benchmarks

## Context

Currently, Lockplane has two separate validation paths:

1. **`validate sql`**: Uses pg_query to parse SQL and check syntax
   - Fast but only catches syntax errors
   - Doesn't catch semantic errors (e.g., index on non-existent table)
   - Custom error enhancement logic
   - Line number tracking issues

2. **`plan` + `apply`**: Actually executes against shadow DB
   - Catches both syntax and semantic errors
   - Already used in production workflow
   - More comprehensive but slower

This creates duplicate validation logic and inconsistencies.

## Problem

**Current issues:**
- Two validation paths mean errors caught differently
- `validate sql` misses semantic errors that `apply` would catch
- Maintaining two separate error enhancement systems
- VSCode extension uses `validate sql` which gives incomplete feedback

**Examples of errors `validate sql` MISSES:**

```sql
-- File 1: schema.sql
CREATE TABLE users (
  id BIGINT PRIMARY KEY
);

-- File 2: indexes.sql
CREATE INDEX idx_posts_user_id ON posts(user_id);
-- ❌ Error: table "posts" doesn't exist
-- ✅ validate sql: passes (syntax is valid)
-- ❌ apply: fails (table doesn't exist)
```

```sql
-- File 1: schema.sql
CREATE TABLE users (
  id TEXT PRIMARY KEY
);

-- File 2: policies.sql
CREATE POLICY user_isolation ON posts
  USING (user_id = current_user_id());
-- ❌ Error: function "current_user_id()" doesn't exist
-- ✅ validate sql: passes (syntax is valid)
-- ❌ apply: fails (function not defined)
```

```sql
-- File 1: schema.sql
CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT UNIQUE
);

-- File 2: views.sql
CREATE VIEW active_users AS
  SELECT id, username, email FROM users
  WHERE deleted_at IS NULL;
-- ❌ Error: column "username" doesn't exist
-- ❌ Error: column "deleted_at" doesn't exist
-- ✅ validate sql: passes
-- ❌ apply: fails
```

## Proposed Solution

**Consolidate on shadow DB validation for all use cases.**

### New Architecture

```
┌─────────────────────────────────────────────────────┐
│  validate                                           │
│  └─ schema [file.json]  →  JSON schema validation  │
│                             (keep current behavior) │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  plan                                               │
│  ├─ --from db --to schema/  →  Normal plan         │
│  └─ --validate schema/       →  NEW: Validation    │
│      └─ Create shadow DB                           │
│      └─ Apply all SQL files in order               │
│      └─ Return diagnostics with file/line context  │
│      └─ Output format: JSON for IDE integration    │
└─────────────────────────────────────────────────────┘
```

### API Design

```bash
# New flag for plan command
lockplane plan --validate schema/ --output json

# For IDE integration (with caching and incremental mode)
lockplane plan --validate schema/ \
  --output json \
  --cache-dir /tmp/lockplane-cache \
  --incremental

# validate command ONLY does JSON schema validation
lockplane validate schema schema.json
```

**Output format (JSON for LSP):**

```json
{
  "diagnostics": [
    {
      "file": "schema/indexes.sql",
      "line": 3,
      "column": 1,
      "severity": "error",
      "message": "relation \"posts\" does not exist",
      "code": "undefined_table",
      "context": "CREATE INDEX idx_posts_user_id ON posts(user_id);"
    }
  ],
  "summary": {
    "total_files": 5,
    "errors": 1,
    "warnings": 0,
    "duration_ms": 245
  }
}
```

## Implementation Phases

### Phase 1: Core Validation Mode

**Goal**: Add `plan --validate` that validates all SQL files against shadow DB

**Tasks**:
1. Add `--validate` flag to `plan` command
2. When `--validate` is set:
   - Create empty shadow DB
   - Apply all SQL files from schema directory in order
   - Collect errors with file/line context
   - Return diagnostics instead of plan
3. Add `--output json` flag for structured output
4. Test with various error scenarios

**Acceptance criteria**:
- Catches syntax errors (like current `validate sql`)
- Catches semantic errors (table doesn't exist, type mismatches, etc.)
- Returns accurate file/line numbers
- Works with multi-file schemas

### Phase 2: Performance Optimization

**Goal**: Make it fast enough for IDE real-time feedback

**Challenge**: Shadow DB setup is expensive (~100-500ms)

**Solutions**:

1. **Shadow DB Caching**:
   ```bash
   lockplane plan --validate schema/ --cache-dir /tmp/lockplane
   ```
   - Cache shadow DB state in /tmp
   - Track file mtimes/hashes
   - Only re-apply changed files
   - Similar to how `tsc --watch` works

2. **Incremental Mode**:
   ```bash
   lockplane plan --validate schema/ --incremental --changed-file schema/new.sql
   ```
   - Load cached shadow DB state
   - Only apply the changed file(s)
   - Much faster for IDE feedback

3. **Quick Mode** (fallback):
   ```bash
   lockplane plan --validate schema/ --quick
   ```
   - Skip shadow DB, just parse with pg_query
   - Fast but less comprehensive
   - Useful as a first pass

**Performance targets**:
- Cold start (no cache): < 500ms
- Incremental (cached): < 100ms
- Quick mode: < 50ms

### Phase 3: VSCode Extension Integration

**Goal**: Update VSCode extension to use new validation

**Changes needed**:
1. Replace `validate sql` calls with `plan --validate`
2. Handle JSON diagnostic output
3. Convert to LSP diagnostic format
4. Use incremental mode for real-time feedback

**User experience**:
```typescript
// VSCode extension pseudocode
async function validateDocument(doc: TextDocument) {
  const result = await exec(`lockplane plan --validate ${schemaDir}
    --output json
    --incremental
    --changed-file ${doc.fileName}
    --cache-dir ${cacheDir}
  `);

  const diagnostics = JSON.parse(result);
  return convertToLSPDiagnostics(diagnostics);
}
```

### Phase 4: Cleanup

**Goal**: Remove old validation code

**Tasks**:
1. Remove `validate sql` command entirely
2. Update `validate` to only do JSON schema validation
3. Remove SQL parsing/enhancement code
4. Update documentation
5. Update all examples and tutorials

## Technical Design

### Shadow DB Lifecycle

```
┌─────────────────────────────────────────────────┐
│ 1. Check cache                                  │
│    └─ Hash all SQL files                       │
│    └─ Check if cached DB matches hash          │
└──────────────┬──────────────────────────────────┘
               │
               ├─ Cache hit ──────────────────┐
               │                              │
               └─ Cache miss                  │
                  │                           │
                  ▼                           ▼
         ┌────────────────────┐    ┌──────────────────┐
         │ 2. Create shadow DB│    │ 2. Load cached DB│
         │    └─ DROP/CREATE  │    │    └─ RESTORE    │
         └──────┬─────────────┘    └────────┬─────────┘
                │                           │
                └───────────┬───────────────┘
                            ▼
                ┌────────────────────────────┐
                │ 3. Apply SQL files         │
                │    └─ In dependency order  │
                │    └─ Capture errors       │
                └──────┬─────────────────────┘
                       │
                       ▼
                ┌────────────────────────────┐
                │ 4. Save to cache           │
                │    └─ pg_dump if success   │
                └──────┬─────────────────────┘
                       │
                       ▼
                ┌────────────────────────────┐
                │ 5. Return diagnostics      │
                └────────────────────────────┘
```

### Error Mapping

When a SQL statement fails, we need to map the error back to:
- Original file path
- Line number within that file
- Column number (if available)

**Current approach (in `apply`)**: Errors from shadow DB include line numbers relative to the concatenated SQL. We track which file each statement came from and adjust line numbers accordingly.

**Improvement**: Since we're applying files one at a time (for incremental validation), we already know which file caused the error. Just need to preserve the line number from the error.

### File Ordering

**Challenge**: Files must be applied in dependency order (tables before indexes, etc.)

**Solutions**:

1. **Convention-based** (current approach):
   ```
   schema/
     001_tables.sql
     002_indexes.sql
     003_constraints.sql
   ```
   Files applied in lexicographic order.

2. **Explicit dependencies** (future enhancement):
   ```sql
   -- Depends: 001_tables.sql
   CREATE INDEX idx_users_email ON users(email);
   ```

3. **Auto-detection** (complex but nice):
   Parse all files, build dependency graph, apply in topological order.

For now, stick with convention-based ordering (same as current `plan` behavior).

## Benefits

### For Users

1. **Comprehensive validation**: Catches both syntax and semantic errors
2. **Faster iteration**: Get feedback before running migrations
3. **Better IDE experience**: Real-time error detection in VSCode
4. **Consistent behavior**: Same validation in CLI and IDE

### For Maintainers

1. **Single validation path**: Easier to maintain
2. **Less code**: Remove duplicate validation logic
3. **Fewer bugs**: One source of truth for what's valid
4. **Better test coverage**: Test validation through actual DB execution

## Open Questions

1. **Shadow DB connection**:
   - Already solved via existing `--shadow-db` flag and `lockplane.toml` config
   - Same approach as `apply` command uses today
   - No additional work needed

2. **Caching strategy**: Where to store cached shadow DBs?
   - `/tmp/lockplane-cache-{project-hash}/`
   - `~/.cache/lockplane/`
   - User-specified via `--cache-dir`?

3. **Multiple dialects**: SQLite vs Postgres
   - Already handled via dialect comment in `.lp.sql` files (existing approach)
   - No additional work needed

4. **Performance**: Can we hit < 100ms for incremental validation?
   - Need benchmarks with real schemas
   - May need connection pooling or persistent shadow DB

## Success Metrics

1. **Performance**:
   - Cold validation: < 500ms for typical schema (5-10 files)
   - Incremental validation: < 100ms
   - Quick mode: < 50ms

2. **Coverage**:
   - Catches 100% of errors that `apply` would catch
   - Better than `validate sql` at finding issues

3. **Adoption**:
   - VSCode extension using new validation by default
   - Positive user feedback on error quality
   - Fewer "why didn't validation catch this?" issues

## Future Enhancements

1. **Watch mode**: Continuous validation as files change
   ```bash
   lockplane plan --validate schema/ --watch
   ```

2. **Parallel validation**: Validate multiple independent files concurrently

3. **Partial validation**: Only validate specific files/tables
   ```bash
   lockplane plan --validate schema/ --only users.sql
   ```

4. **Diff-based validation**: Only validate changes since last commit
   ```bash
   lockplane plan --validate schema/ --since HEAD~1
   ```

5. **Auto-fix suggestions**: Suggest fixes for common errors
   ```
   Error: table "posts" does not exist
   Suggestion: Did you mean to create it in 001_tables.sql?
   ```

## Related Work

- **Flyway**: Validates by running migrations against test DB
- **Liquibase**: Similar shadow DB approach for validation
- **Atlas**: Uses declarative schema and validates against target DB
- **TypeScript tsc --watch**: Incremental compilation with caching

## Questions for Discussion

1. Should we support both modes (`--quick` vs `--full`)?
2. What's the default cache location?
3. How do we handle schema evolution (cached DB is old)?
4. Should we expose this as a separate `validate-plan` command or as a flag to `plan`?
5. Do we need a way to specify file dependencies explicitly?
