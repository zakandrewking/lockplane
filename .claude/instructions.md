# Lockplane Development Instructions

This file contains project-specific instructions for AI assistants working on Lockplane.

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

### Test Organization

- `*_test.go` - Test files colocated with implementation
- `testdata/` - Test fixtures and golden files
  - `testdata/plans-json/` - JSON migration plan fixtures
  - `testdata/fixtures/` - SQL schema fixtures (currently being migrated to JSON)

### Database Tests

Some tests require a running PostgreSQL database:

```bash
# Start test database
docker compose up -d

# Tests will skip if database is unavailable (safe in CI)
```

Environment variables:
- `DATABASE_URL` - Main Postgres connection (default: `postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable`)
- `SHADOW_DATABASE_URL` - Shadow DB for validation (default: `postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable`)

### Test-Driven Development

When adding features:

1. **Write the test first** - Define expected behavior
2. **Run the test** - Should fail (red)
3. **Implement the feature** - Make it work
4. **Run the test** - Should pass (green)
5. **Refactor** - Clean up code while tests stay green

### Continuous Integration

Tests run automatically on:
- Push to `main` branch
- Pull request creation
- All commits are linted with `golangci-lint`

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

## Schema Definitions

**Current format: JSON + JSON Schema**

- Schema definitions are in JSON (not CUE anymore)
- JSON Schema validation: `schema-json/schema.json`, `schema-json/plan.json`
- Examples: `examples/schemas-json/`

## Git Workflow

- Work on `main` branch (small project, no PR workflow yet)
- Always run tests before pushing
- Use descriptive commit messages
- Include `🤖 Generated with [Claude Code](https://claude.com/claude-code)` footer

## Common Pitfalls

1. **Forgetting to run tests** - This is the most common issue. Always run tests!
2. **Not updating test fixtures** - When changing schema format, update test files too
3. **Database not running** - Tests will skip, which might hide issues
4. **Format string errors** - Use `fmt.Errorf("%s", msg)` not `fmt.Errorf(msg)`

## Questions?

See:
- `README.md` - Project overview and usage
- `GETTING_STARTED.md` - Step-by-step guide
- `0001-design.md` - Architecture and design decisions
