# Interactive Init Wizard Enhancement

## Progress Checklist

### Phase 1: Design and Planning
- [x] Document current init behavior
- [x] Define wizard flow and UX
- [x] Specify config file structure
- [x] Add visual design (colors, formatting)
- [x] Design for all project stages (empty, existing config, existing DB)
- [x] Add educational content (shadow DB, best practices)
- [x] Design extensibility (security hardening, future features)
- [x] Review and approve design

### Phase 2: Core Implementation
- [x] Implement multi-step wizard model
- [x] Add environment collection flow
- [x] Add database type detection/selection
- [x] Add connection string validation
- [x] Generate lockplane.toml with custom environments
- [x] Generate corresponding .env files

### Phase 3: Enhanced Features
- [ ] Add "detect from existing database" flow
- [ ] Add connection string testing
- [ ] Add schema path customization
- [ ] Add templates for common setups (Supabase, local Postgres, SQLite)

### Phase 4: Testing
- [ ] Unit tests for wizard state machine
- [ ] Integration tests for config generation
- [ ] Manual testing of interactive flow
- [ ] Test edge cases (existing configs, invalid inputs)

### Phase 5: Documentation
- [ ] Update README.md with new init flow
- [ ] Update docs/getting_started.md
- [ ] Add examples for common scenarios
- [ ] Update CLI help text

## Context

Currently, `lockplane init` creates a basic setup with hardcoded PostgreSQL defaults:
- Creates `schema/` directory
- Generates `schema/lockplane.toml` with a single "local" environment
- Uses hardcoded connection strings for PostgreSQL

**Limitations:**
- No interactive prompts for custom environments
- No support for SQLite or other databases during init
- No guidance for users unfamiliar with connection strings
- No validation of connection strings
- Users must manually edit TOML after creation

**User Pain Points:**
- "How do I set up for SQLite?"
- "What connection string should I use for Supabase?"
- "Can I define multiple environments during init?"
- "How do I connect to my existing database?"

## Key Opinionated Decisions

**This design makes strong choices to reduce decision fatigue:**

1. **Config location**: Always `schema/lockplane.toml` (unless already exists elsewhere)
2. **Schema directory**: Always `schema/` (no customization)
3. **PostgreSQL shadow DB**: Always enabled on port 5433
4. **SQLite shadow DB**: Always disabled (avoids file clutter)
5. **Connection testing**: Always required (no skip option)
6. **Gitignore**: Always auto-updated (no asking)
7. **SSL mode**: Auto-detected (localhost=disable, remote=require)
8. **Default DB**: PostgreSQL (most common for production)
9. **File permissions**: `.env.*` always 0600
10. **Credentials**: Never in lockplane.toml, always in `.env.*`

**Philosophy**: Defaults over choices. The wizard guides users to the best practice path with minimal decision points.

## Goals

1. **Interactive Environment Setup**: Guide users through defining environments with sensible defaults
2. **Database Type Selection**: Support PostgreSQL, SQLite, and libSQL/Turso out of the box
3. **Opinionated Defaults**: Make the right choice by default, allow overrides when needed
4. **Config File Placement**: Always create `schema/lockplane.toml` unless it already exists elsewhere
5. **Secure Credential Handling**: Always generate `.env.{name}` files for sensitive data
6. **Schema Directory Structure**: Always use `schema/` directory - consistent, predictable
7. **Shadow DB by Default**: PostgreSQL gets shadow DB, SQLite doesn't (avoid file clutter)
8. **Auto-configure .gitignore**: Automatically add `.env.*` without asking

## Design

### Wizard Flow (Opinionated)

