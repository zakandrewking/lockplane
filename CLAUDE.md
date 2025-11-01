# Claude Code Instructions for Lockplane

This file contains project-specific instructions for AI assistants working on Lockplane.

## Project Overview

Lockplane is a Postgres-first control plane for safe, AI-friendly schema management. It provides:
- Schema introspection and diff generation
- Migration plan generation with validation
- Shadow DB testing before production deployment
- Automatic rollback generation

## 🚨 COMPLETE WORKFLOW CHECKLIST 🚨

**WHENEVER YOU MAKE CODE CHANGES, FOLLOW THIS CHECKLIST EXACTLY:**

### Phase 1: Planning & Implementation
- [ ] Understand the requirement fully
- [ ] Identify which files need to be modified
- [ ] Make code changes
- [ ] Format code: `go fmt ./...`
- [ ] Vet code: `go vet ./...` (catches common errors)

### Phase 2: Testing (CRITICAL - ALWAYS RUN TESTS)
- [ ] Write or update tests for the changes
- [ ] Run tests: `go test -v ./...`
- [ ] Verify tests pass
- [ ] Build the project: `go build .`
- [ ] Verify build succeeds

### Phase 3: Documentation (CRITICAL - DO NOT SKIP)
- [ ] Update `printHelp()` in `main.go` if CLI changed
- [ ] Update `README.md` with examples and usage
- [ ] Update `.claude/skills/lockplane.md` (Claude skill)
- [ ] Update `llms.txt` (LLM context file)
- [ ] Update `docs/getting_started.md` with user workflows
- [ ] Check if `docs/prisma.md` needs updates
- [ ] Check if `docs/supabase.md` needs updates
- [ ] Check if `docs/alembic.md` needs updates
- [ ] Update any other relevant docs in `docs/` directory
- [ ] Run `./scripts/check-docs-consistency.sh` to verify

### Phase 4: Git (CRITICAL - ALWAYS COMMIT AND PUSH)
- [ ] Check git status: `git status`
- [ ] Review changes: `git diff`
- [ ] Stage all changes: `git add .`
- [ ] Create descriptive commit message following this format:
  ```
  <type>: <short description>

  <longer description if needed>
  - Detail 1
  - Detail 2
  ```
  Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`
- [ ] Commit changes: `git commit -m "message"`
- [ ] Push to remote: `git push`

### Phase 5: Verification
- [ ] Verify commit was successful: `git log -1`
- [ ] Verify push was successful: `git status` should show "up to date"
- [ ] Summarize what was done for the user

**REMEMBER: Code changes without tests, documentation updates, and git commits are incomplete work!**

## Code Organization

- `main.go` - CLI entry point and command handlers
- `planner.go` - Migration plan generation
- `rollback.go` - Rollback plan generation
- `diff.go` - Schema diffing logic
- `json_schema.go` - JSON schema loading and validation
- `init.go` - Docker Compose setup
- `docs/` - Documentation files
- `examples/` - Example schemas and plans
- `testdata/` - Test fixtures
  - `testdata/plans-json/` - JSON migration plan fixtures
  - `testdata/fixtures/` - SQL schema fixtures (being migrated to JSON)

## Testing Requirements

**ALWAYS run tests after making changes to code.** This is critical for maintaining code quality.

### When to Run Tests

Run tests in these scenarios:

1. **After modifying Go code** - Run `go test ./...` to ensure all tests pass
2. **After adding new features** - Write tests first, then implement, then verify
3. **After refactoring** - Tests ensure behavior hasn't changed
4. **After fixing bugs** - Add a regression test, fix the bug, verify the test passes
5. **Before committing** - Final check that everything works

### Test Commands

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run specific test
go test -v -run TestName ./...

# Run tests and format code
go fmt ./... && go test ./...
```

### Database Tests

Some tests require a running PostgreSQL database:

```bash
# Start test database
docker compose up -d

# Tests will skip if database is unavailable (safe in CI)
```

Environment configuration:
- The default environment resolves to `postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable`.
- Override credentials by editing `.env.local` or by passing `--target`/`--shadow-db` in tests instead of exporting shell variables.

### Test-Driven Development

When adding features:

1. **Write the test first** - Define expected behavior
2. **Run the test** - Should fail (red)
3. **Implement the feature** - Make it work
4. **Run the test** - Should pass (green)
5. **Refactor** - Clean up code while tests stay green

## Code Quality

### Before Committing

**ALWAYS run these commands before pushing to ensure code quality:**

```bash
# Format code
go fmt ./...

# Vet code for issues (catches common errors)
go vet ./...

# Run tests
go test ./...
```

**Critical**: Always run `go vet ./...` before pushing. It catches errors like:
- Unchecked error return values
- Printf format string issues
- Unreachable code
- Common mistakes

