# Delayed Prompts - Native OS Scheduling

Execute Claude Code prompts at future times using only built-in OS commands. No external scripts required.

## Quick Start

**Windows (Robust):**
```cmd
schtasks /Create /SC ONCE /TN "ClaudeTask" /TR "claude --prompt=\"Scan markets\"" /ST 14:05 /SD 2025/09/07 /RL LIMITED /F
```

**Windows (Simple Delay):**
```cmd
start "ClaudeDelay" /min cmd /c "timeout /t 900 /nobreak && claude --prompt=\"Generate report\""
```

**macOS/Linux:**
```bash
nohup sh -c 'sleep 900; claude --prompt="Generate report"' >/dev/null 2>&1 &
```

## Syntax Patterns

### IN= (Relative Delay)
- `IN=15m` → 15 minutes
- `IN=2h` → 2 hours  
- `IN=30s` → 30 seconds

### AT= (Absolute Time)
- `AT="2025-09-07 14:05"` → Today at 2:05 PM
- `AT="14:30"` → Today at 2:30 PM
- `AT="tomorrow 09:00"` → Tomorrow 9 AM

## Platform Details

### Windows (schtasks - Robust)
**Clock Time:**
```cmd
schtasks /Create /SC ONCE /TN "<TASK_NAME>" /TR "cmd /c <CLAUDE_CALL>" /ST HH:MM /SD YYYY/MM/DD /RL LIMITED /F
```

**Advantages:** Persistent across reboots, precise timing, system-level scheduling
**Requirements:** Administrator privileges for /Create

### Windows (timeout - Simple)
**Delay Only:**
```cmd
start "ClaudeDelay" /min cmd /c "timeout /t <SECONDS> /nobreak && <CLAUDE_CALL>"
```

**Advantages:** No admin required, immediate setup
**Limitations:** Process dies if parent terminal closes

### macOS/Linux (nohup+sleep)
**Delay:**
```bash
nohup sh -c 'sleep <SECONDS>; <CLAUDE_CALL>' >/dev/null 2>&1 &
```

**Clock Time:** Use `at` command if available:
```bash
echo "<CLAUDE_CALL>" | at 14:05
```

## Safe Quoting Rules

### Paths with Spaces
**Windows:** Use double quotes around entire TR parameter
```cmd
schtasks ... /TR "cmd /c \"C:\Program Files\Claude\claude.exe\" --file=\"C:\My Documents\prompt.md\""
```

**POSIX:** Escape or single-quote the command
```bash
nohup sh -c 'sleep 300; "/usr/local/bin/claude" --file="/home/user/My Documents/prompt.md"' &
```

### Prompt Strings
**Windows:** Escape inner quotes with backslash
```cmd
/TR "claude --prompt=\"Generate \\\"daily\\\" report\""
```

**POSIX:** Use single quotes for outer, double for inner
```bash
'claude --prompt="Generate daily report"'
```

## Sectioned Prompt Files

Mark sections in prompt files with:
```markdown
PROMPT_ID=MORNING_SCAN
Run morning market scan with top-50 gainers analysis.

PROMPT_ID=EVENING_REPORT  
Generate end-of-day momentum report with regime analysis.
```

Execute specific section:
```bash
PROMPT_ID=MORNING_SCAN claude --file=/path/to/prompts.md
```

## Troubleshooting

### schtasks Permission Denied
- Run as Administrator, or
- Use simple `timeout` method, or  
- Grant user "Log on as a batch job" right

### PATH Issues
Use absolute paths to Claude executable:
- Windows: `"C:\Users\%USERNAME%\AppData\Local\Claude\claude.exe"`
- macOS: `"/usr/local/bin/claude"`
- Linux: `"/usr/bin/claude"`

### nohup Command Not Found
- Install coreutils: `brew install coreutils` (macOS)
- Use `screen` alternative: `screen -dmS claude_delay sh -c 'sleep 300; claude --prompt="test"'`

### Time Zone Confusion
- Use 24-hour format: `14:05` not `2:05 PM`
- Specify date explicitly: `2025/09/07` 
- Check system time: `date` (POSIX) or `time /t` (Windows)

## Batch Queue (YAML)

Schedule multiple delayed prompts from a single YAML file:

### Queue File Format
```yaml
# prompts/schedule_queue.yaml
jobs:
  - title: ScoringMenuDelay        # Required: Unique task name
    file: prompts/scoring_menu.txt  # Required: Path to prompt file
    in: 12m                        # Required: Either 'in' OR 'at'
    
  - title: VerifyPostmerge
    file: prompts/verify.txt
    at: "2025-09-07 14:05"         # Clock time (24h format)
    prompt_id: VERIFY.SECTION      # Optional: Execute specific section
    
  - title: BenchSnapshot  
    file: prompts/bench.txt
    in: 20m
    mode: simple                   # Optional: Force timeout method
```

### Field Reference
- `title` *(required)*: Unique identifier for the scheduled task
- `file` *(required)*: Relative or absolute path to prompt file  
- `in` *(timing)*: Relative delay (`15m`, `2h`, `30s`)
- `at` *(timing)*: Absolute time (`"2025-09-07 14:05"`, `"14:30"`)
- `prompt_id` *(optional)*: Execute only tagged section in file
- `mode` *(optional)*: `"auto"` (default) or `"simple"` (force timeout)

**Note**: Each job must have exactly one timing field (`in` OR `at`).

### Usage
```bash
# Load and schedule all jobs from YAML file
QUEUE_FILE=prompts/schedule_queue.yaml CREATE_EXAMPLE=true claude --batch-schedule

# Creates machine-readable receipt in artifacts/scheduled_runs.jsonl
```

## Examples

**Scan markets in 2 hours:**
```cmd
start "MarketScan" /min cmd /c "timeout /t 7200 /nobreak && claude --prompt=\"Scan top-100 USD pairs with regime detection\""
```

**Generate report at market close:**
```bash
echo 'claude --prompt="Generate EOD momentum report with P&L attribution"' | at 16:00
```

**Execute morning routine from file:**
```cmd
schtasks /Create /SC ONCE /TN "MorningRoutine" /TR "claude --file=\"C:\claude\morning_tasks.md\"" /ST 08:30 /SD 2025/09/07 /RL LIMITED /F
```