```
┌─────────────────────────────────────────────────────────────┐
│ Lockplane Init Wizard                                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Step 1: Check for existing config                          │
│   ├─ If schema/lockplane.toml exists:                      │
│   │   └─ "Config exists. Add new environment? (y/N)"       │
│   │       ├─ Yes: Continue to Step 2 (skip welcome)        │
│   │       └─ No: Exit                                       │
│   ├─ If lockplane.toml exists in other location:           │
│   │   └─ "Found config at {path}. Use that location."      │
│   │       Continue with existing config location           │
│   └─ If not found: Create schema/lockplane.toml            │
│                                                             │
│ Step 2: Database Type Selection                            │
│   What database are you using?                              │
│     1. PostgreSQL (recommended for production)              │
│     2. SQLite (simple, file-based)                          │
│     3. libSQL/Turso (edge database)                         │
│                                                             │
│   DEFAULT: PostgreSQL (press Enter)                         │
│                                                             │
│ Step 3: Connection Details (varies by type)                │
│                                                             │
│   For PostgreSQL:                                           │
│     Environment name: [local] ← auto-suggested             │
│     Host: [localhost]                                       │
│     Port: [5432]                                            │
│     Database: [lockplane]                                   │
│     User: [lockplane]                                       │
│     Password: [lockplane] (masked)                          │
│     SSL Mode: [disable] (localhost) / [require] (remote)    │
│                                                             │
│     Shadow DB (for safe migrations):                        │
│       Auto-configured: localhost:5433/lockplane_shadow      │
│       ✓ Shadow DB always enabled for PostgreSQL            │
│                                                             │
│   For SQLite:                                               │
│     Environment name: [local]                               │
│     Database file: [schema/lockplane.db]                    │
│                                                             │
│     Shadow DB: Disabled (creates file clutter)              │
│     ⚠ SQLite migrations run without shadow validation       │
│                                                             │
│   For libSQL/Turso:                                         │
│     Environment name: [production] ← suggested              │
│     Database URL: [libsql://[name]-[org].turso.io]          │
│     Auth token: [****] (masked)                             │
│                                                             │
│     Shadow DB: Not supported by Turso                       │
│                                                             │
│ Step 4: Test Connection                                     │
│   Testing connection... ✓ Connection successful            │
│   ✗ Connection failed: [error message]                      │
│      Retry (r) / Skip (s) / Edit (e)                        │
│                                                             │
│ Step 5: Add Another Environment?                            │
│   Common next steps:                                        │
│     • Add 'staging' environment? (y/N)                      │
│     • Add 'production' environment? (y/N)                   │
│     • Add custom environment? (y/N)                         │
│   ├─ Yes: Loop back to Step 2                              │
│   └─ No: Continue to Step 6                                │
│                                                             │
│ Step 6: Create Files                                        │
│   Creating project structure...                             │
│     ✓ Created schema/ directory                             │
│     ✓ Created schema/lockplane.toml                         │
│     ✓ Created .env.local                                    │
│     ✓ Updated .gitignore                                    │
│                                                             │
│   Configuration:                                            │
│     • Environments: local                                   │
│     • Default: local                                        │
│     • Schema path: schema/                                  │
│                                                             │
│   Next steps:                                               │
│     1. Run: lockplane introspect                            │
│     2. Review: schema/lockplane.toml                        │
│     3. Ensure .env.* files are not committed                │
│                                                             │
│   Run introspect now? (Y/n)                                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Key Opinionated Decisions:**

1. **Always `schema/lockplane.toml`** unless config already exists elsewhere
2. **Always `schema/` directory** for schema files (no customization)
3. **PostgreSQL by default** - most common production database
4. **Shadow DB always on** for PostgreSQL (safety first)
5. **Shadow DB always off** for SQLite (avoids file clutter)
6. **Always test connection** before proceeding (catch errors early)
7. **Auto-update .gitignore** without asking (security best practice)
8. **Suggest common next environments** (staging, production)
9. **Offer to run introspect** immediately after setup

### File Structure (Opinionated)

**Directory layout after `lockplane init`:**

```
project/
├── schema/
│   ├── lockplane.toml          ← Config file (ALWAYS here unless already exists)
│   └── (schema files go here)
├── .env.local                  ← Credentials (NEVER commit)
├── .env.staging                ← Optional additional environments
└── .gitignore                  ← Auto-updated to exclude .env.*
```

#### Generated `schema/lockplane.toml`

```toml
# Lockplane Configuration
# Generated by: lockplane init
#
# Config location: Always in schema/ directory for consistency
# Credentials: Stored in .env.* files (never in this file)

default_environment = "local"

[environments.local]
description = "Local PostgreSQL development database"
# Connection: .env.local
# Shadow DB: Auto-configured at localhost:5433

[environments.staging]
description = "Staging database"
# Connection: .env.staging
# Add this environment with: lockplane init
```

#### Generated `.env.local` (PostgreSQL example)

```bash
# Lockplane Environment: local
# Generated by: lockplane init
#
# ⚠️  DO NOT COMMIT THIS FILE
# (Already added to .gitignore automatically)

# PostgreSQL connection (auto-detected sslmode=disable for localhost)
DATABASE_URL=postgresql://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable

# Shadow database (always configured for PostgreSQL - safe migrations)
SHADOW_DATABASE_URL=postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable
```

#### Generated `.env.local` (SQLite example)

```bash
# Lockplane Environment: local
# Generated by: lockplane init
#
# ⚠️  DO NOT COMMIT THIS FILE
# (Already added to .gitignore automatically)

# SQLite connection (file-based)
DATABASE_URL=sqlite://schema/lockplane.db

# Shadow database disabled for SQLite (avoids file clutter)
# Migrations will run without shadow validation
```

#### Auto-generated `.gitignore` additions

**Always append to .gitignore (create if doesn't exist):**

```gitignore
# Lockplane environment files (added by lockplane init)
# DO NOT remove - contains database credentials
.env.*
!.env.*.example
```

**Rationale:**
- Security first - credentials should never be committed
- Automatic - user doesn't have to remember
- Safe pattern - excludes all `.env.*` but allows `.env.*.example` files

### Config File Location (Opinionated)

**Approach: Always `schema/lockplane.toml`**

```
schema/
├── lockplane.toml    ← Config always here
├── schema.json       ← Schema files
└── migrations/       ← Migration files (future)
```

**Rationale:**
1. **Co-location**: Config lives with the schema files it configures
2. **Clean project root**: No config clutter in top-level directory
3. **Consistency**: Every Lockplane project looks the same
4. **Obvious**: `schema/` clearly indicates database-related files
5. **Portable**: Can copy `schema/` directory to another project

**Backward compatibility:**
- If `lockplane.toml` exists at project root (old behavior), continue using it
- If `lockplane.toml` exists in both locations, prefer existing location and warn
- New `lockplane init` always creates `schema/lockplane.toml`
- Never automatically migrate old configs (user choice)

**Detection order:**
1. Check `schema/lockplane.toml` (preferred)
2. Check `./lockplane.toml` (legacy)
3. Check `--config` flag (override)

### Quick Start Templates (Built-in)

**Wizard automatically generates correct config based on database type selection.**

#### Template 1: Local PostgreSQL (DEFAULT)

**When selected:** User chooses PostgreSQL, wizard auto-fills localhost defaults

```toml
# schema/lockplane.toml
default_environment = "local"

