[CmdletBinding()]
param(
    [string]$Image = 'docker.1ms.run/pgvector/pgvector:pg18',
    [string]$ReportPath = '',
    [string]$LogPath = '',
    [string]$GoExe = '',
    [switch]$Diagnostic
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
# Docker Desktop ships an extensionless `docker` launcher next to `docker.exe`.
# pwsh refuses to run that launcher as a document inside a pipeline, so resolve
# the native executable explicitly for all owned-container commands.
$dockerExe = (Get-Command docker.exe -ErrorAction Stop).Source
if (-not $GoExe) {
    $goCommand = Get-Command go.exe -ErrorAction SilentlyContinue
    if (-not $goCommand) { throw 'Go toolchain is unavailable on PATH' }
    $GoExe = $goCommand.Source
}

# The harness writes its own phase log so callers never need to wrap it in a
# pipeline; pwsh refuses to run native executables when a script itself is a
# pipeline element. No input identity, DSN, credential or report payload is logged.
function Write-OwnedLog([string]$Message) {
    if ($LogPath) { Add-Content -LiteralPath $LogPath -Value $Message }
}
$container = 'stele-longitudinal-v3-' + [guid]::NewGuid().ToString('N').Substring(0,12)
$password = 'owned-' + [guid]::NewGuid().ToString('N')
$aggregatePath = Join-Path ([IO.Path]::GetTempPath()) ($container + '-aggregate.json')
$previousDsn = $env:STELE_TEST_TASK_SERIES_V3_DSN
$previousReport = $env:STELE_TEST_TASK_SERIES_V3_REPORT
$previousCache = $env:GOCACHE
$phase = 'preflight'
$failure = $null
$cleanup = $false
$imageDigest = 'unavailable'
$suites = [System.Collections.Generic.List[string]]::new()
$aggregate = $null
$restartVerified = $false

function Wait-OwnedPostgres {
    $deadline = [DateTime]::UtcNow.AddSeconds(60)
    do {
        $raw = & $dockerExe exec $container psql -U stele -d stele -At -c 'SELECT pg_postmaster_start_time()' 2>$null
        $value = ($raw | Out-String).Trim()
        if ($LASTEXITCODE -eq 0 -and $value) { return $value }
        Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $deadline)
    throw 'owned PostgreSQL readiness timed out'
}

function Invoke-OwnedTest([string]$Pattern) {
    Write-OwnedLog "test_start=$Pattern"
    $result = & $GoExe test ./internal/benchmark -run $Pattern -count=1 -timeout 240s 2>&1
    $code = $LASTEXITCODE
    if ($Diagnostic) { $result | Write-Output }
    Write-OwnedLog "test_exit=$code"
    if ($code -ne 0) {
        foreach ($line in $result) { Write-OwnedLog ('test_output=' + ($line | Out-String).Trim()) }
        throw 'longitudinal v3 test failed'
    }
}

try {
    Write-OwnedLog 'phase=preflight'
    if (-not $ReportPath) { $ReportPath = Join-Path ([IO.Path]::GetTempPath()) ($container + '.json') }
    $ReportPath = [IO.Path]::GetFullPath($ReportPath)
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $ReportPath) | Out-Null
    $env:GOCACHE = Join-Path $repoRoot '.tmp/gocache'
    $phase = 'unit_contracts'
    Invoke-OwnedTest 'TaskSeriesV3|TaskSeriesLegacyRejects|TaskSeriesV2Inconclusive'
    $suites.Add('contracts_bounds_missing_costs_and_safety')
    $phase = 'docker_dependency'
    & $dockerExe info *> $null
    if ($LASTEXITCODE -ne 0) { throw 'Docker unavailable' }
    $rawImage = & $dockerExe image inspect --format '{{.Id}}' $Image
    $imageDigest = ($rawImage | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $imageDigest -notmatch '^sha256:[a-f0-9]{64}$') { throw 'owned image unavailable' }
    $phase = 'owned_postgres_start'
    & $dockerExe run --detach --name $container --env POSTGRES_USER=stele --env "POSTGRES_PASSWORD=$password" --env POSTGRES_DB=stele --publish '127.0.0.1::5432' $Image *> $null
    if ($LASTEXITCODE -ne 0) { throw 'owned container startup failed' }
    $beforeStart = Wait-OwnedPostgres
    $rawPort = & $dockerExe port $container '5432/tcp'
    $portText = ($rawPort | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $portText -notmatch '127\.0\.0\.1:(\d+)') { throw 'owned port unavailable' }
    $env:STELE_TEST_TASK_SERIES_V3_DSN = "postgres://stele:${password}@127.0.0.1:$($Matches[1])/stele?sslmode=disable"
    $env:STELE_TEST_TASK_SERIES_V3_REPORT = $aggregatePath
    $phase = 'owned_staged_replay'
    Invoke-OwnedTest '^TestTaskSeriesV3PostgresOwnedSnapshots$'
    $suites.Add('actual_stage_windows_lineage_disclosure_budgets_and_baseline_replay')
    $aggregate = Get-Content -Raw -LiteralPath $aggregatePath | ConvertFrom-Json
    if ($aggregate.schema_version -ne 'task-series-v3' -or -not $aggregate.synthetic_fixture -or $aggregate.v3.gate_status -ne 'passed' -or $aggregate.v3.comparison_complete -or -not $aggregate.cleanup_verified) { throw 'bounded report contract mismatch' }
    $phase = 'owned_postgres_restart'
    & $dockerExe restart $container *> $null
    if ($LASTEXITCODE -ne 0) { throw 'owned restart failed' }
    $afterStart = Wait-OwnedPostgres
    if ($beforeStart -eq $afterStart) { throw 'postmaster did not restart' }
    # Docker Desktop reassigns an ephemeral host port on container restart;
    # the post-restart DSN must use the currently published port, not the old one.
    $rawPort = & $dockerExe port $container '5432/tcp'
    $restartPortText = ($rawPort | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $restartPortText -notmatch '127\.0\.0\.1:(\d+)') { throw 'owned restart port unavailable' }
    $env:STELE_TEST_TASK_SERIES_V3_DSN = "postgres://stele:${password}@127.0.0.1:$($Matches[1])/stele?sslmode=disable"
    Write-OwnedLog 'phase=post_restart_replay_port_rebound'
    $phase = 'post_restart_replay'
    Invoke-OwnedTest '^TestTaskSeriesV3PostgresOwnedSnapshots$'
    $later = Get-Content -Raw -LiteralPath $aggregatePath | ConvertFrom-Json
    $restartVerified = $later.v3.input_digest -eq $aggregate.v3.input_digest -and $later.normalized_checksum -eq $aggregate.normalized_checksum -and $later.v3.gate_status -eq 'passed'
    if (-not $restartVerified) { throw 'post-restart input identity or gates changed' }
    $suites.Add('post_restart_staged_replay_and_verified_row_cleanup')
} catch { $failure = $phase; Write-OwnedLog ('failed_phase=' + $phase + ' error=' + $_.Exception.Message) }
finally {
    $env:STELE_TEST_TASK_SERIES_V3_DSN = $previousDsn
    $env:STELE_TEST_TASK_SERIES_V3_REPORT = $previousReport
    $env:GOCACHE = $previousCache
    $rawOwned = & $dockerExe ps -aq --filter "name=^$container$" 2>$null
    $owned = ($rawOwned | Out-String).Trim()
    if ($owned) { & $dockerExe rm --force --volumes $container *> $null; if ($LASTEXITCODE -ne 0) { $failure = 'owned_cleanup' } }
    $rawRemaining = & $dockerExe ps -aq --filter "name=^$container$" 2>$null
    $remaining = ($rawRemaining | Out-String).Trim()
    $cleanup = $LASTEXITCODE -eq 0 -and -not $remaining
    if (-not $cleanup) { $failure = 'owned_cleanup' }
    if (Test-Path -LiteralPath $aggregatePath) {
        try { Remove-Item -LiteralPath $aggregatePath } catch { Write-OwnedLog 'aggregate_temp_cleanup_failed' }
    }
    $report = [ordered]@{ contract='locked-longitudinal-owned-stack-v1'; status=$(if ($failure) {'failed'} else {'passed'}); failed_phase=$failure; evidence_kind='synthetic_conformance'; image_digest=$imageDigest; restart_replay_verified=$restartVerified; owned_cleanup_verified=$cleanup; suites=@($suites); evaluation=$aggregate }
    $encoded = $report | ConvertTo-Json -Depth 12 -Compress
    if ($encoded.Length -gt 16384) { $encoded='{"contract":"locked-longitudinal-owned-stack-v1","status":"failed","failed_phase":"report_bound"}';$failure='report_bound' }
    Set-Content -LiteralPath $ReportPath -Value $encoded -Encoding UTF8
    Write-OwnedLog ('cleanup_verified=' + $cleanup + ' restart_verified=' + $restartVerified + ' image_digest=' + $imageDigest)
    Pop-Location
}
Write-OwnedLog ('status=' + $(if ($failure) { 'failed' } else { 'passed' }))
if ($failure) { throw "Longitudinal v3 owned verification failed at '$failure'; bounded report: $ReportPath" }
Write-Output "Longitudinal v3 owned verification passed; bounded report: $ReportPath"
