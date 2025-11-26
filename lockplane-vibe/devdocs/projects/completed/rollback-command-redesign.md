# Rollback Command Redesign

## Progress Checklist
- [x] Phase 1: Planning and Design
  - [x] Create project plan document
  - [x] Review and validate approach
- [x] Phase 2: Rename `rollback` to `plan-rollback`
  - [x] Rename command handler function (runRollback → runPlanRollback)
  - [x] Update command routing in main()
  - [x] Update help strings and error messages
  - [x] Update printHelp() documentation
  - [ ] Update README.md examples (deferred - will do with full impl)
  - [ ] Update all documentation files (deferred - will do with full impl)
  - [ ] Update test files (no test changes needed yet)
  - [x] Run tests to verify
- [x] Phase 3: Implement new `rollback` command
  - [x] Create runRollback() function (apply-like)
  - [x] Implement plan generation internally
  - [x] Add --from flag for before schema (required)
  - [x] Add interactive approval prompt
  - [x] Add --auto-approve flag support
  - [x] Add shadow DB validation with --skip-shadow flag
  - [x] Add verbose logging support
  - [x] Add colored output matching apply command
- [x] Phase 4: Testing
  - [x] Core rollback logic tested in internal/planner/rollback_test.go
  - [x] applyPlan execution tested in integration tests
  - [x] Manual verification of both commands (help text, basic functionality)
  - Note: CLI command-level tests would require refactoring for testability (future work)
- [x] Phase 5: Documentation
  - [x] printHelp() already documents both commands
  - [x] Updated README.md with workflow examples (both plan-rollback and rollback)
  - [x] Updated docs/alembic.md to use plan-rollback
  - [x] Updated docs/sqlalchemy.md to use plan-rollback and show both workflows
  - [x] Ran ./scripts/check-docs-consistency.sh - all checks passed
- [x] Phase 6: Final verification
  - [x] All tests pass (core logic tested, commands verified)
  - [x] CI passes
  - [x] Documentation is consistent

## Context

Currently, `lockplane rollback` generates a rollback plan (JSON) from a forward migration plan. This requires users to:
1. Generate forward plan
2. Generate rollback plan
3. Apply rollback plan separately

This is cumbersome. We want to make rollback work more like `apply`:
1. User runs `lockplane rollback --plan forward.json --target db`
2. System generates rollback plan internally
3. System shows the plan and asks for approval
4. System applies the rollback

## Goals

1. **Rename `rollback` → `plan-rollback`**: Preserve current plan generation functionality
2. **Create new `rollback` command**: Make it work like `apply` but for rollbacks
3. **Maintain backward compatibility**: Old workflows should still work with new command names
4. **Improve UX**: Make rollback as easy as `apply`

## Design

### Command Structure

**Before:**
```bash
# Generate rollback plan
lockplane rollback --plan forward.json --from before.json > rollback.json

# Apply rollback
lockplane apply rollback.json --target-environment production
```

**After:**
```bash
# Option 1: Generate rollback plan (explicit)
lockplane plan-rollback --plan forward.json --from before.json > rollback.json
lockplane apply rollback.json --target-environment production

# Option 2: Generate and apply rollback (new, simpler)
lockplane rollback --plan forward.json --target-environment production
```

### New Command: `lockplane plan-rollback`

This is the renamed version of current `rollback` command.

**Function:** `runPlanRollback(args []string)`

**Flags:**
- `--plan <file>` - Forward migration plan (required)
- `--from <schema|db>` - Source schema (before state) (required)
- `--from-environment <name>` - Named environment for source schema
- `--verbose` / `-v` - Verbose logging

**Behavior:**
- Loads forward plan from file
- Loads "before" schema from file/directory/database
- Generates rollback plan (reverses operations)
- Outputs rollback plan JSON to stdout

**Output:** JSON plan file (same as current rollback)

### New Command: `lockplane rollback`

This is the new command that works like `apply`.