[environments.local]
description = "Local PostgreSQL development database"
```

```bash
# .env.local (auto-generated)
DATABASE_URL=postgresql://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable
SHADOW_DATABASE_URL=postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable
```

**Assumptions:**
- PostgreSQL running on localhost:5432
- Shadow DB on localhost:5433
- User/password: lockplane/lockplane
- SSL disabled (localhost)

#### Template 2: SQLite (Simple)

**When selected:** User chooses SQLite, wizard suggests file in schema/

```toml
# schema/lockplane.toml
default_environment = "local"

[environments.local]
description = "Local SQLite database"
```

```bash
# .env.local (auto-generated)
DATABASE_URL=sqlite://schema/lockplane.db

# No SHADOW_DATABASE_URL - disabled to avoid file clutter
# Migrations run without shadow validation
```

**Assumptions:**
- Database file in `schema/lockplane.db`
- No shadow DB (avoids `lockplane_shadow.db` file)
- Simple setup for prototyping

#### Template 3: Supabase (Production)

**When selected:** User chooses PostgreSQL + enters Supabase host

```toml
# schema/lockplane.toml
default_environment = "production"

[environments.production]
description = "Supabase production database"
```

```bash
# .env.production (user provides values)
DATABASE_URL=postgresql://postgres.[PROJECT-REF]:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:6543/postgres?sslmode=require
SHADOW_DATABASE_URL=postgresql://postgres.[PROJECT-REF]:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:6543/postgres_shadow?sslmode=require
```

**Wizard helps with:**
- Auto-detects SSL required (remote host)
- Suggests port 6543 (Supabase pooler)
- Auto-generates shadow DB name
- Masks password input

#### Template 4: Turso/libSQL (Edge)

**When selected:** User chooses libSQL/Turso

```toml
# schema/lockplane.toml
default_environment = "production"

[environments.production]
description = "Turso edge database"
```

```bash
# .env.production (user provides values)
DATABASE_URL=libsql://[db-name]-[org].turso.io?authToken=[TOKEN]

# No SHADOW_DATABASE_URL - Turso doesn't support shadow databases
# Migrations run directly (use with caution in production)
```

**Wizard helps with:**
- Validates libsql:// prefix
- Masks auth token input
- Warns about no shadow DB support
- Suggests testing in staging first

### Wizard State Machine

```go
type WizardState int

const (
    StateWelcome WizardState = iota
    StateCheckExisting
    StateQuickStartOrCustom
    StateEnvironmentName
    StateEnvironmentDescription
    StateDatabaseType
    StateConnectionDetails
    StateSchemaPath
    StateTestConnection
    StateAddAnother
    StateSummary
    StateCreating
    StateDone
)

type WizardModel struct {
    state           WizardState

    // Existing config detection
    existingConfig  *Config
    addingToExisting bool

    // Quick start selection
    quickStartChoice string // "postgres", "sqlite", "supabase", "custom"

    // Environment being configured
    currentEnv      EnvironmentInput
    environments    []EnvironmentInput

    // Connection testing
    testingConnection bool
    connectionTestResult string

    // Input fields (using bubbletea textinput)
    inputs          []textinput.Model
    focusIndex      int

    // Validation
    errors          map[string]string

    // Final output
    result          *InitResult
    err             error
}

type EnvironmentInput struct {
    Name              string
    Description       string
    DatabaseType      string // "postgres", "sqlite", "libsql"

    // PostgreSQL fields
    Host              string
    Port              string
    Database          string
    User              string
    Password          string
    SSLMode           string
    ShadowDBPort      string

    // SQLite fields
    FilePath          string
    ShadowFilePath    string

    // libSQL fields
    URL               string
    AuthToken         string

    // Common
    SchemaPath        string
}

