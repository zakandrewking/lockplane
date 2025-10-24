#!/bin/bash
# Check that documentation is consistent across all files

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🔍 Checking documentation consistency..."
echo ""

ISSUES=0

# Check that lockplane binary exists
if [ ! -f "$PROJECT_ROOT/lockplane" ]; then
    echo "⚠️  lockplane binary not found. Run 'go build .' first."
    exit 1
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track issues
report_issue() {
    echo -e "${RED}❌ $1${NC}"
    ISSUES=$((ISSUES + 1))
}

report_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

report_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

# 1. Check that all main commands are documented
echo "📋 Checking main commands are documented..."

COMMANDS="introspect diff plan apply rollback convert validate version help"

for cmd in $COMMANDS; do
    # Check README
    if ! grep -q "lockplane $cmd" "$PROJECT_ROOT/README.md"; then
        report_issue "Command '$cmd' not mentioned in README.md"
    fi

    # Check llms.txt
    if ! grep -q "lockplane $cmd" "$PROJECT_ROOT/llms.txt"; then
        report_issue "Command '$cmd' not mentioned in llms.txt"
    fi

    # Check Claude skill
    if ! grep -q "lockplane $cmd" "$PROJECT_ROOT/.claude/skills/lockplane.md"; then
        report_issue "Command '$cmd' not mentioned in .claude/skills/lockplane.md"
    fi
done

report_success "Main commands checked"
echo ""

# 2. Check that validate subcommands are documented
echo "📋 Checking validate subcommands..."

VALIDATE_CMDS="schema sql plan"

for subcmd in $VALIDATE_CMDS; do
    if ! grep -q "validate $subcmd" "$PROJECT_ROOT/README.md"; then
        report_issue "Subcommand 'validate $subcmd' not mentioned in README.md"
    fi

    if ! grep -q "validate $subcmd" "$PROJECT_ROOT/llms.txt"; then
        report_issue "Subcommand 'validate $subcmd' not mentioned in llms.txt"
    fi
done

report_success "Validate subcommands checked"
echo ""

# 3. Check that dangerous pattern error codes are documented
echo "🚨 Checking dangerous pattern codes are documented..."

# Extract codes from validate_sql_safety.go
if [ -f "$PROJECT_ROOT/validate_sql_safety.go" ]; then
    CODES=$(grep -o 'Code:.*"dangerous_[^"]*"' "$PROJECT_ROOT/validate_sql_safety.go" | sed 's/.*"dangerous_/dangerous_/' | sed 's/".*//' | sort -u)

    for code in $CODES; do
        # Check if documented in README
        if ! grep -qi "$code\|DROP TABLE\|DROP COLUMN\|TRUNCATE\|DELETE.*WHERE" "$PROJECT_ROOT/README.md"; then
            report_warning "Error code '$code' not clearly documented in README.md"
        fi
    done
fi

report_success "Dangerous patterns checked"
echo ""

# 4. Check that config file format is consistent
echo "⚙️  Checking config file examples..."

# Extract config from README
README_CONFIG=$(grep -A 10 "database_url.*=" "$PROJECT_ROOT/README.md" | head -3 | grep -v "^$" || true)
LLMS_CONFIG=$(grep -A 10 "database_url.*=" "$PROJECT_ROOT/llms.txt" | head -3 | grep -v "^$" || true)

if [ -z "$README_CONFIG" ]; then
    report_warning "No config example found in README.md"
elif [ -z "$LLMS_CONFIG" ]; then
    report_warning "No config example found in llms.txt"
else
    report_success "Config examples present"
fi

echo ""

# 5. Check that example commands use correct syntax
echo "💻 Checking example command syntax..."

# Common mistakes to check for (excluding troubleshooting examples)
# Look for usage outside of "not" context
if grep "lockplane validate-sql" "$PROJECT_ROOT/README.md" "$PROJECT_ROOT/llms.txt" "$PROJECT_ROOT/.claude/skills/lockplane.md" 2>/dev/null | grep -v "not.*validate-sql" | grep -v "validate-sql.*not" | grep -q .; then
    report_issue "Found 'validate-sql' used incorrectly (should be 'validate sql' with space)"
fi

if grep "lockplane validate-schema" "$PROJECT_ROOT/README.md" "$PROJECT_ROOT/llms.txt" "$PROJECT_ROOT/.claude/skills/lockplane.md" 2>/dev/null | grep -v "not.*validate-schema" | grep -v "validate-schema.*not" | grep -q .; then
    report_issue "Found 'validate-schema' used incorrectly (should be 'validate schema' with space)"
fi

report_success "Command syntax checked"
echo ""

# 6. Verify CLI help matches documentation
echo "📖 Checking CLI help text..."

# Get help output
HELP_OUTPUT=$("$PROJECT_ROOT/lockplane" --help 2>&1 || true)

# Check that documented commands appear in help
for cmd in introspect plan apply validate convert; do
    if ! echo "$HELP_OUTPUT" | grep -q "$cmd"; then
        report_issue "Command '$cmd' not in CLI help output"
    fi
done

report_success "CLI help text checked"
echo ""

# 7. Check for version consistency
echo "📦 Checking version references..."

# This is informational only - version may vary
if grep -q "version.*=" "$PROJECT_ROOT/main.go"; then
    report_success "Version variable found in main.go"
else
    report_warning "No version variable in main.go"
fi

echo ""

# Final report
echo "═════════════════════════════════════"
if [ $ISSUES -eq 0 ]; then
    echo -e "${GREEN}✅ All consistency checks passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Found $ISSUES issue(s)${NC}"
    echo ""
    echo "See docs/documentation-maintenance.md for update procedures"
    exit 1
fi
