# Progress Calculator for CryptoRun v3.2.1
# Computes weighted milestone completion percentage

param(
    [switch]$FailIfNoGain,
    [string]$BaselineRef = "origin/main",
    [string]$ProgressFile = ".progress"
)

# Parse PROGRESS.yaml
function Parse-ProgressYaml {
    param([string]$FilePath)
    
    if (!(Test-Path $FilePath)) {
        Write-Error "PROGRESS.yaml not found at $FilePath"
        exit 1
    }
    
    $content = Get-Content $FilePath -Raw
    $milestones = @{}
    $totalWeight = 0
    $completedWeight = 0
    
    # Simple YAML parsing for milestones section
    $inMilestones = $false
    $currentMilestone = $null
    
    foreach ($line in $content -split "`n") {
        $line = $line.Trim()
        
        if ($line -eq "milestones:") {
            $inMilestones = $true
            continue
        }
        
        if ($inMilestones -and $line -match "^# ") {
            continue  # Skip comments
        }
        
        if ($inMilestones -and $line -eq "" -and $currentMilestone) {
            # End of milestone
            $currentMilestone = $null
            continue
        }
        
        if ($inMilestones -and $line -eq "calculation:") {
            break  # End of milestones section
        }
        
        if ($inMilestones -and $line -match "^([a-z_]+):$") {
            $currentMilestone = $matches[1]
            $milestones[$currentMilestone] = @{
                weight = 0
                completed = $false
                progress = 0
            }
            continue
        }
        
        if ($currentMilestone -and $line -match "^\s*weight:\s*(\d+)") {
            $milestones[$currentMilestone].weight = [int]$matches[1]
            $totalWeight += [int]$matches[1]
        }
        
        if ($currentMilestone -and $line -match "^\s*completed:\s*(true|false)") {
            $milestones[$currentMilestone].completed = ($matches[1] -eq "true")
        }
        
        if ($currentMilestone -and $line -match "^\s*progress:\s*(\d+)") {
            $milestones[$currentMilestone].progress = [int]$matches[1]
        }
    }
    
    # Calculate weighted completion
    foreach ($milestone in $milestones.Keys) {
        $m = $milestones[$milestone]
        if ($m.completed) {
            $completedWeight += $m.weight
        } else {
            $completedWeight += $m.weight * ($m.progress / 100.0)
        }
    }
    
    $percentage = if ($totalWeight -gt 0) { ($completedWeight / $totalWeight) * 100.0 } else { 0.0 }
    
    return @{
        Percentage = [math]::Round($percentage, 1)
        TotalWeight = $totalWeight
        CompletedWeight = [math]::Round($completedWeight, 1)
        Milestones = $milestones
    }
}

# Get current progress
$currentProgress = Parse-ProgressYaml "PROGRESS.yaml"
$currentPercent = $currentProgress.Percentage

# Write current progress to file
$currentPercent | Out-File -FilePath $ProgressFile -NoNewline -Encoding ASCII

Write-Host "Progress: $currentPercent% ($($currentProgress.CompletedWeight)/$($currentProgress.TotalWeight) weighted points)"

# If -FailIfNoGain is specified, check against baseline
if ($FailIfNoGain) {
    $baselinePercent = 0.0
    
    # Try to get baseline from git
    $gitAvailable = $false
    try {
        git status *>$null
        $gitAvailable = $true
    } catch {
        Write-Warning "Git not available, using file-based baseline"
    }
    
    if ($gitAvailable) {
        # Get baseline PROGRESS.yaml from base branch
        try {
            $baselineContent = git show "${BaselineRef}:PROGRESS.yaml" 2>$null
            if ($baselineContent -and $LASTEXITCODE -eq 0) {
                $tempFile = [System.IO.Path]::GetTempFileName()
                $baselineContent | Out-File -FilePath $tempFile -Encoding UTF8
                $baselineProgress = Parse-ProgressYaml $tempFile
                $baselinePercent = $baselineProgress.Percentage
                Remove-Item $tempFile -Force
                Write-Host "Baseline ($BaselineRef): $baselinePercent%"
            } else {
                Write-Warning "Could not get baseline from $BaselineRef, assuming 0%"
            }
        } catch {
            Write-Warning "Error getting baseline from git: $_"
        }
    } else {
        # Use previous .progress file if exists
        if (Test-Path $ProgressFile) {
            try {
                $baselinePercent = [double](Get-Content $ProgressFile -Raw).Trim()
                Write-Host "Previous progress: $baselinePercent%"
            } catch {
                Write-Warning "Could not parse previous progress file"
            }
        }
    }
    
    $delta = $currentPercent - $baselinePercent
    Write-Host "Progress delta: $delta%"
    
    if ($delta -lt 0.1) {
        Write-Error "❌ Progress must increase by at least 0.1% (current: $currentPercent%, baseline: $baselinePercent%, delta: $delta%)"
        exit 1
    } else {
        Write-Host "✅ Progress increased by $delta%"
    }
}

exit 0