type InitResult struct {
    ConfigPath        string
    ConfigCreated     bool
    ConfigUpdated     bool
    EnvFiles          []string
    SchemaDir         string
    SchemaDirCreated  bool
    GitignoreUpdated  bool
}
```

### Connection String Validation

```go
// ValidateConnectionString checks if a connection string is well-formed
func ValidateConnectionString(connStr string, dbType string) error {
    switch dbType {
    case "postgres":
        // Check for postgresql:// or postgres://
        // Validate basic structure
        if !strings.HasPrefix(connStr, "postgres://") &&
           !strings.HasPrefix(connStr, "postgresql://") {
            return fmt.Errorf("PostgreSQL connection string must start with postgres:// or postgresql://")
        }
        // Optional: Parse and validate host, port, database

    case "sqlite":
        // Check for sqlite:// or file path
        if !strings.HasPrefix(connStr, "sqlite://") &&
           !strings.HasPrefix(connStr, "./") &&
           !strings.HasPrefix(connStr, "/") {
            return fmt.Errorf("SQLite connection string must be sqlite:// or a file path")
        }

    case "libsql":
        // Check for libsql://
        if !strings.HasPrefix(connStr, "libsql://") {
            return fmt.Errorf("libSQL connection string must start with libsql://")
        }
    }
    return nil
}

// TestConnection attempts to connect to the database
func TestConnection(connStr string, dbType string) error {
    driver, err := newDriver(dbType)
    if err != nil {
        return err
    }

    db, err := sql.Open(getSQLDriverName(dbType), connStr)
    if err != nil {
        return fmt.Errorf("failed to open connection: %w", err)
    }
    defer db.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := db.PingContext(ctx); err != nil {
        return fmt.Errorf("failed to ping database: %w", err)
    }

    return nil
}
```

### UX Principles (Opinionated)

1. **Defaults over Choices**: Pre-fill smart defaults, Enter key accepts them
   - PostgreSQL: localhost:5432/lockplane
   - SQLite: schema/lockplane.db
   - Environment name: "local" or "production" based on context

2. **Progressive Disclosure**: Only show fields relevant to selected database type
   - PostgreSQL → host, port, user, pass, ssl
   - SQLite → file path only
   - libSQL → URL and auth token only

3. **Auto-configuration**: Don't ask what we can infer
   - localhost → sslmode=disable
   - remote host → sslmode=require
   - PostgreSQL → shadow DB always enabled
   - SQLite → shadow DB always disabled

4. **Immediate Validation**: Test connection before proceeding
   - Catch errors early (wrong port, bad credentials)
   - Offer to retry or edit
   - Don't allow "skip" - connection must work

5. **Security by Default**:
   - Auto-update .gitignore (don't ask)
   - Mask password/token input
   - Set .env.* permissions to 0600
   - Never write credentials to lockplane.toml

6. **Clear Guidance**: Tell users what to do next
   - "Run: lockplane introspect"
   - "Review: schema/lockplane.toml"
   - Offer to run introspect immediately (Y/n)

### Error Handling (Strict)

1. **Existing Config**:
   - If `schema/lockplane.toml` exists → "Add new environment? (y/N)"
   - If user says No → Exit cleanly
   - If user says Yes → Go directly to database type selection
   - Never overwrite existing config without explicit confirmation

2. **Invalid Input**:
   - Validate immediately (e.g., port must be 1-65535)
   - Show error inline in red
   - Don't allow "Next" until fixed
   - Provide examples: "Example: 5432"

3. **Connection Failure**:
   - Always test connection before proceeding
   - On failure, show clear error: "Connection failed: could not connect to server"
   - Options: Retry (r) / Edit values (e) / Quit (q)
   - **No skip option** - connection must work to proceed
   - After 3 failed attempts, suggest checking if database is running

4. **File Write Errors**:
   - Check write permissions before starting wizard
   - If can't write to `schema/`, fail fast with clear message
   - Suggest: "mkdir schema" or "chmod u+w schema"
   - Never leave partial files (use temp files + atomic rename)

5. **Atomic Commits**:
   - Write all files to temp locations first
   - Only move to final locations if all succeed
   - On Ctrl+C or error, clean up temp files
   - All or nothing - no partial state

6. **Validation Rules** (enforced):
   - Environment name: alphanumeric + underscores only
   - Port: 1-65535
   - File paths: must be relative, no "../" traversal
   - Connection strings: must match expected format for type

### Security (Non-negotiable)

1. **Masked Input**: Always mask passwords and auth tokens during input
   - Use `***` display
   - Don't echo to terminal history

2. **Strict File Permissions**:
   - Create `.env.*` files with 0600 (owner read/write only)
   - Verify permissions after write
   - Warn on Windows (different permission model)

3. **Auto-update .gitignore**:
   - Always add `.env.*` pattern (don't ask user)
   - Create .gitignore if doesn't exist
   - Never skip this step

4. **Never Store Credentials in TOML**:
   - `lockplane.toml` contains NO secrets
   - All credentials → `.env.*` files
   - TOML only has structure + references

5. **Validation**:
   - Reject connection strings with hardcoded passwords in TOML
   - Force use of environment variables
   - Check for common mistakes (password in URL)

6. **Warnings**:
   - If user enters remote host with sslmode=disable → warn
   - If .env.* files already exist → warn before overwrite
   - If .gitignore doesn't exclude .env.* after adding → error

## Implementation Plan

### Phase 1: Core Wizard Structure
- Implement multi-step wizard state machine
- Add navigation (next, back, quit)
- Add input validation framework

### Phase 2: Environment Collection
- Implement environment name/description inputs
- Add database type selection menu
- Build connection details forms for each DB type

### Phase 3: File Generation
- Implement TOML generation with multiple environments
- Implement `.env.*` file generation
- Add `.gitignore` update logic

### Phase 4: Enhanced Features
- Add connection testing
- Add quick start templates
- Add existing config detection and merging

### Phase 5: Polish
- Add help text and examples
- Improve error messages
- Add progress indicators

## Testing Strategy

### Unit Tests
- Wizard state transitions
- Connection string validation
- TOML generation with various inputs
- `.env` file generation

### Integration Tests
- Full wizard flow (automated with pre-filled inputs)
- Config file creation and loading
- Multiple environment scenarios

### Manual Testing
- Complete wizard with each database type
- Test with existing configs
- Test error cases (bad permissions, invalid input)
- Test on different platforms (Linux, macOS, Windows)

## Migration Strategy

### Backward Compatibility

**Current behavior**: Creates `schema/lockplane.toml` (basic, non-interactive)
**New behavior**: Creates `schema/lockplane.toml` (interactive wizard)

**No breaking changes**:
- Config still in `schema/lockplane.toml` location
- If exists at root (`./lockplane.toml`), continue using that
- `--yes` flag still works (skips wizard, uses defaults)
- All existing configs work without modification

### Detection Logic

```
1. Check for --config flag
   └─ If provided: use that path

