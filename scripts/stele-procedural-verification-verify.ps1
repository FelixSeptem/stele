[CmdletBinding()]
param(
    [string]$Image = "docker.1ms.run/pgvector/pgvector:pg18",
    [string]$ReportPath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot

$containerName = "stele-procedural-verification-$([guid]::NewGuid().ToString('N').Substring(0, 12))"
$password = "verify-$([guid]::NewGuid().ToString('N'))"
$phase = "preflight"
$failurePhase = $null
$imageDigest = "unavailable"
$suiteResults = [System.Collections.Generic.List[object]]::new()
$variable = "STELE_TEST_POSTGRES_VERIFICATION_DSN"
$previousDsn = [Environment]::GetEnvironmentVariable($variable, "Process")

function Invoke-OwnedSuite([string]$Name, [string]$Dsn) {
    $script:phase = $Name
    [Environment]::SetEnvironmentVariable($variable, $Dsn, "Process")
    & go test ./internal/storage/postgres -run '^TestProceduralVerificationPostgresReplayScopeAndLeaseRecovery$' -count=1 -timeout 90s *> $null
    if ($LASTEXITCODE -ne 0) { throw "owned procedural verification suite failed" }
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
    $portOutput = (& docker port $containerName "5432/tcp" | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $portOutput -notmatch '127\.0\.0\.1:(\d+)') { throw "owned PostgreSQL port unavailable" }
    $port = $Matches[1]
    $deadline = [DateTime]::UtcNow.AddSeconds(60)
    $previousPostmasterStart = ""
    $postgresReady = $false
    do {
        $postmasterStart = (& docker exec $containerName psql -U stele -d postgres -At -c "SELECT pg_postmaster_start_time()" 2>$null | Out-String).Trim()
        if ($LASTEXITCODE -eq 0 -and $postmasterStart) {
            if ($postmasterStart -eq $previousPostmasterStart) {
                $postgresReady = $true
                break
            }
            $previousPostmasterStart = $postmasterStart
        } else {
            $previousPostmasterStart = ""
        }
        Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $deadline)
    if (-not $postgresReady) { throw "owned PostgreSQL readiness timed out" }

    $phase = "owned_database_setup"
    $ownedDatabasePresent = (& docker exec $containerName psql -U stele -d postgres -At -c "SELECT 1 FROM pg_database WHERE datname = 'stele'" | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) { throw "owned PostgreSQL database lookup failed" }
    if ($ownedDatabasePresent -ne "1") {
        & docker exec $containerName createdb -U stele --maintenance-db=postgres stele
        if ($LASTEXITCODE -ne 0) { throw "owned PostgreSQL database creation failed" }
    }

    $phase = "pgvector_dependency"
    $vectorVersion = (& docker exec $containerName psql -U stele -d stele -At -c "CREATE EXTENSION IF NOT EXISTS vector; SELECT extversion FROM pg_extension WHERE extname='vector'" | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($vectorVersion)) { throw "owned PostgreSQL pgvector extension unavailable" }

    Invoke-OwnedSuite "verification_replay_scope_lease_restart" "postgres://stele:${password}@127.0.0.1:${port}/stele?sslmode=disable"
}
catch {
    $failurePhase = $phase
}
finally {
    [Environment]::SetEnvironmentVariable($variable, $previousDsn, "Process")
    $ownedContainerID = ((& docker ps -aq --filter "name=^$containerName$" 2>$null | Out-String).Trim())
    if ($ownedContainerID) { & docker rm --force $containerName *> $null }
    $report = [ordered]@{
        contract = "procedural-verification-owned-stack-v1"
        status = if ($failurePhase) { "failed" } else { "passed" }
        failed_phase = $failurePhase
        image_digest = $imageDigest
        suites = @($suiteResults)
    }
    $reportJson = $report | ConvertTo-Json -Depth 4
    if ($reportJson.Length -gt 4096) { $reportJson = '{"contract":"procedural-verification-owned-stack-v1","status":"failed","failed_phase":"report_limit","suites":[]}' }
    Set-Content -LiteralPath $ReportPath -Value $reportJson -Encoding UTF8
    Pop-Location
}

if ($failurePhase) { throw "Procedural verification owned-stack verification failed at phase '$failurePhase'; bounded report: $ReportPath" }
Write-Output "Procedural verification owned-stack verification passed; bounded report: $ReportPath"