**Function:** `runRollback(args []string)` (new implementation)

**Flags:**
- `--plan <file>` - Forward migration plan to rollback (required)
- `--target <url>` - Target database URL
- `--target-environment <name>` - Named environment for target
- `--auto-approve` - Skip interactive approval
- `--skip-shadow` - Skip shadow DB validation (not recommended)
- `--shadow-db <url>` - Shadow database URL
- `--shadow-environment <name>` - Named environment for shadow DB
- `--verbose` / `-v` - Verbose logging

**Behavior:**
1. Load forward plan from file
2. Introspect target database (current state)
3. Validate source hash from forward plan
   - The target DB should be in the "after" state of the forward plan
   - Forward plan contains hash of "before" state
   - We need to check that target was modified by this plan
4. Generate rollback plan internally
   - Use the forward plan
   - Use the "before" schema from plan metadata OR introspect
5. Display rollback plan to user
6. Prompt for approval (unless --auto-approve)
7. Validate on shadow DB (unless --skip-shadow)
8. Apply rollback plan to target database
9. Report success/failure

**Output:** Execution result JSON (same as apply)

### Hash Validation Strategy

**Problem:** How do we validate that the rollback is being applied to the correct database state?

**Current forward plan structure:**
```json
{
  "source_hash": "abc123...",  // Hash of "before" state
  "steps": [...]
}
```

**For rollback, we need to check:**
- Target database is in the "after" state of the forward plan
- This means target should have been modified by the forward plan

**Approach:**
1. Compute hash of target database (current state)
2. Apply forward plan to before state (in memory) to compute expected "after" hash
3. Compare with target database hash
4. If mismatch, warn user and abort

**Alternative simpler approach:**
1. Store "target_hash" (after state hash) in forward plans going forward
2. For now, just check that source_hash exists
3. Warn user if we can't verify the database state

### Shadow DB Validation

For rollback, shadow DB validation should:
1. Apply current target state to shadow DB
2. Execute rollback plan on shadow DB
3. Verify it succeeds
4. Rollback shadow DB changes (always)

This is the same pattern as `apply`.

## Implementation Phases

### Phase 1: Planning and Design ✓
- [x] Create this document
- [ ] Review with stakeholders
- [ ] Finalize approach

### Phase 2: Rename `rollback` to `plan-rollback`

**Files to modify:**
1. `main.go`:
   - Rename `runRollback()` → `runPlanRollback()`
   - Update command routing in `main()` switch statement
   - Update `printHelp()` command list and examples
   - Update all error messages and help text

2. `README.md`:
   - Update all examples using `rollback` → `plan-rollback`
   - Add new workflow examples with new `rollback` command

3. `docs/getting_started.md`:
   - Update workflow examples

4. `docs/*.md`:
   - Search for "rollback" mentions
   - Update as appropriate

5. `CLAUDE.md` / `AGENTS.md`:
   - Update command references

6. Test files:
   - Search for test names with "rollback"
   - Update as needed

**Testing:**
- Run `go test ./...` to ensure nothing breaks
- Manually test: `lockplane plan-rollback -h`
- Verify help output is correct

### Phase 3: Implement new `rollback` command

**Create new `runRollback()` function in `main.go`:**

Structure (similar to `runApply()`):

```go
func runRollback(args []string) {
    // 1. Parse flags
    //    --plan (required)
    //    --target / --target-environment
    //    --auto-approve
    //    --skip-shadow
    //    --shadow-db / --shadow-environment
    //    --verbose

    // 2. Load forward plan

    // 3. Resolve target database connection

    // 4. Connect to target database

    // 5. Introspect target database (current state)

    // 6. Validate source hash (optional but recommended)
    //    - Check that target was modified by forward plan
    //    - Warn if can't verify

    // 7. Generate rollback plan internally
    //    - Need "before" schema from forward plan OR introspect
    //    - Call planner.GenerateRollback()

    // 8. Display rollback plan

    // 9. Prompt for approval (unless --auto-approve)

    // 10. Shadow DB validation (unless --skip-shadow)

    // 11. Apply rollback plan

    // 12. Report results
}
```

