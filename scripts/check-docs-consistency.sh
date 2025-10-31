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

# 1. Check that commands are documented appropriately per file
echo "📋 Checking commands are documented..."

# README should have main workflow commands (simplified docs focus on basic workflow)
README_COMMANDS="introspect plan apply rollback convert validate version"
for cmd in $README_COMMANDS; do
    if ! grep -q "lockplane $cmd" "$PROJECT_ROOT/README.md"; then
        report_issue "Command '$cmd' not mentioned in README.md"
    fi
done

# llms.txt and SKILL.md should have the simplified workflow (validate + apply only)
SIMPLE_COMMANDS="validate apply"
for cmd in $SIMPLE_COMMANDS; do
    if ! grep -q "lockplane $cmd" "$PROJECT_ROOT/llms.txt"; then
        report_issue "Command '$cmd' not mentioned in llms.txt"
    fi

    if [ -f "$PROJECT_ROOT/claude-plugin/skills/lockplane/SKILL.md" ]; then
        if ! grep -q "lockplane $cmd" "$PROJECT_ROOT/claude-plugin/skills/lockplane/SKILL.md"; then
            report_issue "Command '$cmd' not mentioned in claude-plugin/skills/lockplane/SKILL.md"
        fi
    fi
done

# Verify that advanced commands (introspect, plan, etc.) are NOT in skill files
ADVANCED_COMMANDS="introspect plan rollback convert diff"
for cmd in $ADVANCED_COMMANDS; do
    if grep -q "lockplane $cmd" "$PROJECT_ROOT/llms.txt" 2>/dev/null; then
        report_warning "Advanced command '$cmd' found in llms.txt (should be removed for simplicity)"
    fi

    if [ -f "$PROJECT_ROOT/claude-plugin/skills/lockplane/SKILL.md" ]; then
        if grep -q "lockplane $cmd" "$PROJECT_ROOT/claude-plugin/skills/lockplane/SKILL.md" 2>/dev/null; then
            report_warning "Advanced command '$cmd' found in SKILL.md (should be removed for simplicity)"
        fi
    fi
done

report_success "Commands checked"
echo ""

# 2. Check that validate subcommands are documented
echo "📋 Checking validate subcommands..."

# README has all validate subcommands
README_VALIDATE_CMDS="schema sql plan"
for subcmd in $README_VALIDATE_CMDS; do
    if ! grep -q "validate $subcmd" "$PROJECT_ROOT/README.md"; then
        report_issue "Subcommand 'validate $subcmd' not mentioned in README.md"
    fi
done

# llms.txt and SKILL.md only need validate sql
if ! grep -q "validate sql" "$PROJECT_ROOT/llms.txt"; then
    report_issue "Subcommand 'validate sql' not mentioned in llms.txt"
fi

if [ -f "$PROJECT_ROOT/claude-plugin/skills/lockplane/SKILL.md" ]; then
    if ! grep -q "validate sql" "$PROJECT_ROOT/claude-plugin/skills/lockplane/SKILL.md"; then
        report_issue "Subcommand 'validate sql' not mentioned in SKILL.md"
    fi
fi

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

# Check for config-related content
README_HAS_CONFIG=$(grep -qi "DATABASE_URL\|database_url\|Configuration" "$PROJECT_ROOT/README.md" && echo "yes" || echo "no")
LLMS_HAS_CONFIG=$(grep -qi "DATABASE_URL\|database_url\|Configuration" "$PROJECT_ROOT/llms.txt" && echo "yes" || echo "no")

if [ "$README_HAS_CONFIG" = "no" ]; then
    report_warning "No config example found in README.md"
elif [ "$LLMS_HAS_CONFIG" = "no" ]; then
    report_warning "No config example found in llms.txt"
else
    report_success "Config examples present"
fi

echo ""

# 5. Check that example commands use correct syntax
echo "💻 Checking example command syntax..."

# Common mistakes to check for (excluding troubleshooting examples)
SKILL_FILE="$PROJECT_ROOT/claude-plugin/skills/lockplane/SKILL.md"
FILES_TO_CHECK="$PROJECT_ROOT/README.md $PROJECT_ROOT/llms.txt"
if [ -f "$SKILL_FILE" ]; then
    FILES_TO_CHECK="$FILES_TO_CHECK $SKILL_FILE"
fi

if echo "$FILES_TO_CHECK" | xargs grep "lockplane validate-sql" 2>/dev/null | grep -v "not.*validate-sql" | grep -v "validate-sql.*not" | grep -q .; then
    report_issue "Found 'validate-sql' used incorrectly (should be 'validate sql' with space)"
fi

if echo "$FILES_TO_CHECK" | xargs grep "lockplane validate-schema" 2>/dev/null | grep -v "not.*validate-schema" | grep -v "validate-schema.*not" | grep -q .; then
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
    echo "See devdocs/documentation-maintenance.md for update procedures"
    exit 1
fi
