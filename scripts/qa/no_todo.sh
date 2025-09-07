#!/bin/bash
set -euo pipefail

# No-TODO QA Gate Scanner
# Fails builds if TODO/FIXME/STUB/XXX markers are found in tracked source code

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ALLOW_FILE="${SCRIPT_DIR}/no_todo.allow"
REPORT_FILE="${SCRIPT_DIR}/../../no_todo_report.txt"

# Patterns to search for (case-insensitive)
PATTERNS="TODO|FIXME|XXX|STUB|PENDING"

# Default exclusions (can be overridden by .allow file)
EXCLUDE_PATTERNS=(
    "vendor/**"
    "third_party/**"
    "*.pb.go"
    "*.gen.go"
    "generated/**"
    "artifacts/**"
    "out/**"
    ".git/**"
    "node_modules/**"
    "scripts/qa/no_todo.sh"  # Don't scan this script
)

echo "🔍 Running No-TODO QA Gate Scanner..."
echo "Searching for patterns: ${PATTERNS}"

# Function to check if file matches any exclusion pattern
should_exclude() {
    local file="$1"
    
    # Check built-in exclusions
    for pattern in "${EXCLUDE_PATTERNS[@]}"; do
        if [[ "$file" == *"${pattern#*/}"* ]] || [[ "$file" == "${pattern}" ]]; then
            return 0
        fi
    done
    
    # Check allow file if it exists
    if [[ -f "$ALLOW_FILE" ]]; then
        while IFS= read -r pattern; do
            [[ -z "$pattern" || "$pattern" =~ ^[[:space:]]*# ]] && continue
            if [[ "$file" == *"$pattern"* ]]; then
                return 0
            fi
        done < "$ALLOW_FILE"
    fi
    
    return 1
}

# Find all tracked files (excluding binary files)
echo "Scanning tracked source files..."

# Initialize report
echo "No-TODO QA Gate Report - $(date)" > "$REPORT_FILE"
echo "=================================" >> "$REPORT_FILE"
echo >> "$REPORT_FILE"

found_issues=0
total_files=0

# Use git to get tracked files, or fallback to find if not a git repo
if git rev-parse --git-dir > /dev/null 2>&1; then
    file_list=$(git ls-files)
else
    file_list=$(find . -type f \( -name "*.go" -o -name "*.js" -o -name "*.ts" -o -name "*.py" -o -name "*.java" -o -name "*.c" -o -name "*.cpp" -o -name "*.h" -o -name "*.hpp" -o -name "*.rs" \) | sed 's|^\./||')
fi

while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    
    # Skip if file doesn't exist or is not a regular file
    [[ ! -f "$file" ]] && continue
    
    # Skip binary files
    if file "$file" 2>/dev/null | grep -q "binary"; then
        continue
    fi
    
    # Check if file should be excluded
    if should_exclude "$file"; then
        continue
    fi
    
    ((total_files++))
    
    # Search for patterns in the file (case-insensitive)
    if grep -i -n -E "$PATTERNS" "$file" 2>/dev/null; then
        echo >> "$REPORT_FILE"
        echo "❌ $file:" >> "$REPORT_FILE"
        grep -i -n -E "$PATTERNS" "$file" >> "$REPORT_FILE" 2>/dev/null || true
        ((found_issues++))
    fi
done <<< "$file_list"

echo >> "$REPORT_FILE"
echo "Summary:" >> "$REPORT_FILE"
echo "- Files scanned: $total_files" >> "$REPORT_FILE"
echo "- Files with issues: $found_issues" >> "$REPORT_FILE"

if [[ $found_issues -gt 0 ]]; then
    echo >> "$REPORT_FILE"
    echo "❌ QA Gate FAILED: Found TODO/FIXME/STUB markers in $found_issues file(s)" >> "$REPORT_FILE"
    echo >> "$REPORT_FILE"
    echo "To resolve:" >> "$REPORT_FILE"
    echo "1. Remove or resolve the TODO/FIXME/STUB items" >> "$REPORT_FILE"
    echo "2. Add specific exemptions to scripts/qa/no_todo.allow if needed" >> "$REPORT_FILE"
    echo "3. Re-run the scanner: scripts/qa/no_todo.sh" >> "$REPORT_FILE"
    
    # Output to console
    cat "$REPORT_FILE"
    
    echo
    echo "❌ QA Gate FAILED: Found TODO/FIXME/STUB markers in $found_issues file(s)"
    echo "📋 Full report: $REPORT_FILE"
    exit 1
else
    echo "✅ QA Gate PASSED: No TODO/FIXME/STUB markers found in $total_files scanned files"
    echo "✅ All clear: No TODO/FIXME/STUB markers found" >> "$REPORT_FILE"
    exit 0
fi