**Key decisions:**

1. **How to get "before" schema for rollback generation?**

   Option A: Store in forward plan (requires plan format change)
   - Pros: Guaranteed correct, no extra introspection
   - Cons: Breaking change, larger plan files

   Option B: Use source_hash to find schema file
   - Pros: No format change
   - Cons: Requires keeping schema files around

   Option C: Require --from flag
   - Pros: Explicit, no format change
   - Cons: More flags for user to provide

   **Recommendation: Option C for now**
   - Add `--from <schema|db>` flag (same as plan-rollback)
   - This makes the command signature consistent
   - Later we can optimize by embedding schema in plan

2. **Hash validation approach?**

   For now:
   - Check that forward plan has source_hash
   - Optionally compute expected "after" hash
   - Warn if target doesn't match expected state
   - Require explicit --force flag to bypass

**Modified signature:**
```bash
lockplane rollback --plan forward.json --from before.json --target-environment production
```

### Phase 4: Testing

**Unit tests:**
- Test `runPlanRollback` (renamed command)
- Test `runRollback` (new command)
- Test hash validation logic
- Test approval prompt logic

**Integration tests:**
- Test plan-rollback generates correct plan
- Test rollback applies plan correctly
- Test rollback validates hashes
- Test rollback prompts for approval
- Test rollback respects --auto-approve

**Manual testing:**
- Create a forward plan
- Apply forward plan
- Generate and apply rollback
- Verify database returns to original state

### Phase 5: Documentation

**Files to update:**

1. `printHelp()` in `main.go`:
   - Add `plan-rollback` command description
   - Update `rollback` command description
   - Add workflow examples for both

2. `README.md`:
   - Update main examples section
   - Add "Rollback Workflows" section
   - Show both plan-rollback and rollback commands

3. `docs/getting_started.md`:
   - Add rollback workflow examples
   - Show both approaches (explicit plan vs direct rollback)

4. Custom usage functions:
   - Update `runPlanRollback` usage
   - Create `runRollback` usage (similar to apply)

5. Run consistency check:
   ```bash
   ./scripts/check-docs-consistency.sh
   ```

### Phase 6: Final Verification

**Checklist:**
- [ ] All unit tests pass: `go test ./...`
- [ ] All integration tests pass
- [ ] Code formatted: `go fmt ./...`
- [ ] Code vetted: `go vet ./...`
- [ ] Linting passes: `staticcheck ./...` (if available)
- [ ] Build succeeds: `go build .`
- [ ] Documentation is consistent
- [ ] CI passes on push
- [ ] Manual smoke tests complete

## Edge Cases and Considerations

### 1. What if forward plan has no source_hash?

Old plans generated before source hash feature.

**Solution:**
- Warn user that hash validation is skipped
- Proceed with rollback
- Recommend regenerating plan with newer version

### 2. What if target database is not in expected state?

Hash validation fails.

**Solution:**
- Show detailed error message
- Show expected hash vs actual hash
- Suggest introspecting current state
- Offer --force flag (dangerous)

### 3. What if rollback requires different database driver?

Forward plan was for Postgres, but rolling back to SQLite?

**Solution:**
- Detect target database driver
- Use appropriate driver for rollback generation
- Warn if driver mismatch detected

### 4. What if forward plan has already been rolled back?

Database is already in "before" state.

**Solution:**
- Detect via hash validation
- Warn user and suggest introspecting to verify
- Offer --force to proceed anyway

### 5. Shadow DB for rollback validation

Should we validate rollbacks on shadow DB?

**Solution:**
- Yes, same as forward migrations
- Use --skip-shadow to bypass (not recommended)
- Show clear warnings when skipped

## Backward Compatibility

