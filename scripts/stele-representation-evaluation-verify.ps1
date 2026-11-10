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

$containerName = "stele-representation-evaluation-$([guid]::NewGuid().ToString('N').Substring(0, 12))"
$password = "verify-$([guid]::NewGuid().ToString('N'))"
$phase = "preflight"
$failurePhase = $null
$imageDigest = "unavailable"
$suiteResults = [System.Collections.Generic.List[object]]::new()
$dsnVariable = "STELE_TEST_REPRESENTATION_EVALUATION_DSN"
$previousDsn = [Environment]::GetEnvironmentVariable($dsnVariable, "Process")
$previousPhase = $env:STELE_TEST_REPRESENTATION_PHASE
$previousCache = $env:GOCACHE
$env:GOCACHE = Join-Path $repoRoot '.tmp/gocache'

function Get-OwnedDsn {
    $portOutput = (& docker port $containerName "5432/tcp" | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $portOutput -notmatch '127\.0\.0\.1:(\d+)') { throw "owned PostgreSQL port unavailable" }
    return "postgres://stele:${password}@127.0.0.1:$($Matches[1])/stele?sslmode=disable"
}

function Get-OwnedEvidenceChecksum {
    $sql = "SELECT (SELECT count(*) FROM representation_evaluation_reports), (SELECT md5(string_agg(run_identity||aggregate::text,',' ORDER BY run_identity)) FROM representation_evaluation_reports), (SELECT count(*) FROM memory_versions), (SELECT md5(string_agg(id::text||version::text||content,',' ORDER BY id)) FROM memory_versions)"
    $checksum = (& docker exec $containerName psql -U stele -d stele -At -c $sql | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $checksum -notmatch '^4\|[a-f0-9]{32}\|\d+\|[a-f0-9]{32}$') { throw "owned persisted evidence is incomplete" }
    return $checksum
}

function Wait-OwnedPostgres([string]$Name) {
    $deadline = [DateTime]::UtcNow.AddSeconds(60)
	$previousPostmasterStart = ""
    do {
		$postmasterStart = (& docker exec $Name psql -U stele -d postgres -At -c "SELECT pg_postmaster_start_time()" 2>$null | Out-String).Trim()
		if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($postmasterStart)) {
			if ($postmasterStart -eq $previousPostmasterStart) { return }
			$previousPostmasterStart = $postmasterStart
		} else {
			$previousPostmasterStart = ""
		}
        Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "owned PostgreSQL readiness timed out"
}

function Invoke-OwnedSuite([string]$Name, [string]$Dsn) {
    $script:phase = $Name
    [Environment]::SetEnvironmentVariable($dsnVariable, $Dsn, "Process")
    if ($Name -eq "representation_safety_and_bounds") {
        $output = & go test ./internal/retrieval -run '^TestRepresentation' -count=1 -timeout 90s 2>&1
    } else {
        $output = & go test ./internal/storage/postgres -run '^TestRepresentationEvaluationPostgresOwnedReplayAndScope$' -count=1 -timeout 120s 2>&1
    }
    $exitCode = $LASTEXITCODE
    if ($Diagnostic) { $output | Write-Output }
    if ($exitCode -ne 0) { throw "owned representation evaluation suite failed" }
    $script:suiteResults.Add([pscustomobject]@{ name = $Name; status = "passed" })
}