2. Check for schema/lockplane.toml
   └─ If exists: use it (preferred location)

3. Check for ./lockplane.toml
   └─ If exists: use it (legacy location)
   └─ Show one-time message: "Using legacy config location"

4. If neither exists:
   └─ Run init wizard (creates schema/lockplane.toml)
```

### No Migration Needed

- Users with `./lockplane.toml` can continue using it
- New `lockplane init` creates `schema/lockplane.toml`
- Users can manually move if desired (optional)
- Both locations supported indefinitely

## Design Decisions (Opinionated)

### 1. Interactive Wizard (Not Flags)

**Decision**: Use interactive wizard with smart defaults

**Rejected alternative**: `lockplane init --env local --type postgres --host localhost --port 5432 ...`

**Why**:
- Too many flags (poor UX)
- Can't validate connection during setup
- Hard for beginners to remember
- No guidance or error correction

**Compromise**: Support `--yes` flag for CI/scripts (uses all defaults)

### 2. Config in `schema/` (Not Root)

**Decision**: Always create `schema/lockplane.toml`

**Rejected alternative**: `./lockplane.toml` at project root

**Why**:
- Co-location with schema files
- Clean project root
- Consistency across projects
- Easy to move schema/ to another project

**Backward compatibility**: Continue supporting root location if already exists

### 3. Separate .env Files (Not Embedded)

**Decision**: Credentials always in `.env.*` files

**Rejected alternative**: Everything in lockplane.toml

**Why**:
- Security - credentials never in version control
- Standard practice (.env pattern widely used)
- Easy to share config without secrets
- Different credentials per developer

### 4. Shadow DB Defaults by Database Type

**Decision**:
- PostgreSQL: Shadow DB always enabled (port 5433)
- SQLite: Shadow DB always disabled
- libSQL/Turso: Not supported

**Rejected alternative**: Ask user every time

**Why**:
- PostgreSQL: Shadow DB is critical for safe migrations
- SQLite: Creates file clutter, less critical for dev DBs
- Clear defaults reduce decision fatigue

### 5. Always Test Connection

**Decision**: Test connection before proceeding, no skip option

**Rejected alternative**: Connection testing optional

**Why**:
- Catch errors early (wrong port, bad password)
- Better UX than failing on first introspect
- Forces user to fix config before continuing
- Validates that shadow DB is reachable

## Implementation Scope

### Must Have (Phase 2-3)

1. ✅ PostgreSQL setup with shadow DB
2. ✅ SQLite setup without shadow DB
3. ✅ Connection testing
4. ✅ Auto-update .gitignore
5. ✅ Multi-environment support
6. ✅ File permissions (0600 for .env.*)

### Should Have (Phase 4)

1. ✅ libSQL/Turso support
2. ✅ Smart SSL mode detection (localhost vs remote)
3. ✅ Offer to run introspect after setup
4. ⏳ Supabase-specific guidance (port 6543, pooler URL format)

### Nice to Have (Future)

1. ❌ Import from Prisma/Alembic/Django (out of scope)
2. ❌ Generate docker-compose.yml (out of scope)
3. ❌ Auto-detect running databases (out of scope)
4. ❌ Cloud provider OAuth (out of scope)

### Explicitly Not Doing

1. ❌ Config file format migration (user's responsibility)
2. ❌ Database creation (user must create DB first)
3. ❌ PostgreSQL installation (user must install first)
4. ❌ Schema migration from other tools (separate tool)

## Success Metrics

- Reduced time to first successful `lockplane introspect` after `init`
- Fewer GitHub issues about "how do I configure for X database?"
- Increased usage of multiple environments
- Better credential security (more users with `.env.*` in `.gitignore`)

## Future Enhancements

1. **Environment cloning**: Copy and modify existing environment
2. **Import from connection string**: Parse and populate fields
3. **Database detection**: Auto-detect running databases (localhost scanning)
4. **Cloud provider integration**: OAuth flows for Supabase, PlanetScale, etc.
5. **Configuration validation**: `lockplane config check` command
6. **Environment switching**: `lockplane use <env>` to change default

---

## New Design Requirements (2025-11-08)

### 1. Visual Design - Nicer Colors & Formatting

**Use terminal colors for better visual hierarchy and user experience:**

```
┌─────────────────────────────────────────────────────────────┐
│ 🔧 Lockplane Init Wizard                                    │ ← Cyan header
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ 📦 Database Type Selection                                  │ ← Bold section header
│                                                             │
│   What database are you using?                              │ ← Gray question text
│     ► 1. PostgreSQL (recommended for production)            │ ← Green highlight for selection
│       2. SQLite (simple, file-based)                        │
│       3. libSQL/Turso (edge database)                       │
│                                                             │
│   💡 PostgreSQL provides the most features including        │ ← Blue info box
│      shadow databases for safe migration testing.           │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ ✓ Connection successful                                     │ ← Green success
│ ⚠ Warning: Remote host without SSL                         │ ← Yellow warning
│ ✗ Connection failed: could not connect to server           │ ← Red error
│                                                             │
│ 🔍 What is a shadow database?                               │ ← Collapsible help
│    A shadow database is a temporary copy used to test       │ ← Dim gray help text
│    migrations before applying them to your actual database. │
│    This prevents data loss from failed migrations.          │
│                                                             │
└─────────────────────────────────────────────────────────────┘