### Linting

The project uses `golangci-lint` in CI. Common issues:
- Unused variables or imports
- Non-constant format strings in fmt.Errorf (use `%s` placeholder)
- Missing comments on exported functions
- Inconsistent formatting

## Development Workflow

### When Adding New Features or Changing Behavior

**ALWAYS FOLLOW THE CHECKLIST ABOVE - NO EXCEPTIONS**

The checklist ensures:
1. Code quality through testing and vetting
2. User experience through documentation
3. Version control through git commits
4. Team collaboration through git push

**Common mistake:** Making code changes and forgetting to update docs or commit. This leaves the repository in an inconsistent state.

### Documentation Files to Check

When making changes to CLI commands or workflows:
- [ ] `README.md` - Main documentation with examples
- [ ] `docs/getting_started.md` - Step-by-step guide for new users
- [ ] `main.go` - `printHelp()` function
- [ ] `docs/prisma.md` - If relevant to Prisma integration
- [ ] `docs/supabase.md` - If relevant to Supabase integration
- [ ] `docs/alembic.md` - If relevant to Alembic integration

## Common Tasks

### Adding a New CLI Flag

1. Add the flag in the relevant `run*()` function using `flag.NewFlagSet()`
2. Update the usage message in that function
3. Update `printHelp()` in `main.go`
4. Add example in `README.md`
5. Add example in `docs/getting_started.md`
6. **Follow the complete checklist above**

### Adding a New Command

1. Add case in `main()` switch statement
2. Create `run*()` handler function
3. Update `printHelp()` function
4. Add documentation in `README.md`
5. Add workflow examples in `docs/getting_started.md`
6. Write tests
7. **Follow the complete checklist above**

### Modifying Migration Logic

1. Update the relevant planner/rollback/diff file
2. Add or update tests
3. Check if examples need updating
4. Document any breaking changes
5. **Follow the complete checklist above**

## Schema Format

**Current format: JSON + JSON Schema**

Lockplane uses JSON Schema for validation. Schema files reference:
- `https://raw.githubusercontent.com/zakandrewking/lockplane/main/schema-json/schema.json`
- `https://raw.githubusercontent.com/zakandrewking/lockplane/main/schema-json/plan.json`
- Replace `main` with version tags (e.g., `v0.1.0`) for stability
- Examples: `examples/schemas-json/`

## Important Principles

1. **Schema is the source of truth** - Migration plans are generated on demand
2. **Shadow DB validation** - Always test migrations before production
3. **Reversibility** - Every forward migration should have a rollback
4. **Explainability** - Every operation should have a clear description
5. **Safety** - Validate migrations before execution

## Documentation Standards

- Use clear, concise language
- Provide working examples
- Show both success and error cases
- Include environment variable requirements
- Document all CLI flags and options
- Keep examples up to date with code changes

## Git Workflow

- Work on `main` branch (small project, no PR workflow yet)
- Always run tests before pushing
- Use descriptive commit messages (see Phase 4 of checklist)
- Always push after committing

## Common Pitfalls

1. **Forgetting to run tests** - This is the most common issue. Always run tests!
2. **Not running `go vet`** - Catches common errors before they become bugs
3. **Not updating test fixtures** - When changing schema format, update test files too
4. **Database not running** - Tests will skip, which might hide issues
5. **Format string errors** - Use `fmt.Errorf("%s", msg)` not `fmt.Errorf(msg)`
6. **Forgetting to update docs** - Documentation must match code
7. **Not committing or pushing** - Work isn't complete until it's pushed

## Continuous Integration

Tests run automatically on:
- Push to `main` branch
- Pull request creation
- All commits are linted with `golangci-lint`

## Remember

**THE COMPLETE WORKFLOW CHECKLIST AT THE TOP IS MANDATORY**

This is not optional. The checklist ensures:

1. **Documentation stays in sync** - Users can trust the docs match the code
2. **Tests prevent regressions** - Changes don't break existing functionality
3. **Git history is complete** - All work is tracked and shareable
4. **Remote stays updated** - Team members can see the latest changes

**Incomplete work is:**
- ❌ Code changes without tests
- ❌ Code changes without running `go vet`
- ❌ Code changes without documentation updates
- ❌ Code changes without git commit
- ❌ Git commits without git push
- ❌ Skipping any step in the checklist

**Complete work is:**
- ✅ All checklist items completed
- ✅ Code formatted with `go fmt`
- ✅ Code vetted with `go vet`
- ✅ Tests pass
- ✅ Build succeeds
- ✅ Documentation updated
- ✅ Changes committed and pushed
- ✅ User informed of what was done

## Questions?

See:
- `README.md` - Project overview and usage
- `docs/getting_started.md` - Step-by-step guide
- `0001-design.md` - Architecture and design decisions
