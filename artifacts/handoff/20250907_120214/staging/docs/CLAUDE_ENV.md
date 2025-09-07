# Claude Code Environment Setup

This document provides instructions for maintaining and troubleshooting the Claude Code agent environment.

## Quick Start

To fix agent configuration and settings issues, run:

```powershell
# Fix agent front-matter and settings permissions
pwsh -File tools/claude_doctor.ps1

# Run in dry-run mode to preview changes
pwsh -File tools/claude_doctor.ps1 -DryRun

# Run with verbose output
pwsh -File tools/claude_doctor.ps1 -Verbose
```

After running the doctor script, validate fixes in Claude Code with:
```
/doctor
```

## What the Doctor Script Does

The `tools/claude_doctor.ps1` script ensures:

1. **Agent Front-Matter Validation**:
   - All `.claude/agents/*.md` files have proper YAML front-matter
   - Missing `name:` fields are auto-generated from filename
   - Maintains idempotent behavior (safe to run multiple times)

2. **Settings Normalization**:
   - Standardizes `.claude/settings.json` permissions format
   - Ensures consistent allow/deny patterns
   - Preserves existing configurations while fixing format issues

## Agent File Structure

Agent files should follow this structure:

```yaml
---
name: agent-slug
description: Brief description of agent capabilities.
model: sonnet
tools: Read, Bash, Edit
---

# Agent implementation details...
```

## Common Issues

### Missing Agent Front-Matter
**Symptom**: Claude Code reports agent registration errors
**Fix**: Run `pwsh -File tools/claude_doctor.ps1`

### Settings Permission Errors
**Symptom**: Claude Code denies tool usage unexpectedly
**Fix**: Run doctor script to normalize permissions format

### Agent Name Conflicts
**Symptom**: Multiple agents with same name or missing names
**Fix**: Doctor script generates unique slugs based on filenames

## Manual Verification

To manually check agent health:

```powershell
# Check agent files have front-matter
Get-ChildItem .claude/agents/*.md | ForEach-Object {
    $content = Get-Content $_.FullName -Raw
    if (-not $content.StartsWith('---')) {
        Write-Warning "Missing front-matter: $($_.Name)"
    }
}

# Validate settings.json format
Get-Content .claude/settings.json | ConvertFrom-Json | ConvertTo-Json -Depth 10
```

## Best Practices

1. **Regular Maintenance**: Run doctor script after adding new agents
2. **Idempotent Operations**: Doctor script is safe to run repeatedly
3. **Version Control**: Always commit agent file changes
4. **Testing**: Use `/doctor` command in Claude Code to validate

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Agent not recognized | Run doctor script to fix front-matter |
| Permission denied | Check settings.json with doctor script |
| Duplicate agent names | Use unique filenames; doctor generates slugs |
| Settings corruption | Doctor script normalizes format |

## Files Managed

- `.claude/agents/*.md` - Agent definition files
- `.claude/settings.json` - Claude Code configuration
- `tools/claude_doctor.ps1` - Maintenance script

For more information, see the Claude Code documentation.