Arrow keys: navigate  │  Enter: select  │  ?: help  │  Ctrl+C: cancel
← Dim status bar with keyboard shortcuts
```

**Color Palette:**
- **Primary (Cyan)**: Headers, branding
- **Success (Green)**: Checkmarks, successful connections, valid input
- **Warning (Yellow)**: Warnings, non-critical issues
- **Error (Red)**: Errors, failed connections, invalid input
- **Info (Blue)**: Help text, educational content, tips
- **Muted (Gray)**: Labels, secondary text, borders
- **Highlight (Bright White/Bold)**: Selected items, current input focus

**Icons & Symbols:**
- ✓ Success
- ✗ Error
- ⚠ Warning
- 💡 Tip/Info
- 🔧 Tool/Action
- 📦 Package/Database
- 🔍 Help/Learn More
- 🔒 Security
- ⏳ In Progress (spinner)

**Implementation:**
- Use `github.com/charmbracelet/lipgloss` for styling
- Use `github.com/charmbracelet/bubbles` for components (spinner, input, etc.)
- Consistent spacing and alignment
- Responsive to terminal width (graceful degradation)

---

### 2. Handle All Project Stages

**The wizard must gracefully handle three scenarios:**

#### Scenario A: Empty Project (First Time)
```
Current directory: /home/user/myapp
├── (empty or just code files)

Wizard behavior:
1. Welcome message: "Let's set up Lockplane!"
2. Create schema/ directory
3. Create schema/lockplane.toml
4. Generate .env.* files
5. Update .gitignore
6. Offer to run introspect

Result:
/home/user/myapp
├── schema/
│   └── lockplane.toml
├── .env.local
└── .gitignore (updated)
```

#### Scenario B: Existing Config (Add Environment)
```
Current directory: /home/user/myapp
├── schema/
│   └── lockplane.toml  ← EXISTS
└── .env.local          ← EXISTS

Wizard behavior:
1. Detect existing config
2. Show message: "Found existing config at schema/lockplane.toml"
3. List current environments: "Environments: local"
4. Ask: "Add a new environment? (Y/n)"
   - Yes: Skip welcome, go to environment setup
   - No: Exit gracefully with "No changes made"
5. Add new environment to existing config
6. Create new .env.{name} file
7. Show summary: "Added 'staging' environment"

Result (if added 'staging'):
/home/user/myapp
├── schema/
│   └── lockplane.toml  (updated with staging env)
├── .env.local
└── .env.staging        (new)
```

#### Scenario C: Database Already Running (Import/Connect)
```
Current state: User has PostgreSQL running on localhost:5432

Wizard behavior:
1. During database type selection, offer:
   "Do you have a database already running? (Y/n)"

2. If Yes:
   a. "Let me help you connect to it"
   b. Prompt for connection details
   c. TEST CONNECTION immediately
   d. On success: "✓ Connected to PostgreSQL 15.3"
   e. Ask: "Introspect existing schema now? (Y/n)"

3. If user chooses to introspect:
   a. Run introspection
   b. Show: "Found 5 tables: users, posts, comments, tags, categories"
   c. Ask: "Save schema to schema/schema.json? (Y/n)"
   d. Generate schema file
   e. Show next steps: "Schema saved. Ready to manage migrations!"

4. If user skips introspection:
   a. Create config for empty schema
   b. Show: "Config created. Run 'lockplane introspect' when ready"

