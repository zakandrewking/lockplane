# Publishing Lockplane to npm

This guide explains how to publish lockplane to npm so users can run `npx lockplane`.

## Prerequisites

1. **npm account**: Create one at https://www.npmjs.com/signup
2. **npm CLI**: Login with `npm login`
3. **GitHub Release**: Must have a corresponding GitHub release with binaries

## How it Works

The npm package doesn't contain the Go binary. Instead:

1. When users run `npm install lockplane` or `npx lockplane`, the postinstall script runs
2. The script downloads the appropriate pre-built binary from GitHub releases
3. The binary is extracted to `bin/` directory
4. The `bin/lockplane.js` wrapper executes the binary

This keeps the npm package small (~10KB) and works across all platforms.

## Publishing Steps

### 1. Create a GitHub Release

First, ensure you have a GitHub release with binaries:

```bash
# Tag and push
git tag v0.1.0
git push origin v0.1.0

# GoReleaser will automatically create the release with binaries
# (if you have GitHub Actions set up)

# Or manually with GoReleaser:
goreleaser release --clean
```

Verify the release has binaries at:
https://github.com/lockplane/lockplane/releases/tag/v0.1.0

### 2. Update package.json Version

Make sure `package.json` version matches the Git tag:

```json
{
  "version": "0.1.0"
}
```

### 3. Test Locally

Test the package installation locally:

```bash
# Pack the npm package
npm pack

# This creates lockplane-0.1.0.tgz

# Test in another directory
mkdir test-install
cd test-install
npm install ../lockplane-0.1.0.tgz

# Test it works
npx lockplane --help
```

### 4. Publish to npm

```bash
# Login to npm (first time only)
npm login

# Publish
npm publish

# For first publish, you might need --access public
npm publish --access public
```

### 5. Verify

```bash
# Test installation
npx lockplane@0.1.0 --help

# Or install globally
npm install -g lockplane
lockplane --help
```

## Version Management

The version in `package.json` must match the GitHub release tag (without the `v` prefix).

**Example:**
- Git tag: `v0.1.0`
- package.json: `"version": "0.1.0"`
- GitHub release: `https://github.com/lockplane/lockplane/releases/tag/v0.1.0`
- Binaries: `lockplane_0.1.0_Linux_x86_64.tar.gz`, etc.

## Automated Publishing (Trusted Publishers / OIDC)

Publishing is automated in `.github/workflows/release.yml` using npm's Trusted Publisher support. To authorize GitHub Actions to publish on your behalf:

1. **Create a GitHub Environment** named `npm`
   - In your repository settings, add an environment called `npm` (the workflow already references it).
   - Optionally require reviewers so releases need approval before publishing.

2. **Configure npm Trusted Publisher**
   - Log in to [npmjs.com](https://www.npmjs.com/) → _Access Tokens_ → _Trusted publishers_.
   - Add a new Trusted Publisher pointing to this GitHub repo and the `npm` environment.
   - npm will now accept OIDC tokens issued for that environment.

3. **Workflow requirements** (already satisfied in `release.yml`):

```yaml
permissions:
  contents: read
  id-token: write

jobs:
  npm-publish:
    environment: npm
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v5
        with:
          node-version: '22'
          registry-url: 'https://registry.npmjs.org'
      - run: npm publish --provenance --access public
```

No `NPM_TOKEN` secret is required—`setup-node` exchanges the GitHub OIDC token for a short-lived npm token automatically. If you need to force provenance on every publish, you can also set `NPM_CONFIG_PROVENANCE=true`, but the workflow already passes `--provenance`.

> ℹ️ You never set `NODE_AUTH_TOKEN` or `NPM_TOKEN` manually when using Trusted Publishers; npm injects those after validating the GitHub OIDC token.

## Troubleshooting

### Binary not found after install

The postinstall script failed. Check:
- Does the GitHub release exist?
- Are the binary names correct?
- Does the version match?

Users can manually download from GitHub releases as a fallback.

### Platform not supported

Check `scripts/install.js` supports the platform:
- Linux x64/arm64
- macOS (Darwin) x64/arm64
- Windows x64

### Permission denied

On Unix systems, ensure binary is executable:
```bash
chmod +x bin/lockplane
```

The postinstall script handles this automatically.

## Package Scope

Currently published as `lockplane` (unscoped).

If you want scoped package (e.g., `@lockplane/cli`):
1. Update `package.json` name to `"@lockplane/cli"`
2. Publish with `npm publish --access public`

## Useful Commands

```bash
# Check package contents
npm pack --dry-run

# View package info
npm view lockplane

# Deprecate a version
npm deprecate lockplane@0.1.0 "Please upgrade to 0.2.0"

# Unpublish (within 72 hours)
npm unpublish lockplane@0.1.0
```

## Support

Users can also install via:

- **Homebrew** (when tap is set up): `brew install lockplane/tap/lockplane`
- **Direct download**: GitHub releases page
- **Go install**: `go install github.com/lockplane/lockplane@latest`
