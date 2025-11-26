# Lockplane Restructure - November 26, 2025

## Summary

All existing lockplane code has been moved into `lockplane-vibe/` directory. This creates a clean separation between the vibe-coded version and the future curated version that will live at the root.

## What Was Moved

Everything except `.git` was moved from the root into `lockplane-vibe/`:

### Configuration Files
- `.claude-plugin/` - Claude plugin configuration
- `.github/` - GitHub Actions workflows and configuration
- `.gitignore` - Git ignore patterns
- `.golangci.yml` - Go linting configuration
- `.goreleaser.yml` - GoReleaser configuration for releases
- `.npmignore` - NPM ignore patterns
- `.pre-commit-config.yaml` - Pre-commit hook configuration
- `.python-version` - Python version specification
- `.venv/` - Python virtual environment
- `_config.yml` - Jekyll site configuration
- `codecov.yml` - Code coverage configuration
- `go.mod` - Go module definition
- `go.sum` - Go module checksums
- `package.json` - NPM package definition
- `pyproject.toml` - Python project configuration
- `uv.lock` - UV lock file

### Documentation
- `AGENTS.md` - Agent documentation
- `CLAUDE.md` - Claude Code project instructions
- `LICENSE` - Project license
- `NPM_PUBLISH.md` - NPM publishing documentation
- `README.md` - Main project README (original, now in lockplane-vibe/)
- `index.md` - Jekyll index page
- `llms.txt` - LLM context file

### Source Code Directories
- `cmd/` - CLI command implementations
- `database/` - Database driver implementations
- `diagnostic/` - Diagnostic utilities
- `internal/` - Internal packages:
  - `config/` - Configuration handling
  - `executor/` - Plan execution
  - `introspect/` - Schema introspection
  - `locks/` - Lock analysis and management
  - `parser/` - SQL parsing
  - `planner/` - Migration planning
  - `schema/` - Schema operations
  - `shadow/` - Shadow database handling
  - `sqliteutil/` - SQLite utilities
  - `sqlvalidation/` - SQL validation
  - `state/` - State management
  - `strutil/` - String utilities
  - `testutil/` - Test utilities
  - `validation/` - Validation utilities
  - `wizard/` - Interactive wizard

### Build and Development
- `bin/` - Binary output directory
- `scripts/` - Build and development scripts
- `Dockerfile.goreleaser` - Docker build configuration
- `main.go` - Main entry point
- `lockplane` - Compiled binary
- `coverage.txt` - Test coverage output

### Documentation and Examples
- `devdocs/` - Development documentation
- `docs/` - User documentation
- `examples/` - Example schemas and plans
- `schema-json/` - JSON schema definitions

### Testing
- `tests/` - Integration tests

### Other
- `assets/` - Static assets
- `claude-plugin/` - Claude plugin source
- `tmp/` - Temporary files
- `vscode-lockplane/` - VS Code extension
- `_includes/` - Jekyll includes
- `_sass/` - Jekyll stylesheets
- `index.js` - NPM package entry point

## What Stayed at Root

- `.git/` - Git repository (must stay at root)
- `lockplane-vibe/` - New directory containing all the moved code
- `README.md` - New empty README (to be filled with curated content)
- `RESTRUCTURE.md` - This file documenting the changes

## Directory Structure After Restructure

```
lockplane/
├── .git/                     # Git repository (unchanged)
├── lockplane-vibe/           # All original code moved here
│   ├── .github/
│   ├── cmd/
│   ├── database/
│   ├── docs/
│   ├── examples/
│   ├── internal/
│   ├── scripts/
│   ├── tests/
│   ├── go.mod
│   ├── main.go
│   ├── README.md            # Original README
│   └── ... (all other files)
├── README.md                 # New empty README
└── RESTRUCTURE.md           # This documentation
```

## Commands Executed

```bash
# 1. Create the lockplane-vibe directory
mkdir -p lockplane-vibe

# 2. Move dot files and directories (except .git)
mv .claude-plugin .github .gitignore .golangci.yml .goreleaser.yml \
   .npmignore .pre-commit-config.yaml .python-version .venv lockplane-vibe/

# 3. Move documentation and config files
mv AGENTS.md CLAUDE.md Dockerfile.goreleaser LICENSE NPM_PUBLISH.md \
   README.md _config.yml codecov.yml index.md llms.txt lockplane-vibe/

# 4. Move all directories
mv _includes _sass assets bin claude-plugin cmd database devdocs diagnostic \
   docs examples internal schema-json scripts tests tmp vscode-lockplane lockplane-vibe/

# 5. Move remaining files
mv go.mod go.sum index.js main.go package.json pyproject.toml \
   uv.lock coverage.txt lockplane lockplane-vibe/
```

## Impact

### ✅ What Works
- Git history is preserved (all in `.git/` at root)
- All files are safely moved with no data loss
- Directory structure is clean and organized

### ⚠️ Temporarily Broken (Expected)
- **Tests**: All test paths will need updating or running from `lockplane-vibe/`
- **Builds**: Go builds will need to be run from `lockplane-vibe/` directory
- **CI/CD**: GitHub Actions will need path updates or workflow changes
- **Documentation site**: Jekyll paths may need updating
- **NPM package**: Package references will need updating
- **Import paths**: No Go import paths broken (all relative within lockplane-vibe/)

## Next Steps

1. ✅ Create fresh README.md at root (completed)
2. ✅ Document restructure (this file)
3. ⏭️ Gradually extract solid features from `lockplane-vibe/` to root
4. ⏭️ Build curated version at root level
5. ⏭️ Update CI/CD to work with new structure when ready

## Notes

- This restructure was done with `mv` commands - no copying, so it's clean and efficient
- Git will track this as a rename operation in the history
- The lockplane-vibe code remains fully functional in its subdirectory
- Root level is now a clean slate for building the curated version
