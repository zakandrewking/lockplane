# Development Workflow

## IMPORTANT: Never push directly to main!

**Always create a Pull Request** for any code changes, even small fixes.

## Checklist for Every Change

### 1. Create a Branch
- [ ] Create a new branch: `git checkout -b descriptive-branch-name`
- [ ] Make your changes on this branch

### 2. Implement and Test
- [ ] Write tests for new code
- [ ] Run tests: `go test -v ./...`
- [ ] Verify all tests pass
- [ ] Run validation tools:
  - `go fmt ./...`
  - `go vet ./...`
  - `errcheck ./...`
  - `staticcheck ./...`

### 3. Commit Changes
- [ ] Commit with descriptive message
- [ ] Push branch: `git push -u origin branch-name`

### 4. Create Pull Request
- [ ] **ALWAYS** create PR: `gh pr create --web`
- [ ] Fill in PR description with:
  - What changed
  - Why it changed
  - Test coverage
- [ ] **NEVER** push directly to main (even for small fixes)

### 5. Monitor CI and Fix Issues
- [ ] Check PR status: `gh pr view <number> --json statusCheckRollup`
- [ ] If tests fail, view logs: `gh run view <run-id> --log-failed`
- [ ] Fix issues and push updates to the same branch
- [ ] Keep iterating until all workflows pass

### 6. Merge
- [ ] Wait for approval (if required)
- [ ] Merge PR only after CI passes

## Exceptions

The **ONLY** time you can push directly to main is when the user **explicitly** says:
- "push this to main"
- "commit directly to main"
- "skip the PR"

Otherwise, **ALWAYS CREATE A PULL REQUEST**.
