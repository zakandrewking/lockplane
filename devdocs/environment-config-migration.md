# Environment-Based Configuration Migration Plan

## 1. Current State Audit
- Map every usage of `DATABASE_URL` / `SHADOW_DATABASE_URL` across code, tests, samples, and docs.
- Document where priority ordering (`flags → env vars → config → defaults`) is implemented, especially in `config.go`, command handlers in `main.go`, helper functions, fixtures, `lockplane.toml.example`, and documentation (`README.md`, `docs/`, plugin skill files).

## 2. New Configuration Model
- Evolve `Config` to include a `default_environment` (defaulting to `local`) and `[environments.<name>]` definitions.
- Each environment references a `.env.<name>` file (implicit default naming, with optional `env_file` override) and may inline URL values for non-dotenv use cases.
- Introduce a global `--env` flag respected by all CLI commands, with precedence and defaults aligned with the TOML configuration.

## 3. Environment Loading Utilities
- Implement helpers to locate `lockplane.toml`, resolve the selected environment, and read the associated `.env.<name>` file (via `github.com/joho/godotenv` or similar) to extract connection details.
- Remove direct `os.Getenv` lookups; replace `GetDatabaseURL` / `GetShadowDatabaseURL` logic with `explicit flag → resolved environment → config defaults → hard-coded fallback`.
- Provide precise error messages for missing environments, absent `.env` files, or incomplete key data.

## 4. CLI Command Updates
- Thread the new environment resolution through every `run*` handler and drop the `getEnv` helper.
- Update usage text, help output, and error messages to guide users toward environment-based configuration rather than shell exports.
- Ensure schema path handling remains consistent with the new environment model.

## 5. Testing Strategy
- Refactor existing tests that depended on environment variables to use temporary `.env.<name>` files or inline environment definitions.
- Add unit tests covering environment selection, defaulting behavior, missing file handling, and conflict cases between flags and environments.
- Review fixtures to ensure they align with the new configuration approach.

## 6. Documentation Updates
- Rewrite `README.md`, `docs/getting_started.md`, and relevant docs in `docs/` to describe the environment workflow and `.env.<name>` files.
- Update `lockplane.toml.example` and other samples to demonstrate the new structure.
- Remove references to `DATABASE_URL` / `SHADOW_DATABASE_URL` from docs and plugin collateral.

## 7. Verification & Workflow
- After implementation, run `go fmt ./...`, `go vet ./...`, `go test -v ./...`, and `go build .`.
- Follow the project checklist for git operations: review `git status`, inspect diffs, stage, commit with the prescribed format, and push.
- Summarize the completed work and remaining considerations for future iterations.
