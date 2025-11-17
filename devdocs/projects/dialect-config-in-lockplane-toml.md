# Dialect Configuration via `lockplane.toml`

**Status**: 🧭 Proposed (2025-11-17)

## Goal
Move the per-file dialect declaration (currently a comment like `-- dialect: sqlite`) into a first-class option within `lockplane.toml`, making schema dialect selection explicit, tool-friendly, and easier to discover.

## Motivation
- Comments are invisible to tooling and easy to forget.
- Having dialect defined in config improves auto-complete, validation, and CLI behavior.
- Aligns with other Lockplane configuration moving into `lockplane.toml` / `.env`.

## Open Questions
1. **Scope:** Should dialect be set per-schema file, per-directory, or globally per environment?
2. **Backwards compatibility:** How do existing comment-based declarations co-exist? Do we deprecate them or allow both with deterministic precedence?
3. **CLI defaults:** How do `lockplane plan/apply` behave if dialect isn’t specified?

## Initial Plan

### Phase 1 – Design (WIP)
- [ ] Decide on configuration shape (e.g., `schema_defaults.dialect = "sqlite"` or per-environment key).
- [ ] Define precedence order among CLI flags, `lockplane.toml`, `.env`, and inline comments.
- [ ] Document migration path for existing users.

### Phase 2 – Implementation
- [ ] Extend config loader to parse the new option with validation + defaults.
- [ ] Update schema loader to consume the config value when dialect isn’t specified inline.
- [ ] Add warning/deprecation notice when both comment and config disagree.

### Phase 3 – Tooling & Tests
- [ ] Update tests covering config resolution, schema parsing, and CLI flows.
- [ ] Ensure `lockplane init` wizard surfaces the new option when relevant.

### Phase 4 – Documentation
- [ ] Update README + docs to highlight the new config setting with examples.
- [ ] Add migration guide snippet for teams moving from inline comments.

## Risks
- Breaking existing comment-based workflows if precedence isn’t clearly defined.
- Introducing conflicting configuration (config vs comments vs CLI) without clear messaging.

## Next Steps
- Finalize the configuration schema and precedence rules.
- Prototype config parsing with feature flag to gather feedback.
