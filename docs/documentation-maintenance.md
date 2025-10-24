# Documentation Maintenance Strategy

This document describes how we keep all Lockplane documentation in sync.

## Documentation Sources

We maintain documentation in multiple locations for different audiences:

1. **README.md** - Quick start, installation, key features
2. **docs/** - Detailed guides and design documents
3. **.claude/skills/lockplane.md** - Claude Code skill
4. **llms.txt** - LLM context file (llmstxt.org standard)
5. **CLI help text** - Built into the code (`main.go`, help functions)

## Sync Strategy

### 1. Source of Truth Hierarchy

Different content has different sources of truth:

| Content Type | Source of Truth | Sync Method |
|--------------|----------------|-------------|
| CLI commands & flags | Code (main.go, help functions) | Extract & test |
| Command examples | Shared snippets | Include/copy |
| Core concepts | README.md | Copy to llms.txt, skill |
| Detailed workflows | docs/ | Reference from others |
| Safety validations | validate_sql_safety.go | Document what code does |

### 2. Update Checklist

When making changes, update in this order:

#### Adding a new command
- [ ] Implement in `main.go`
- [ ] Add help text with `fs.Usage = func() { ... }`
- [ ] Update `printHelp()` in `main.go`
- [ ] Add to README.md examples
- [ ] Add to `.claude/skills/lockplane.md` commands section
- [ ] Add to `llms.txt` commands section
- [ ] Add to relevant `docs/` files
- [ ] Add example to test suite

#### Adding a new validation
- [ ] Implement in `validate_sql_safety.go`
- [ ] Add error message with recommendation
- [ ] Add to README.md validation section
- [ ] Add to `.claude/skills/lockplane.md` safety section
- [ ] Add to `llms.txt` safety section
- [ ] Add example to `examples/dangerous/`
- [ ] Document in design doc if applicable

#### Changing behavior
- [ ] Update code
- [ ] Update help text
- [ ] Update all examples that use it
- [ ] Update README.md
- [ ] Update skill & llms.txt
- [ ] Update docs/
- [ ] Update CHANGELOG.md

### 3. Shared Content Snippets

Common content that appears in multiple places:

**Installation snippet** (used in README, skill, llms.txt):
```bash
go install github.com/zakandrewking/lockplane@latest
# OR
curl -L https://github.com/zakandrewking/lockplane/releases/latest/download/lockplane_linux_amd64.tar.gz | tar xz
```

**Quick start snippet** (used in README, llms.txt):
```bash
lockplane init docker-compose
docker compose up -d
lockplane introspect > current.json
lockplane apply --auto-approve --from current.json --to schema/ --validate
```

**Config file snippet** (used in README, docs/, skill, llms.txt):
```toml
database_url = "postgresql://user:password@localhost:5432/myapp?sslmode=disable"
shadow_database_url = "postgresql://user:password@localhost:5433/myapp_shadow?sslmode=disable"
schema_path = "lockplane/schema/"
```

When updating these, search for them across all files.

### 4. Automated Validation

We use automated tests to catch documentation drift:

**Command output validation** (`docs_test.go`):
```go
// Test that CLI help matches what we document
func TestHelpText(t *testing.T) {
    cmd := exec.Command("./lockplane", "--help")
    output, _ := cmd.Output()

    // Verify documented commands exist
    assert.Contains(t, string(output), "introspect")
    assert.Contains(t, string(output), "validate")
    // ... etc
}
```

**Example validation** (run in CI):
```bash
# Test that examples in README actually work
./test-readme-examples.sh
```

**Consistency validation**:
```bash
# Check that llms.txt and skill mention same commands
./scripts/check-docs-consistency.sh
```

### 5. Review Checklist

Before releasing a new version:

- [ ] Run `go test ./...` (includes docs tests)
- [ ] Run `./scripts/check-docs-consistency.sh`
- [ ] Manually review CHANGELOG.md
- [ ] Check that README examples work
- [ ] Verify help text is up to date
- [ ] Test Claude skill in actual Claude Code session
- [ ] Verify llms.txt has all new features

## Common Maintenance Tasks

### Updating Command Examples

When CLI syntax changes:

1. Update in code first (source of truth)
2. Run `go build .`
3. Run `./lockplane <command> --help` to see new output
4. Update examples in:
   - README.md
   - .claude/skills/lockplane.md
   - llms.txt
   - Relevant docs/ files
5. Update tests

### Adding New Integration Guide

When documenting integration with a new tool (e.g., Django):

1. Create `docs/django.md` with detailed guide
2. Add brief mention in README.md
3. Add example to `.claude/skills/lockplane.md`
4. Add example to `llms.txt`
5. Link from README to the detailed guide

### Deprecating a Feature

When removing or changing a feature:

1. Update code
2. Add deprecation notice to help text
3. Update CHANGELOG.md
4. Update all docs to show new approach
5. Keep old approach documented with "deprecated" note for one version
6. Remove old documentation after deprecation period

## Tools & Scripts

### scripts/check-docs-consistency.sh

Checks for common issues:
- Commands mentioned in docs exist in CLI
- Examples use correct flag syntax
- All validation codes documented
- Version numbers match

### scripts/extract-help-text.sh

Extracts help text from CLI for inclusion in docs:
```bash
./scripts/extract-help-text.sh > docs/cli-reference.md
```

### scripts/test-examples.sh

Runs all code examples from documentation:
```bash
./scripts/test-examples.sh README.md
./scripts/test-examples.sh .claude/skills/lockplane.md
```

## Documentation Owners

Different sections have different owners:

| Section | Owner | Update Frequency |
|---------|-------|------------------|
| CLI reference | Auto-generated from code | Every release |
| Getting started | Manual | When workflow changes |
| Integration guides | Manual | When tools change |
| Safety validations | Auto-documented from code | When new validations added |
| Design docs | Manual | When new features designed |

## Future Improvements

1. **Auto-generate CLI reference** from help text
2. **Literate programming** - Extract examples from tests
3. **Documentation CI** - Fail build if docs inconsistent
4. **Snippet includes** - DRY for shared content
5. **Automated screenshots** - For visual guides
6. **Documentation versioning** - Per-release docs

## Quick Reference: Where Does Content Live?

| Content | README | docs/ | skill | llms.txt |
|---------|--------|-------|-------|----------|
| Quick start | ✅ Primary | Reference | ✅ Copy | ✅ Copy |
| Full commands | Examples | ✅ Primary | ✅ Copy | ✅ Copy |
| Workflows | Examples | ✅ Primary | ✅ Copy | Summarized |
| Integration guides | Link | ✅ Primary | Examples | Examples |
| Design decisions | Link | ✅ Primary | ❌ | ❌ |
| Safety validations | List | Details | ✅ Full | ✅ Full |
| Troubleshooting | Basic | ✅ Primary | ✅ Copy | ✅ Copy |
| API/internals | ❌ | ✅ Primary | ❌ | ❌ |

Legend:
- ✅ Primary = Source of truth
- ✅ Copy = Copy from primary
- ✅ Full = Full content included
- Link = Reference to primary
- Examples = Selected examples only
- Summarized = Brief version
- ❌ = Not included
