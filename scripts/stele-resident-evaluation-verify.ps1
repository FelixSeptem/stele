[CmdletBinding()]
param(
    [string]$Image = "docker.1ms.run/pgvector/pgvector:pg18",
    [string]$ReportPath = "",
    [switch]$Diagnostic
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
$containerName = "stele-resident-evaluation-$([guid]::NewGuid().ToString('N').Substring(0, 12))"
$password = "verify-$([guid]::NewGuid().ToString('N'))"
$phase = "preflight"
$failurePhase = $null
$imageDigest = "unavailable"
$suites = [System.Collections.Generic.List[object]]::new()
$previousDsn = $env:STELE_TEST_RESIDENT_EVALUATION_DSN
$previousPhase = $env:STELE_TEST_RESIDENT_PHASE
$previousReport = $env:STELE_TEST_RESIDENT_REPORT_PATH
$previousCache = $env:GOCACHE
$env:GOCACHE = Join-Path $repoRoot '.tmp/gocache'
$aggregatePath = Join-Path ([IO.Path]::GetTempPath()) "$containerName-aggregate.json"
$aggregate = $null
$beforeRestart = $null
$afterRestart = $null
$cleanupVerified = $false

function Get-OwnedDsn {
    $portOutput = (& docker port $containerName '5432/tcp' | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $portOutput -notmatch '127\.0\.0\.1:(\d+)') { throw 'owned PostgreSQL port unavailable' }
    return "postgres://stele:${password}@127.0.0.1:$($Matches[1])/stele?sslmode=disable"
}

function Wait-OwnedPostgres {
    $deadline = [DateTime]::UtcNow.AddSeconds(60)
    $previousStart = ''
    do {
        $start = (& docker exec $containerName psql -U stele -d postgres -At -c 'SELECT pg_postmaster_start_time()' 2>$null | Out-String).Trim()
        if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($start)) {
            if ($start -eq $previousStart) { return $start }
            $previousStart = $start
        } else { $previousStart = '' }
        Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $deadline)
    throw 'owned PostgreSQL readiness timed out'
}

function Get-OwnedEvidenceChecksum {
    # Hash complete actual stored rows, including immutable text, metadata,
    # citations and projection status. No source material enters the report.
    $sql = "SELECT (SELECT count(*) FROM context_projections), (SELECT count(*) FROM context_projection_items), md5((SELECT string_agg(to_jsonb(p)::text,',' ORDER BY id) FROM context_projections p)||(SELECT string_agg(to_jsonb(i)::text,',' ORDER BY id) FROM context_projection_items i)), md5((SELECT string_agg(to_jsonb(m)::text,',' ORDER BY id) FROM canonical_memories m)||(SELECT string_agg(to_jsonb(v)::text,',' ORDER BY id) FROM memory_versions v)||(SELECT string_agg(to_jsonb(e)::text,',' ORDER BY id) FROM raw_events e)||(SELECT string_agg(to_jsonb(l)::text,',' ORDER BY id) FROM provenance_links l))"
    $checksum = (& docker exec $containerName psql -U stele -d stele -At -c $sql | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $checksum -notmatch '^2\|3\|[a-f0-9]{32}\|[a-f0-9]{32}$') { throw 'owned persisted evidence incomplete' }
    return $checksum
}

function Invoke-OwnedSuite([string]$Name, [switch]$Unit) {
    $script:phase = $Name
    if ($Unit) {
        $output = & go test ./internal/retrieval -run Resident -count=1 -timeout 90s 2>&1
    } else {
        $output = & go test ./internal/storage/postgres -run '^TestResidentEvaluationPostgresOwnedReplayAndScope$' -count=1 -timeout 120s 2>&1
    }
    $exitCode = $LASTEXITCODE
    if ($Diagnostic) { $output | Write-Output }
    if ($exitCode -ne 0) { throw 'owned resident evaluation suite failed' }
    $script:suites.Add([pscustomobject]@{ name = $Name; status = 'passed' })
}

