# Environment-Based Configuration Migration Plan

## 1. Current State Audit
- [x] Map every usage of `DATABASE_URL` / `SHADOW_DATABASE_URL` across code, tests, samples, and docs.
- [x] Document where priority ordering (`flags → env vars → config → defaults`) is implemented, especially in `config.go`, command handlers in `main.go`, helper functions, fixtures, `lockplane.toml.example`, and documentation (`README.md`, `docs/`, plugin skill files).

## 2. New Configuration Model
- [x] Evolve `Config` to include a `default_environment` (defaulting to `local`) and `[environments.<name>]` definitions. (Design: `Config` gains `DefaultEnvironment`, `SchemaPath`, `Environments map[string]EnvironmentConfig`, plus internal `configDir` metadata.)
- [x] Ensure each environment references a `.env.<name>` file by convention and may inline URL values for non-dotenv use cases. (Design: `EnvironmentConfig` supports inline `database_url` / `shadow_database_url`; resolver overlays values from `.env.<name>` when present.)
- [x] Define command-specific environment selection flags (e.g., `--target-environment` for apply) that map to named environments in the TOML file. (Design: apply/plan/rollback share `--target-environment`; introspect/diff get `--source-environment` for DB lookups.)

## 3. Environment Loading Utilities
- [x] Implement helpers to locate `lockplane.toml`, resolve the selected environment, and read the associated `.env.<name>` file (via `github.com/joho/godotenv`) to extract connection details.
- [x] Remove direct `os.Getenv` lookups; replace `GetDatabaseURL` / `GetShadowDatabaseURL` logic with `explicit flag → resolved environment → config defaults → hard-coded fallback`.
- [x] Provide precise error messages for missing environments, absent `.env` files, or incomplete key data.

## 4. CLI Command Updates
- [x] Thread the new environment resolution through every `run*` handler, introducing `--target-environment` for apply/plan/rollback and a counterpart (e.g., `--source-environment`) for introspect/diff, while dropping the legacy `getEnv` helper.
- [x] Update usage text, help output, and error messages to guide users toward environment-based configuration rather than shell exports.
- [x] Ensure schema path handling remains consistent with the new environment model.

## 5. Testing Strategy
- [x] Refactor existing tests that depended on environment variables to use temporary `.env.<name>` files or inline environment definitions.
- [x] Add unit tests covering environment selection, defaulting behavior, missing file handling, and conflict cases between flags and environments.
- [x] Review fixtures to ensure they align with the new configuration approach (no updates required).

## 6. Documentation Updates
- [x] Rewrite `README.md`, `docs/getting_started.md`, and relevant docs in `docs/` to describe the environment workflow and `.env.<name>` files.
- [x] Update `lockplane.toml.example` and other samples to demonstrate the new structure.
- [x] Add a final step to the README checklist explaining why environments matter and how to configure them, giving newcomers actionable guidance.
- [x] Refresh agent-focused guides (`AGENTS.md`, Claude skill files, `llms.txt`) so assistants stop recommending direct env var usage.
- [x] Remove references to `DATABASE_URL` / `SHADOW_DATABASE_URL` from docs and plugin collateral.

## 7. Verification & Workflow
- [x] After implementation, run `go fmt ./...`, `go vet ./...`, `go test -v ./...`, and `go build .`.
- [x] Follow the project checklist for git operations: review `git status`, inspect diffs, stage, commit with the prescribed format, and push.
- [ ] Summarize the completed work and remaining considerations for future iterations.