try {
    if ([string]::IsNullOrWhiteSpace($ReportPath)) {
        $ReportPath = Join-Path ([IO.Path]::GetTempPath()) "$containerName.json"
    }
    $ReportPath = [IO.Path]::GetFullPath($ReportPath)
    $reportParent = Split-Path -Parent $ReportPath
    if ($reportParent) { New-Item -ItemType Directory -Force -Path $reportParent | Out-Null }

    $phase = "docker_dependency"
    & docker info *> $null
    if ($LASTEXITCODE -ne 0) { throw "Docker unavailable" }
    $imageDigest = (& docker image inspect --format '{{.Id}}' $Image | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $imageDigest -notmatch '^sha256:[a-f0-9]{64}$') { throw "PostgreSQL + pgvector image unavailable" }

    $phase = "owned_postgres_start"
    & docker run --detach --name $containerName --env "POSTGRES_USER=stele" --env "POSTGRES_PASSWORD=$password" --env "POSTGRES_DB=stele" --publish "127.0.0.1::5432" $Image *> $null
    if ($LASTEXITCODE -ne 0) { throw "owned PostgreSQL container failed to start" }
    Wait-OwnedPostgres $containerName

    $phase = "pgvector_dependency"
    $vectorVersion = (& docker exec $containerName psql -U stele -d stele -At -c "CREATE EXTENSION IF NOT EXISTS vector; SELECT extversion FROM pg_extension WHERE extname='vector'" | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($vectorVersion)) { throw "owned PostgreSQL pgvector extension unavailable" }
    $ownedDsn = Get-OwnedDsn

    Invoke-OwnedSuite "representation_safety_and_bounds" $ownedDsn
    $env:STELE_TEST_REPRESENTATION_PHASE = 'seed'
    Invoke-OwnedSuite "representation_postgres_real_comparison_and_rollback" $ownedDsn
    $beforeRestart = Get-OwnedEvidenceChecksum

    $phase = "owned_postgres_restart"
    if ($Diagnostic) {
        & docker restart $containerName
    } else {
        & docker restart $containerName *> $null
    }
    if ($LASTEXITCODE -ne 0) { throw "owned PostgreSQL restart failed" }
    Wait-OwnedPostgres $containerName
    if ((Get-OwnedEvidenceChecksum) -ne $beforeRestart) { throw "representation reports or canonical history changed across restart" }
    $ownedDsn = Get-OwnedDsn
    $env:STELE_TEST_REPRESENTATION_PHASE = 'post_restart'
    Invoke-OwnedSuite "representation_post_restart_read_and_scoped_cleanup" $ownedDsn
    $remaining = (& docker exec $containerName psql -U stele -d stele -At -c "SELECT count(*) FROM representation_evaluation_reports" | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $remaining -ne '0') { throw "owned report cleanup failed" }
    $suiteResults.Add([pscustomobject]@{ name = "representation_post_restart_persistence"; status = "passed" })
}
catch {
    $failurePhase = $phase
}
finally {
    [Environment]::SetEnvironmentVariable($dsnVariable, $previousDsn, "Process")
    $env:STELE_TEST_REPRESENTATION_PHASE = $previousPhase
    $env:GOCACHE = $previousCache
    $ownedContainerID = ((& docker ps -aq --filter "name=^$containerName$" 2>$null | Out-String).Trim())
    if ($ownedContainerID) {
        & docker rm --force $containerName *> $null
        if ($LASTEXITCODE -ne 0) { $failurePhase = 'owned_container_cleanup' }
    }
    $remainingContainer = (& docker ps -aq --filter "name=^$containerName$" | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $remainingContainer) { $failurePhase = 'owned_container_cleanup' }
    $report = [ordered]@{
        contract = "task-appropriate-representation-evaluation-owned-stack-v1"
        status = if ($failurePhase) { "failed" } else { "passed" }
        failed_phase = $failurePhase
        image_digest = $imageDigest
        suites = @($suiteResults)
    }
    $reportJson = $report | ConvertTo-Json -Depth 4 -Compress
    if ($reportJson.Length -gt 4096) { $reportJson = '{"contract":"task-appropriate-representation-evaluation-owned-stack-v1","status":"failed","failed_phase":"report_limit","suites":[]}' }
    Set-Content -LiteralPath $ReportPath -Value $reportJson -Encoding UTF8
    Pop-Location
}

if ($failurePhase) { throw "Task-appropriate representation owned-stack verification failed at phase '$failurePhase'; bounded report: $ReportPath" }
Write-Output "Task-appropriate representation owned-stack verification passed; bounded report: $ReportPath"
