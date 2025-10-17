# Release Process

This document describes how to create a new release of Lockplane.

## Prerequisites

- Push access to the main repository
- GitHub CLI (`gh`) installed (optional, but helpful)
- Clean git working directory

## Automated Release Process

Lockplane uses GitHub Actions and GoReleaser to automate the entire release process.

### Create a New Release

1. **Ensure all changes are merged to main:**
   ```bash
   git checkout main
   git pull origin main
   ```

2. **Verify tests pass:**
   ```bash
   go test ./...
   ```

3. **Create and push a version tag:**
   ```bash
   # Determine next version (following semver)
   # Major version: Breaking changes
   # Minor version: New features
   # Patch version: Bug fixes

   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

4. **GitHub Actions will automatically:**
   - Run all tests
   - Build binaries for:
     - linux/amd64
     - linux/arm64
     - darwin/amd64 (macOS Intel)
     - darwin/arm64 (macOS Apple Silicon)
     - windows/amd64
   - Create GitHub release with:
     - Binaries
     - Checksums
     - Auto-generated changelog
   - Build and push Docker images (optional)
   - Update Homebrew tap (optional)

5. **Monitor the release:**
   - Check GitHub Actions: https://github.com/lockplane/lockplane/actions
   - Once complete, verify the release: https://github.com/lockplane/lockplane/releases

## Manual Release Testing

Before creating a production release, test the release process:

1. **Test goreleaser locally:**
   ```bash
   # Dry run (doesn't create release)
   goreleaser release --snapshot --clean

   # Check dist/ directory for built binaries
   ls -lh dist/
   ```

2. **Test installation script:**
   ```bash
   # Test locally (will use dev version)
   bash install.sh
   ```

3. **Test binary:**
   ```bash
   ./dist/lockplane_linux_amd64_v1/lockplane version
   ./dist/lockplane_linux_amd64_v1/lockplane help
   ```

## Version Numbering

Follow [Semantic Versioning](https://semver.org/):

- **v0.x.x** - Pre-1.0 releases (current phase)
- **v1.0.0** - First stable release
- **MAJOR.MINOR.PATCH** format

Examples:
- `v0.1.0` - Initial release with plan generator
- `v0.2.0` - Added rollback generation
- `v0.2.1` - Bug fix for rollback generation
- `v1.0.0` - First production-ready release

## Release Checklist

Before tagging a release:

- [ ] All tests pass (`go test ./...`)
- [ ] Documentation is up to date
- [ ] CHANGELOG.md is updated (if using manual changelog)
- [ ] Version number follows semver
- [ ] No uncommitted changes
- [ ] On main branch
- [ ] Pulled latest changes

## Troubleshooting

### Release Failed

If the GitHub Action fails:

1. Check the action logs for errors
2. Fix the issue
3. Delete the tag: `git tag -d v0.1.0 && git push origin :v0.1.0`
4. Create a new tag with incremented version

### Binary Not Working

If a released binary doesn't work:

1. Test with `goreleaser release --snapshot`
2. Check the binary on the target platform
3. Verify ldflags are correct
4. Create a patch release

### Update Existing Release

To update release notes or assets:

1. Go to https://github.com/lockplane/lockplane/releases
2. Edit the release
3. Upload new assets or update description
4. Save changes

## Post-Release

After a successful release:

1. **Announce the release:**
   - Update README if needed
   - Post in discussions/community channels
   - Update documentation site (if exists)

2. **Verify installation methods:**
   ```bash
   # Test installation script
   curl -sSL https://raw.githubusercontent.com/lockplane/lockplane/main/install.sh | bash

   # Verify version
   lockplane version
   ```

3. **Update any dependent projects or examples**

## GoReleaser Configuration

The release process is configured in `.goreleaser.yml`. Key sections:

- **builds**: Defines which platforms to build for
- **archives**: How to package the binaries
- **checksum**: Generates checksums for verification
- **release**: GitHub release configuration
- **brews**: Homebrew formula (optional)
- **dockers**: Docker image builds (optional)

To modify the release process, edit `.goreleaser.yml` and test with:
```bash
goreleaser check
goreleaser release --snapshot --clean
```

## Hotfix Process

For urgent bug fixes:

1. Create a hotfix branch from the release tag:
   ```bash
   git checkout -b hotfix/v0.1.1 v0.1.0
   ```

2. Make and commit the fix
3. Tag the hotfix:
   ```bash
   git tag -a v0.1.1 -m "Hotfix: Fix critical bug"
   ```

4. Push tag to trigger release:
   ```bash
   git push origin v0.1.1
   ```

5. Merge hotfix back to main:
   ```bash
   git checkout main
   git merge hotfix/v0.1.1
   git push origin main
   ```