Enhanced flow:
┌─────────────────────────────────────────────────────────────┐
│ 📦 Database Setup                                           │
│                                                             │
│   Do you have a PostgreSQL database already running?        │
│     ● Yes - Connect to existing database                    │
│     ○ No - Set up for future database                       │
│                                                             │
│   💡 If you have a database running, I can connect to it    │
│      and introspect your existing schema right now!         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Detection & Recovery:**
- Check for lockplane.toml in schema/ and ./
- Check for .env.* files
- Offer to import/merge if partial setup found
- Never overwrite without confirmation
- Always show "what will change" before proceeding

---

### 3. Educational Content & Help

**Integrate learning into the wizard - teach while configuring:**

#### Inline Tips (Context-Aware)
```
┌─────────────────────────────────────────────────────────────┐
│ 🔒 Shadow Database Setup                                    │
│                                                             │
│   Shadow DB: localhost:5433/lockplane_shadow                │
│   ✓ Auto-configured (PostgreSQL only)                       │
│                                                             │
│   💡 What is a shadow database?                             │
│      A shadow database is a temporary, isolated copy of     │
│      your database used to test migrations safely.          │
│                                                             │
│      How it works:                                          │
│      1. Apply migration to shadow DB first                  │
│      2. Verify it succeeds without errors                   │
│      3. Only then apply to your real database               │
│      4. Prevents data loss from failed migrations           │
│                                                             │
│      ⚠ SQLite doesn't use shadow DBs (creates file clutter) │
│      ℹ Turso doesn't support shadow DBs (edge databases)    │
│                                                             │
│   Press 'h' for more help, Enter to continue                │
└─────────────────────────────────────────────────────────────┘
```

#### Educational Moments (Strategic)

**Moment 1: First Environment Setup**
```
💡 TIP: Environments
   Lockplane supports multiple environments (local, staging, prod).

   • local: Your development database on localhost
   • staging: Pre-production testing database
   • production: Live database (use with caution!)

   You can add more environments later with: lockplane init
```

**Moment 2: Connection String Security**
```
🔒 SECURITY: Why .env files?

   Database credentials should NEVER be committed to git.

   ✓ lockplane.toml - Safe to commit (no secrets)
   ✗ .env.* - Never commit (contains passwords)

   We'll automatically add .env.* to .gitignore for you.
```

**Moment 3: SSL Mode**
```
🔐 SSL Connections

   Detected remote host: db.example.com
   → Using sslmode=require (encrypted connection)

   ℹ localhost connections use sslmode=disable (faster, still secure)
   ⚠ Never use sslmode=disable for remote databases!
```

**Moment 4: Migration Safety**
```
✓ Connection successful!

💡 Best Practice: Test migrations safely

   1. Shadow DB validates migrations before applying
   2. Rollback plans generated automatically
   3. Always test in staging before production

   Your setup: Shadow DB enabled ✓
   Learn more: https://lockplane.dev/docs/shadow-db
```

#### Expandable Help (Press '?' anytime)
```
┌─────────────────────────────────────────────────────────────┐
│ 📚 Help: Database Connection                                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ PostgreSQL Connection String Format:                        │
│   postgresql://USER:PASSWORD@HOST:PORT/DATABASE?sslmode=X  │
│                                                             │
│ Examples:                                                   │
│   Local:     postgresql://user:pass@localhost:5432/mydb    │
│   Supabase:  postgresql://postgres:***@aws-0-us-west.      │
│              pooler.supabase.com:6543/postgres             │
│                                                             │
│ Common Issues:                                              │
│   ✗ "connection refused" → Database not running            │
│   ✗ "authentication failed" → Wrong password               │
│   ✗ "database does not exist" → Create DB first            │
│                                                             │
│ Press 'b' to go back, 'q' to quit                          │
└─────────────────────────────────────────────────────────────┘
```

**Help System Implementation:**
- Press '?' at any step shows contextual help
- Help is step-specific (relevant to current question)
- Links to docs for deeper learning
- Examples for every input type
- Common pitfalls and solutions

---

### 4. Extensibility Design - Future Features

**Design the wizard to be easily extensible for future capabilities:**

#### Plugin Architecture
```go
// Extensible validator system
type Validator interface {
    Name() string
    Description() string
    Validate(ctx context.Context, env *Environment) ValidationResult
    Required() bool  // true = must pass, false = warning only
}

// Example validators (now and future)
var validators = []Validator{
    &ConnectionValidator{},           // Phase 2 (now)
    &ShadowDBValidator{},              // Phase 2 (now)
    &SecurityHardeningValidator{},     // Future
    &PerformanceValidator{},           // Future
    &BackupConfigValidator{},          // Future
    &SSLCertificateValidator{},        // Future
}

// Future: Security Hardening Validator
type SecurityHardeningValidator struct{}

func (v *SecurityHardeningValidator) Validate(ctx context.Context, env *Environment) ValidationResult {
    checks := []Check{
        checkPasswordPolicy(ctx, env),
        checkRolePermissions(ctx, env),
        checkEncryptionAtRest(ctx, env),
        checkAuditLogging(ctx, env),
        checkConnectionLimits(ctx, env),
    }

    return ValidationResult{
        Validator: "Security Hardening",
        Checks:    checks,
        Level:     WarningLevel,  // Don't block init, just inform
    }
}

// Wizard shows results:
// 🔒 Security Hardening Check
//    ✓ Password policy: Strong (min 12 chars)
//    ⚠ Role permissions: postgres user has superuser
//    ✗ Encryption at rest: Not enabled
//    ⚠ Audit logging: Not configured
//    ✓ Connection limits: 100 (reasonable)
//
//    💡 Improve security: lockplane security harden
```

