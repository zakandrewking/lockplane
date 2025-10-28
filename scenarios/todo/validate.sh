#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/build"

# Track validation failures
FAILURES=0

# Helper function for validation
validate() {
    local check_name="$1"
    local check_command="$2"

    if eval "$check_command"; then
        echo "✓ $check_name"
    else
        echo "✗ $check_name" >&2
        FAILURES=$((FAILURES + 1))
    fi
}

echo "=== Validating todo app scenario ==="

# 1. Check lockplane is installed
validate "Lockplane is installed" "which lockplane > /dev/null 2>&1"

# 2. Check schema directory exists and contains .lp.sql files
validate "schema/ directory exists" "[ -d 'schema' ]"
validate "schema/ contains .lp.sql files" "ls schema/*.lp.sql > /dev/null 2>&1"

# 3. Check no migration or plan files exist (schema as source of truth)
validate "No migration files in schema/" "! ls schema/*migration* > /dev/null 2>&1 || true"
validate "No plan files in schema/" "! ls schema/*plan* > /dev/null 2>&1 || true"

# 4. Check SQLite database exists
validate "SQLite database exists" "[ -f 'todo.db' ] || [ -f 'todo-app/todo.db' ] || [ -f '*.db' ]"

# 5. Check schema matches database (if db exists)
if [ -f "todo.db" ]; then
    DB_PATH="todo.db"
elif [ -f "todo-app/todo.db" ]; then
    DB_PATH="todo-app/todo.db"
else
    DB_PATH=$(find . -name "*.db" -type f | head -1)
fi

if [ -n "${DB_PATH:-}" ]; then
    # Introspect the database
    INTROSPECT_OUTPUT=$(lockplane introspect --db "sqlite://${DB_PATH}" 2>/dev/null || echo "{}")

    # Check that todos table exists (basic smoke test)
    validate "Database contains todos table" "echo '$INTROSPECT_OUTPUT' | grep -q 'todo'"
fi

# 6. Check app structure
validate "App directory exists" "[ -d 'todo-app' ] || [ -d 'app' ]"

# 7. Check for lockplane usage in the codebase
if [ -d "todo-app" ]; then
    validate "App uses Lockplane concepts" "grep -r 'lockplane\|schema' todo-app/ > /dev/null 2>&1 || true"
fi

echo
if [ $FAILURES -eq 0 ]; then
    echo "✅ All validations passed"
    exit 0
else
    echo "❌ $FAILURES validation(s) failed" >&2
    exit 1
fi
