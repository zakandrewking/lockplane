# Lockplane Evaluation Scenarios

This directory contains end-to-end evaluation scenarios for testing Lockplane with AI assistants like Claude Code.

## Prerequisites

- Python 3.11+
- [uv](https://github.com/astral-sh/uv) - Fast Python package installer
- Lockplane CLI

Install uv:
```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

## Structure

Each scenario is a subdirectory containing:

- `scenario.py` - Python script that runs the scenario (with inline dependencies via uv)
- `validate.py` - Python script that validates expected outcomes
- `scenario.yaml` - Metadata (name, description, timeout, tags)
- `README.md` - (optional) Human-readable description
- `expected/` - (optional) Expected output files for comparison

## Running Scenarios

**⚠️ Important**: You must specify a scenario name. Running all scenarios is disabled to avoid unnecessary costs.

### Run a scenario

```bash
./run-evals.py <scenario-name>
```

### Examples

```bash
# Run plugin installation test
./run-evals.py plugin-install

# Run plugin access test
./run-evals.py plugin-access

# Run with verbose output
./run-evals.py plugin-install --verbose

# Generate JSON report
./run-evals.py plugin-install --format json > results.json
```

### List available scenarios

```bash
./run-evals.py nonexistent
# Will show error and list all available scenarios
```

## Creating a New Scenario

1. Create a new directory under `scenarios/`:

```bash
mkdir scenarios/my-scenario
```

2. Create `scenario.yaml`:

```yaml
name: my-scenario
description: Short description of what this tests
timeout: 300  # seconds
tags:
  - cli
  - migrations
```

3. Create `scenario.py`:

```python
#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "requests>=2.31.0",  # Add any dependencies here
# ]
# ///

"""
My scenario description.
"""

import sys
from pathlib import Path

def main():
    """Run the scenario."""
    # Your scenario implementation
    print("Running scenario...")
    return 0  # 0 = success, non-zero = failure

if __name__ == "__main__":
    sys.exit(main())
```

4. Create `validate.py`:

```python
#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Validation for my scenario.
"""

import sys
from pathlib import Path

def main():
    """Validate the scenario results."""
    failures = 0

    # Your validation logic
    print("✓ Check passed")
    # Or: print("✗ Check failed", file=sys.stderr); failures += 1

    if failures == 0:
        print("✅ All validations passed")
        return 0
    else:
        print(f"❌ {failures} validation(s) failed", file=sys.stderr)
        return 1

if __name__ == "__main__":
    sys.exit(main())
```

5. Make scripts executable:

```bash
chmod +x scenarios/my-scenario/*.py
```

6. (Optional) Create `README.md` documenting the scenario.

## Scenario Best Practices

- **Isolation**: Scenarios should clean up their own state
- **Deterministic**: Scenarios should produce consistent results
- **Fast**: Keep scenarios under 5 minutes when possible
- **Clear validation**: Validation should clearly report what failed
- **Documentation**: Explain what the scenario tests and why
- **Dependencies**: Use uv's inline script metadata for Python dependencies

## Why Python + uv?

- **Inline dependencies**: Specify dependencies at the top of each script
- **No virtual env management**: uv handles everything automatically
- **Fast**: uv is extremely fast at resolving and installing packages
- **Reproducible**: Dependencies are explicit and version-locked
- **Better error handling**: Python's exception handling is clearer than bash

## Validation Patterns

### File existence

```python
from pathlib import Path

if not Path("expected-file.txt").exists():
    print("✗ expected-file.txt not found", file=sys.stderr)
    failures += 1
```

### File content

```python
content = Path("file.txt").read_text()
if "expected pattern" not in content:
    print("✗ file.txt missing expected pattern", file=sys.stderr)
    failures += 1
```

### Command output

```python
import subprocess

result = subprocess.run(["lockplane", "version"], capture_output=True, text=True)
if "lockplane version" not in result.stdout:
    print("✗ Unexpected lockplane version output", file=sys.stderr)
    failures += 1
```

### JSON validation

```python
import json

with open("output.json") as f:
    data = json.load(f)

if "expected_key" not in data:
    print("✗ Missing expected key in output", file=sys.stderr)
    failures += 1
```

### Database state

```python
import subprocess
import json

result = subprocess.run(
    ["lockplane", "introspect", "--db", db_url],
    capture_output=True, text=True, check=True
)
schema = json.loads(result.stdout)

has_users_table = any(t["name"] == "users" for t in schema.get("tables", []))
if not has_users_table:
    print("✗ users table not found", file=sys.stderr)
    failures += 1
```

## Available Scenarios

### `plugin-install` 🚨 **TDD - Expected to FAIL**
**Type**: TDD / Integration test
**Duration**: < 2 minutes
**Prerequisites**: Claude Code CLI

Tests that Claude Code automatically installs the Lockplane plugin when given the GitHub repository link. This test is designed to fail until the plugin system is fully configured and Claude can discover and install plugins automatically.

**Expected failures:**
- Lockplane plugin is not auto-installed
- Lockplane skill is not available after providing GitHub link
- Claude does not recognize it needs the plugin

**Success criteria (when passing):**
- Claude recognizes Lockplane from GitHub link
- Claude runs `/plugin install` automatically
- Plugin installation completes successfully
- Lockplane skill becomes available

### `plugin-access`
**Type**: Integration test
**Duration**: < 1 minute
**Prerequisites**: Claude Code CLI

Tests that Claude Code can access and use an installed Lockplane plugin. Verifies the complete plugin workflow: explicit install → access → use skills. The scenario manually installs the plugin in an isolated environment and validates that Claude provides Lockplane-specific guidance.

**Success criteria:**
- Plugin files installed correctly in isolated environment
- Claude can access the installed plugin
- Claude uses Lockplane skill to provide expert guidance
- Response includes Lockplane commands and safety concepts

## Continuous Integration

Scenarios can run automatically on:
- Push to main
- Pull requests
- Scheduled nightly builds

Add a GitHub Actions workflow at `.github/workflows/evals.yml`:

```yaml
name: Run Evaluations

on: [push, pull_request]

jobs:
  evals:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: lockplane
          POSTGRES_USER: lockplane
          POSTGRES_DB: lockplane
        ports:
          - 5432:5432
          - 5433:5433

    steps:
      - uses: actions/checkout@v4

      - name: Install uv
        run: curl -LsSf https://astral.sh/uv/install.sh | sh

      - name: Install Lockplane
        run: |
          # Download and install lockplane
          # Or build from source

      - name: Run evaluations
        run: scenarios/run-evals.py --format json
```