try {
    if ([string]::IsNullOrWhiteSpace($ReportPath)) { $ReportPath = Join-Path ([IO.Path]::GetTempPath()) "$containerName.json" }
    $ReportPath = [IO.Path]::GetFullPath($ReportPath)
    $reportParent = Split-Path -Parent $ReportPath
    if ($reportParent) { New-Item -ItemType Directory -Force -Path $reportParent | Out-Null }
    $env:STELE_TEST_RESIDENT_REPORT_PATH = $aggregatePath

    Invoke-OwnedSuite 'resident_safety_bounds_and_rollback' -Unit
    $phase = 'docker_dependency'
    & docker info *> $null
    if ($LASTEXITCODE -ne 0) { throw 'Docker unavailable' }
    $imageDigest = (& docker image inspect --format '{{.Id}}' $Image | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $imageDigest -notmatch '^sha256:[a-f0-9]{64}$') { throw 'PostgreSQL + pgvector image unavailable' }

    $phase = 'owned_postgres_start'
    & docker run --detach --name $containerName --env 'POSTGRES_USER=stele' --env "POSTGRES_PASSWORD=$password" --env 'POSTGRES_DB=stele' --publish '127.0.0.1::5432' $Image *> $null
    if ($LASTEXITCODE -ne 0) { throw 'owned PostgreSQL container failed to start' }
    $initialStart = Wait-OwnedPostgres
    $phase = 'pgvector_dependency'
    $vectorVersion = (& docker exec $containerName psql -U stele -d stele -At -c "CREATE EXTENSION IF NOT EXISTS vector; SELECT extversion FROM pg_extension WHERE extname='vector'" | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($vectorVersion)) { throw 'owned pgvector extension unavailable' }
    $env:STELE_TEST_RESIDENT_EVALUATION_DSN = Get-OwnedDsn
    $env:STELE_TEST_RESIDENT_PHASE = 'seed'
    Invoke-OwnedSuite 'resident_postgres_materialization_lineage_drift_scope'
    $beforeRestart = Get-OwnedEvidenceChecksum
    $aggregate = Get-Content -Raw -LiteralPath $aggregatePath | ConvertFrom-Json
    if ($aggregate.schema -ne 'resident-evaluation-v1' -or $aggregate.status -ne 'shadow' -or $aggregate.outcome_status -ne 'inconclusive' -or -not $aggregate.rollback_tested) { throw 'resident aggregate contract invalid' }

    $phase = 'owned_postgres_restart'
    & docker restart $containerName *> $null
    if ($LASTEXITCODE -ne 0) { throw 'owned PostgreSQL restart failed' }
    $restartedAt = Wait-OwnedPostgres
    if ($restartedAt -eq $initialStart) { throw 'PostgreSQL postmaster did not restart' }
    $afterRestart = Get-OwnedEvidenceChecksum
    if ($afterRestart -ne $beforeRestart) { throw 'stored projections, history or evidence changed across restart' }
    $env:STELE_TEST_RESIDENT_EVALUATION_DSN = Get-OwnedDsn
    $env:STELE_TEST_RESIDENT_PHASE = 'post_restart'
    Invoke-OwnedSuite 'resident_post_restart_projection_history_and_citations'
    $suites.Add([pscustomobject]@{ name = 'resident_post_restart_stored_row_checksums'; status = 'passed' })
}
catch { $failurePhase = $phase }
finally {
    $env:STELE_TEST_RESIDENT_EVALUATION_DSN = $previousDsn
    $env:STELE_TEST_RESIDENT_PHASE = $previousPhase
    $env:STELE_TEST_RESIDENT_REPORT_PATH = $previousReport
    $env:GOCACHE = $previousCache
    # Only the exact unique container created above is ever removed. Its data
    # is disposable owned verification evidence; existing services are untouched.
    $ownedID = (& docker ps -aq --filter "name=^$containerName$" 2>$null | Out-String).Trim()
    if ($ownedID) {
        & docker rm --force --volumes $containerName *> $null
        if ($LASTEXITCODE -ne 0) { $failurePhase = 'owned_container_cleanup' }
    }
    $remaining = (& docker ps -aq --filter "name=^$containerName$" 2>$null | Out-String).Trim()
    $cleanupVerified = $LASTEXITCODE -eq 0 -and [string]::IsNullOrWhiteSpace($remaining)
    if (-not $cleanupVerified) { $failurePhase = 'owned_container_cleanup' }
    if (Test-Path -LiteralPath $aggregatePath) { Remove-Item -LiteralPath $aggregatePath }
    $report = [ordered]@{
        contract = 'evidence-gated-resident-evaluation-owned-stack-v1'
        status = if ($failurePhase) { 'failed' } else { 'passed' }
        failed_phase = $failurePhase
        evidence_kind = 'synthetic_conformance'
        image_digest = $imageDigest
        restart_row_checksums_equal = $null -ne $beforeRestart -and $beforeRestart -eq $afterRestart
        owned_cleanup_verified = $cleanupVerified
        suites = @($suites)
        evaluation = $aggregate
    }
    $reportJson = $report | ConvertTo-Json -Depth 8 -Compress
    if ($reportJson.Length -gt 8192) { $reportJson = '{"contract":"evidence-gated-resident-evaluation-owned-stack-v1","status":"failed","failed_phase":"report_limit"}'; $failurePhase = 'report_limit' }
    Set-Content -LiteralPath $ReportPath -Value $reportJson -Encoding UTF8
    Pop-Location
}

if ($failurePhase) { throw "Resident owned-stack verification failed at phase '$failurePhase'; bounded report: $ReportPath" }
Write-Output "Resident owned-stack verification passed; bounded report: $ReportPath"
