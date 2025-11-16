# Shadow Database Consistency Project

**Status**: ✅ Completed (2025-11-15)
**Goal**: Align shadow database documentation, tooling, and wizard UX so users always understand how migrations stay safe.

---

## Outcomes

1. **Documentation is accurate across the board**
   - README.md, CLAUDE.md, llms.txt, and Supabase docs now describe all three shadow DB strategies (PostgreSQL, SQLite, libSQL/Turso).
   - Examples show the real `_shadow.db` files and the default PostgreSQL `<database>_shadow` + port `5433` behavior.
   - The manual configuration section highlights overriding shadow URLs or using `SHADOW_SCHEMA` for schema-based isolation.

2. **Init wizard now surfaces shadow DB behavior**
   - After choosing a database type (or PostgreSQL input method), the wizard shows a dedicated “Shadow Database Preview” screen that explains exactly what will be provisioned.
   - Users can press Esc to revisit their database selection before entering credentials.
   - The post-connection summary displays each environment’s primary and shadow targets before any files are written, reinforcing that migrations are validated safely.

3. **Per-environment summary before saving**
   - The summary screen now lists every new or updated environment with:
     - Primary connection (host/file/url)
     - Shadow configuration preview (e.g., `user@host:5433/app_shadow`, `./schema/mydb_shadow.db`, or `./schema/turso_shadow.db`)
   - Clear “Enter to create files / Esc to go back” messaging ensures users can review shadow settings before anything hits disk.

---

## Implementation Details

### Documentation
- README wizard section gained a dedicated bullet explaining the new preview step plus a note that summaries include shadow info.
- Existing references to “PostgreSQL-only shadow DBs” or “SQLite uses :memory:” were removed (already backed by schema-based support and file-backed SQLite shadow DBs).
- No new flags were introduced—shadow overrides remain available via `.env`, `--shadow-db`, or `SHADOW_SCHEMA`.

### Wizard Enhancements (`internal/wizard/wizard.go`)
- Added `StateShadowInfo` and `renderShadowInfo()` with database-specific messaging.
- Updated flow so every environment (Postgres, SQLite, libSQL) pauses on the explanation screen before input.
- Summary view now uses helper formatters to print primary and shadow targets. Pressing “Save and finish” or Esc from the “Add another?” step always routes through this confirmation screen first.
- Extended tests (`internal/wizard/wizard_test.go`) to cover the new state transitions and summary behavior.

---

## Testing

- `go fmt ./...`
- `go vet ./...`
- `errcheck ./...`
- `staticcheck ./...`
- `go test -v ./...`
- `go build .`
- `go install .`

---

## Future Considerations

- **Advanced customization**: Provide “Advanced shadow options” inside the wizard to tweak port/path without editing `.env`.
- **Non-interactive flags**: Mirror those advanced options for `lockplane init --yes`.
- **Additional presets**: Offer Turso/libSQL templates that explicitly show the local SQLite shadow DB in the wizard summary.

These items can spin out into a follow-up project if/when users need more control over automatic shadow configuration.
