#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Validation script for todo app scenario.

Validates that:
- Lockplane is installed
- Schema files exist in the correct format
- No migration files exist (schema as source of truth)
- Database exists and matches schema
- App structure is present
"""

import json
import subprocess
import sys
from pathlib import Path


class Validator:
    """Validation runner with nice output."""

    def __init__(self):
        self.failures = 0

    def check(self, name: str, condition: bool, error_msg: str = ""):
        """Run a validation check."""
        if condition:
            print(f"✓ {name}")
            return True
        else:
            print(f"✗ {name}", file=sys.stderr)
            if error_msg:
                print(f"  {error_msg}", file=sys.stderr)
            self.failures += 1
            return False

    def check_command(self, name: str, cmd: list[str], check_output: str = None):
        """Run a command and check if it succeeds."""
        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                check=False,
            )
            success = result.returncode == 0

            if check_output and success:
                success = check_output in result.stdout

            return self.check(name, success, result.stderr if not success else "")
        except FileNotFoundError:
            return self.check(name, False, f"Command not found: {cmd[0]}")

    def has_failures(self) -> bool:
        """Return True if any checks failed."""
        return self.failures > 0

    def summary(self):
        """Print summary."""
        print()
        if self.failures == 0:
            print("✅ All validations passed")
            return 0
        else:
            print(f"❌ {self.failures} validation(s) failed", file=sys.stderr)
            return 1


def main():
    """Run all validations."""
    validator = Validator()

    # Get paths
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    if not build_dir.exists():
        print("❌ Build directory not found. Run scenario first.", file=sys.stderr)
        return 1

    print("=== Validating todo app scenario ===")

    # 1. Check lockplane is installed
    validator.check_command("Lockplane is installed", ["which", "lockplane"])

    # 2. Check for schema directory
    schema_dir = None
    for possible_schema in [build_dir / "schema", build_dir / "todo-app" / "schema"]:
        if possible_schema.exists():
            schema_dir = possible_schema
            break

    validator.check("schema/ directory exists", schema_dir is not None)

    # 3. Check for .lp.sql files
    if schema_dir:
        lp_sql_files = list(schema_dir.glob("*.lp.sql"))
        validator.check(
            "schema/ contains .lp.sql files",
            len(lp_sql_files) > 0,
            f"Found {len(lp_sql_files)} .lp.sql files",
        )

        # 4. Check no migration or plan files
        migration_files = list(schema_dir.glob("*migration*")) + list(schema_dir.glob("*plan*"))
        validator.check(
            "No migration/plan files in schema/",
            len(migration_files) == 0,
            f"Found unwanted files: {[f.name for f in migration_files]}",
        )

    # 5. Check for SQLite database
    db_files = list(build_dir.rglob("*.db")) + list(build_dir.rglob("*.sqlite"))
    validator.check(
        "SQLite database exists",
        len(db_files) > 0,
        f"Found {len(db_files)} database files",
    )

    # 6. Check database schema (if db exists)
    if db_files:
        db_path = db_files[0]
        try:
            result = subprocess.run(
                ["lockplane", "introspect", "--db", f"sqlite://{db_path}"],
                capture_output=True,
                text=True,
                check=True,
            )
            schema = json.loads(result.stdout)

            # Check for todos-related table
            has_todos_table = any(
                "todo" in table.get("name", "").lower()
                for table in schema.get("tables", [])
            )

            validator.check(
                "Database contains todos table",
                has_todos_table,
                "No todos-related table found in schema",
            )
        except (subprocess.CalledProcessError, json.JSONDecodeError) as e:
            validator.check("Database introspection", False, str(e))

    # 7. Check app structure
    app_dirs = [
        build_dir / "todo-app",
        build_dir / "app",
        build_dir / "src",
    ]
    app_dir = next((d for d in app_dirs if d.exists()), None)
    validator.check("App directory exists", app_dir is not None)

    # 8. Check for lockplane usage (grep for lockplane or schema references)
    if app_dir:
        try:
            result = subprocess.run(
                ["grep", "-r", "-i", "lockplane", str(app_dir)],
                capture_output=True,
                text=True,
            )
            # grep returns 0 if found, 1 if not found
            found_lockplane = result.returncode == 0

            validator.check(
                "App references Lockplane",
                found_lockplane,
                "No Lockplane references found in app code",
            )
        except Exception as e:
            validator.check("App code search", False, str(e))

    return validator.summary()


if __name__ == "__main__":
    sys.exit(main())
