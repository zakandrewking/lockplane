# Shadow Database Consistency Project

**Status**: ✅ Completed (2025-11-16)
**Goal**: Align docs, CLI UX, and automation so every workflow clearly exposes schema-based and database-based shadow options in the required order.

---

## Completed Checklist

### Phase A: Documentation & Project Tracking
- [x] Move the project document back under `devdocs/projects/` while in progress, then archive here after completion
- [x] Update README Shadow DB + manual configuration examples to highlight both `POSTGRES_SHADOW_URL` and `SHADOW_SCHEMA`
- [x] Document schema-based selection in wizard copy and environment summaries

### Phase B: Wizard Flow Rework
- [x] Enforce required flow (DB → Postgres input method → connection details → shadow strategy → shadow details → test)
- [x] Replace the legacy `StateShadowInfo/StateShadowAdvanced` with the new `StateShadowOptions` + `StateShadowDetails` phases
- [x] Provide database-specific defaults:
  - PostgreSQL: choose between `<db>_shadow` via dedicated port vs. schema mode, collecting port/schema input
  - SQLite/libSQL: confirm recommended shadow file path or enter a custom override
- [x] Wire the new state data into summaries and `.env` generation (including `SHADOW_SCHEMA`)

### Phase C: Tests & Verification
- [x] Extend wizard tests to cover schema mode, SQLite normalization, libSQL defaults, and new navigation states
- [x] Update `.env.example` to include the schema override guidance
- [x] Run the full Go quality + build checklist

---

## Key Outcomes

1. **README + manual setup flows now explain schema-based isolation up front.** Examples show both `POSTGRES_SHADOW_URL` and `SHADOW_SCHEMA`, and the wizard section describes the decision point explicitly.
2. **Wizard UX mirrors the desired decision order.** After entering connection info, users land on a dedicated shadow strategy screen, then a focused detail form that captures either the port or schema (Postgres) or the exact file path (SQLite/libSQL).
3. **Env outputs & summaries accurately reflect schema mode.** The summary prints either `user@host:port/db_shadow` or `db (schema: <name>)`, `.env` files emit `SHADOW_SCHEMA` when selected, and `.env.example` now teaches overriding via schema.
4. **Regression coverage expanded.** New tests exercise the separate DB defaults, schema inputs, libSQL defaults, and back-navigation, ensuring the flow can’t regress silently.

---

## Implementation Notes

- `internal/wizard/wizard.go`
  - Added `StateShadowOptions`/`StateShadowDetails`, keyboard handlers, and focus management for the new text inputs.
  - `renderConnectionDetails` now describes the next (shadow) step, and `renderShadowOptions` renders database-specific labels (`<db>_shadow on port 5433`, `SHADOW_SCHEMA`, normalized file paths, etc.).
  - Connection detail collection resets validation errors each time, `collectShadowDetailValues` normalizes SQLite/Turso paths, and summaries only display the schema tip when applicable.
- `internal/wizard/generation.go`
  - `.env` generation writes either `POSTGRES_SHADOW_URL` or `SHADOW_SCHEMA`.
  - `.env.example` gained a sample `SHADOW_SCHEMA` line so users discover schema mode without digging.
- `internal/wizard/wizard_test.go`
  - Added coverage for schema selection, separate DB port overrides, libSQL defaults, SQLite normalization, and validation reset behavior.
- `README.md` + supporting docs
  - Reordered the wizard bullet list to match the new state machine and highlighted the schema option everywhere shadow behavior is discussed.

---

## Testing & Verification

✅ `go fmt ./...`
✅ `go vet ./...`
✅ `errcheck ./...`
✅ `staticcheck ./...`
✅ `go test -v ./...`
✅ `go build .`
✅ `go install .`
