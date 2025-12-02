# Instructions

- [ ] Always implement and run tests for new code
- [ ] Always create changes in a new branch and push a pull request using the
  GitHub CLI (`gh pr create`) when ready for review (unless already working on a
  branch).
- [ ] After pushing commits, always use `gh` CLI to check workflow status and debug any failures:
  - Check PR status: `gh pr view <number> --json statusCheckRollup`
  - View failed logs: `gh run view <run-id> --log-failed`
  - Check latest run: `gh run list --branch <branch> --limit 1`
  - Keep debugging and fixing issues until all workflows pass