### Breaking Changes

1. `lockplane rollback` command changes behavior
   - Old: generates plan JSON
   - New: applies rollback to database

**Migration path:**
- Rename old command to `plan-rollback`
- Document the change clearly
- Add deprecation warning in next release?

### Non-Breaking Changes

1. All existing plans continue to work
2. `lockplane apply rollback.json` still works
3. Plan format doesn't change (initially)

## Future Enhancements

1. **Embed "before" schema in forward plans**
   - Eliminates need for --from flag
   - Makes rollback generation easier
   - Larger plan files (trade-off)

2. **Store "after" hash in forward plans**
   - Better hash validation for rollbacks
   - Can verify database state more accurately

3. **Rollback history tracking**
   - Track which plans have been applied
   - Track which plans have been rolled back
   - Prevent double-rollback

4. **Partial rollback**
   - Rollback only specific steps
   - Rollback to a specific point in time

5. **Automatic rollback on failure**
   - If forward migration fails, auto-rollback
   - Configurable behavior

## Testing Plan

### Unit Tests

1. **Test plan-rollback command:**
   - Loads forward plan correctly
   - Loads before schema correctly
   - Generates correct rollback plan
   - Outputs valid JSON

2. **Test rollback command:**
   - Parses flags correctly
   - Loads forward plan
   - Validates hashes
   - Prompts for approval
   - Applies rollback plan

### Integration Tests

1. **End-to-end rollback workflow:**
   ```go
   // Create initial state
   // Generate forward plan
   // Apply forward plan
   // Verify database changed
   // Generate and apply rollback
   // Verify database restored to original state
   ```

2. **Hash validation:**
   - Apply forward plan
   - Modify database manually
   - Attempt rollback
   - Verify hash validation fails

3. **Shadow DB validation:**
   - Generate rollback
   - Test on shadow DB
   - Verify shadow DB rollback succeeds
   - Verify shadow DB is cleaned up

### Manual Testing

1. Test both commands with help flags
2. Test plan-rollback with various inputs
3. Test rollback with approval prompt
4. Test rollback with --auto-approve
5. Test rollback with invalid hashes
6. Test rollback with shadow DB
7. Test rollback with --skip-shadow

## Open Questions

1. **Should rollback require --from flag?**
   - Yes (for now): Explicit and consistent
   - Future: Embed schema in plan to eliminate this

2. **Should we add --force flag for hash bypass?**
   - Yes: Allow expert users to bypass validation
   - Add scary warnings when used

3. **Should rollback support positional plan argument?**
   - Like apply: `lockplane rollback plan.json --target ...`
   - Yes: Consistent with apply command

4. **Should we rename to `plan-rollback` or `rollback-plan`?**
   - Preference: `plan-rollback` (verb-noun pattern)
   - Consistent with: "plan a rollback"

5. **Should hash validation be required or optional?**
   - Optional but strongly recommended
   - Warn loudly when skipped
   - Future: Consider making required

## Success Criteria

This refactoring is successful when:

1. ✅ `lockplane plan-rollback` works exactly like old `rollback`
2. ✅ `lockplane rollback` works like `apply` for rollbacks
3. ✅ All tests pass (unit, integration, manual)
4. ✅ Documentation is complete and consistent
5. ✅ CI passes
6. ✅ Backward compatibility maintained (via command rename)
7. ✅ User experience is improved (simpler rollback workflow)

## Timeline Estimate

- Phase 1 (Planning): 1 hour ✓
- Phase 2 (Rename): 1-2 hours
- Phase 3 (Implement): 3-4 hours
- Phase 4 (Testing): 2-3 hours
- Phase 5 (Documentation): 1-2 hours
- Phase 6 (Verification): 1 hour

**Total: 9-13 hours**

## Notes

- This is a significant refactoring that touches many files
- Take care to update all documentation consistently
- Test thoroughly before pushing to main
- Consider beta testing with users before release
