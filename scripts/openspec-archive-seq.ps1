param(
    [string]$ChangeName = "",
    [switch]$MigrateExisting = $false,
    [switch]$SkipSpecs = $false,
    [switch]$NoValidate = $false
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-SlugFromArchiveDirName {
    param([string]$Name)

    if ($Name -match "^\d{3}-(.+)$") {
        return $Matches[1]
    }

    if ($Name -match "^\d{4}-\d{2}-\d{2}-(.+)$") {
        return $Matches[1]
    }

    return $Name
}

function Get-SeqFromArchiveDirName {
    param([string]$Name)

    if ($Name -match "^(\d{3})-.+$") {
        return [int]$Matches[1]
    }

    return $null
}

function Get-NextSequence {
    param([string]$ArchiveRoot)

    $maxSeq = 0
    if (Test-Path $ArchiveRoot) {
        Get-ChildItem -Path $ArchiveRoot -Directory | ForEach-Object {
            $seq = Get-SeqFromArchiveDirName -Name $_.Name
            if ($null -ne $seq -and $seq -gt $maxSeq) {
                $maxSeq = $seq
            }
        }
    }

    return ($maxSeq + 1)
}

function Write-ArchiveIndex {
    param(
        [string]$ArchiveRoot,
        [string]$OutputPath
    )

    $lines = @(
        "# Archive Index",
        "",
        "Updated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')",
        ""
    )

    $dirs = @()
    if (Test-Path $ArchiveRoot) {
        $dirs = Get-ChildItem -Path $ArchiveRoot -Directory |
            Sort-Object {
                $seq = Get-SeqFromArchiveDirName -Name $_.Name
                if ($null -eq $seq) {
                    return 999999
                }

                return $seq
            }, Name
    }

    foreach ($dir in $dirs) {
        $seq = Get-SeqFromArchiveDirName -Name $dir.Name
        $slug = Get-SlugFromArchiveDirName -Name $dir.Name
        if ($null -ne $seq) {
            $lines += ("- {0:D3} -> {1}" -f $seq, $slug)
            continue
        }

        $lines += ("- n/a -> {0}" -f $slug)
    }

    $lines | Set-Content -Path $OutputPath
}

function Rename-ArchiveDirs {
    param(
        [string]$ArchiveRoot,
        [array]$Plan
    )

    if ($Plan.Count -eq 0) {
        return
    }

    $tmpPlan = New-Object System.Collections.ArrayList

    foreach ($item in $Plan) {
        $sourcePath = Join-Path $ArchiveRoot $item.Source
        if (-not (Test-Path $sourcePath)) {
            continue
        }
        $tempName = "__tmp__" + [guid]::NewGuid().ToString("N")
        Rename-Item -Path $sourcePath -NewName $tempName
        [void]$tmpPlan.Add([pscustomobject]@{
                Temp   = $tempName
                Target = $item.Target
            })
    }

    foreach ($item in $tmpPlan) {
        $tempPath = Join-Path $ArchiveRoot $item.Temp
        Rename-Item -Path $tempPath -NewName $item.Target
    }
}

function Migrate-ExistingArchive {
    param([string]$ArchiveRoot)

    if (-not (Test-Path $ArchiveRoot)) {
        New-Item -ItemType Directory -Path $ArchiveRoot | Out-Null
    }

    $dirs = @(Get-ChildItem -Path $ArchiveRoot -Directory)
    if ($dirs.Count -eq 0) {
        return
    }

    $occupiedSeq = @{}
    $plan = New-Object System.Collections.ArrayList
    $pending = New-Object System.Collections.ArrayList

    foreach ($dir in ($dirs | Sort-Object Name)) {
        $slug = Get-SlugFromArchiveDirName -Name $dir.Name
        $existingSeq = Get-SeqFromArchiveDirName -Name $dir.Name

        if ($null -ne $existingSeq -and -not $occupiedSeq.ContainsKey($existingSeq)) {
            $occupiedSeq[$existingSeq] = $true
            $target = ("{0:D3}-{1}" -f $existingSeq, $slug)
            if ($dir.Name -ne $target) {
                [void]$plan.Add([pscustomobject]@{
                        Source = $dir.Name
                        Target = $target
                    })
            }
            continue
        }

        [void]$pending.Add($dir)
    }

    $next = 1
    foreach ($dir in ($pending | Sort-Object LastWriteTime, Name)) {
        while ($occupiedSeq.ContainsKey($next)) {
            $next++
        }

        $slug = Get-SlugFromArchiveDirName -Name $dir.Name
        $target = ("{0:D3}-{1}" -f $next, $slug)
        $occupiedSeq[$next] = $true

        if ($dir.Name -ne $target) {
            [void]$plan.Add([pscustomobject]@{
                    Source = $dir.Name
                    Target = $target
                })
        }

        $next++
    }

    Rename-ArchiveDirs -ArchiveRoot $ArchiveRoot -Plan $plan
}

function Invoke-OpenSpecArchiveCommand {
    param(
        [string]$Change,
        [switch]$SkipSpecs = $false,
        [switch]$NoValidate = $false
    )

    $arguments = @("archive", $Change, "-y")
    if ($SkipSpecs) {
        $arguments += "--skip-specs"
    }
    if ($NoValidate) {
        $arguments += "--no-validate"
    }

    & openspec @arguments
}

function Archive-OneChange {
    param(
        [string]$Change,
        [string]$ChangesRoot,
        [string]$ArchiveRoot,
        [string]$RepoRoot,
        [switch]$SkipSpecs = $false,
        [switch]$NoValidate = $false
    )

    if ([string]::IsNullOrWhiteSpace($Change)) {
        return
    }

    $activeDir = Join-Path $ChangesRoot $Change

    Push-Location $RepoRoot
    try {
        Invoke-OpenSpecArchiveCommand -Change $Change -SkipSpecs:$SkipSpecs -NoValidate:$NoValidate
    } catch {
        Write-Warning ("openspec archive returned error: " + $_.Exception.Message)
    } finally {
        Pop-Location
    }

    $escaped = [regex]::Escape($Change)
    $archivedDir = Get-ChildItem -Path $ArchiveRoot -Directory |
        Where-Object { $_.Name -match ("^(\d{3}|\d{4}-\d{2}-\d{2})-" + $escaped + "$") } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1

    if ($null -eq $archivedDir) {
        Write-Warning ("archive output for change '" + $Change + "' was not found; skip cleanup.")
        return
    }

    $requiredFiles = @("proposal.md", "tasks.md")
    if (Test-Path (Join-Path $activeDir ".openspec.yaml")) {
        $requiredFiles += ".openspec.yaml"
    }
    if (Test-Path (Join-Path $activeDir "design.md")) {
        $requiredFiles += "design.md"
    }

    foreach ($relativePath in $requiredFiles) {
        if (-not (Test-Path (Join-Path $archivedDir.FullName $relativePath))) {
            Write-Warning ("archive incomplete for '" + $Change + "': missing " + $relativePath + "; skip cleanup.")
            return
        }
    }

    $activeSpecsDir = Join-Path $activeDir "specs"
    if (Test-Path $activeSpecsDir) {
        $activeSpecFiles = Get-ChildItem -Path $activeSpecsDir -Recurse -File -Filter "spec.md"
        foreach ($spec in $activeSpecFiles) {
            $relative = $spec.FullName.Substring($activeDir.Length).TrimStart('\', '/')
            if (-not (Test-Path (Join-Path $archivedDir.FullName $relative))) {
                Write-Warning ("archive incomplete for '" + $Change + "': missing " + $relative + "; skip cleanup.")
                return
            }
        }
    }

    $archivedSpecCount = (Get-ChildItem -Path $archivedDir.FullName -Recurse -File -Filter "spec.md" | Measure-Object).Count
    if ($archivedSpecCount -eq 0) {
        Write-Warning ("archive incomplete for '" + $Change + "': no spec.md found; skip cleanup.")
        return
    }

    if (Test-Path $activeDir) {
        Remove-Item -Path $activeDir -Recurse -Force
    }

    if ($archivedDir.Name -match "^\d{3}-$escaped$") {
        return
    }

    if ($archivedDir.Name -notmatch "^\d{4}-\d{2}-\d{2}-$escaped$") {
        return
    }

    $nextSeq = Get-NextSequence -ArchiveRoot $ArchiveRoot
    $target = "{0:D3}-{1}" -f $nextSeq, $Change
    Rename-Item -Path $archivedDir.FullName -NewName $target
}

function Invoke-OpenSpecArchiveSeq {
    param(
        [string]$ChangeName = "",
        [switch]$MigrateExisting = $false,
        [switch]$SkipSpecs = $false,
        [switch]$NoValidate = $false
    )

    $repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
    $changesDir = Join-Path $repoRoot "openspec/changes"
    $archiveDir = Join-Path $changesDir "archive"
    $indexPath = Join-Path $archiveDir "INDEX.md"

    if (-not (Test-Path $archiveDir)) {
        New-Item -ItemType Directory -Path $archiveDir | Out-Null
    }

    if ($MigrateExisting) {
        Migrate-ExistingArchive -ArchiveRoot $archiveDir
    }

    if (-not [string]::IsNullOrWhiteSpace($ChangeName)) {
        Archive-OneChange -Change $ChangeName -ChangesRoot $changesDir -ArchiveRoot $archiveDir -RepoRoot $repoRoot -SkipSpecs:$SkipSpecs -NoValidate:$NoValidate
        Migrate-ExistingArchive -ArchiveRoot $archiveDir
    }

    Write-ArchiveIndex -ArchiveRoot $archiveDir -OutputPath $indexPath
    Write-Host "Done. Archive naming is normalized with incrementing sequence prefixes and INDEX.md is updated."
}

if ($MyInvocation.InvocationName -ne ".") {
    Invoke-OpenSpecArchiveSeq -ChangeName $ChangeName -MigrateExisting:$MigrateExisting -SkipSpecs:$SkipSpecs -NoValidate:$NoValidate
}
