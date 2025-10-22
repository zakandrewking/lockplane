# Change Log

All notable changes to the "lockplane" extension will be documented in this file.

## [0.1.0] - 2025-10-21

### Added
- Initial release
- Real-time validation for .lp.sql files
- Validate on save and on type
- Integration with lockplane CLI
- Diagnostic reporting in Problems panel
- Inline error squiggles
- Configuration options for schema path and validation behavior
- Multi-file schema validation

### Known Issues
- Validation uses CLI spawning (may have slight latency)
- Limited to validation only (no autocomplete, hover, etc.)

## Future

### [0.2.0] - Planned
- Language Server Protocol (LSP) implementation for better performance
- Hover tooltips showing table/column information
- Go to definition for table references
- Find references for tables

### [0.3.0] - Planned
- Schema-aware autocomplete
- Workspace symbols for quick navigation
- Code actions and quick fixes