#### Wizard Step Plugin System
```go
// Extensible step system
type WizardStep interface {
    Name() string
    Enabled(ctx *WizardContext) bool  // Conditional steps
    Render(model *WizardModel) string
    Update(msg tea.Msg, model *WizardModel) (*WizardModel, tea.Cmd)
    Validate(model *WizardModel) error
}

// Core steps (Phase 2-3)
var coreSteps = []WizardStep{
    &WelcomeStep{},
    &DatabaseTypeStep{},
    &ConnectionDetailsStep{},
    &TestConnectionStep{},
    &SummaryStep{},
}

// Optional/future steps (enabled by flags or config)
var optionalSteps = []WizardStep{
    &IntrospectStep{enabled: func(ctx) bool {
        return ctx.ConnectionSuccessful && ctx.UserWantsIntrospect
    }},

    &SecurityCheckStep{enabled: func(ctx) bool {
        return ctx.Flags.CheckSecurity  // --check-security flag
    }},

    &BackupConfigStep{enabled: func(ctx) bool {
        return ctx.DatabaseType == "postgres" && ctx.Flags.SetupBackups
    }},

    &PerformanceBaselineStep{enabled: func(ctx) bool {
        return ctx.Flags.Benchmark  // --benchmark flag
    }},
}

// Easy to add new steps without modifying core wizard
```

#### Template System for Database Types
```go
// Easily add new database types
type DatabaseTemplate interface {
    Name() string
    DisplayName() string
    Icon() string
    ConnectionFields() []Field
    Validators() []Validator
    DefaultShadowDB() bool
    GenerateConnectionString(fields map[string]string) string
    HelpText() string
}

// Current templates
var databaseTemplates = map[string]DatabaseTemplate{
    "postgres": &PostgreSQLTemplate{},
    "sqlite":   &SQLiteTemplate{},
    "libsql":   &LibSQLTemplate{},
}

// Easy to add future databases
// "mysql":      &MySQLTemplate{},      // Future
// "cockroach":  &CockroachTemplate{},  // Future
// "yugabyte":   &YugabyteTemplate{},   // Future
```

#### Feature Flags for Progressive Enhancement
```go
type FeatureFlags struct {
    // Current features (always on)
    ConnectionTesting    bool  // true
    ShadowDBSetup       bool  // true
    GitignoreUpdate     bool  // true

    // Future features (opt-in via flags)
    SecurityHardening   bool  // --check-security
    BackupConfiguration bool  // --setup-backups
    PerformanceTuning   bool  // --tune-performance
    MultiRegionSetup    bool  // --multi-region

    // Experimental features (hidden)
    AutoSchemaImport    bool  // --experimental-import
    CloudProvisioning   bool  // --experimental-cloud
}

// Usage:
// lockplane init                           (basic)
// lockplane init --check-security          (with security checks)
// lockplane init --setup-backups           (with backup config)
// lockplane init --experimental-import     (auto-import from Prisma/etc)
```

#### Post-Init Hook System
```go
// Allow running actions after successful init
type PostInitHook interface {
    Name() string
    ShouldRun(ctx *WizardContext) bool
    Run(ctx context.Context, cfg *Config) error
    RollbackOnError() bool
}

var postInitHooks = []PostInitHook{
    &GitignoreUpdateHook{},           // Always run
    &IntrospectHook{},                // If user opts in
    &SecurityCheckHook{},             // If --check-security
    &CreateInitialMigrationHook{},    // If schema exists
    &SetupCIWorkflowHook{},           // If --setup-ci
    &InstallPreCommitHookHook{},      // If --setup-hooks
}

// Example future hook:
type SecurityCheckHook struct{}

func (h *SecurityCheckHook) ShouldRun(ctx *WizardContext) bool {
    return ctx.Flags.CheckSecurity
}

func (h *SecurityCheckHook) Run(ctx context.Context, cfg *Config) error {
    fmt.Println("\n🔒 Running security hardening checks...")

    results := runSecurityChecks(ctx, cfg)
    displaySecurityReport(results)

    if hasHighRiskIssues(results) {
        fmt.Println("\n⚠ High-risk security issues found")
        fmt.Println("  Run: lockplane security harden")
    }

    return nil
}
```

**Extensibility Benefits:**
1. Add new features without breaking existing wizard
2. Optional features don't clutter basic flow
3. Easy to experiment with new capabilities
4. Backward compatible (new steps are opt-in)
5. Clear separation of concerns

**Future Feature Examples:**
- `lockplane init --check-security` → Security hardening validation
- `lockplane init --setup-backups` → Configure automated backups
- `lockplane init --tune-performance` → Analyze and suggest DB tuning
- `lockplane init --setup-ci` → Generate GitHub Actions workflow
- `lockplane init --multi-region` → Configure multi-region deployment
