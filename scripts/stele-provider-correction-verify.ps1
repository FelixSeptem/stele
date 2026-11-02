[CmdletBinding()]
param(
    [string]$Image = "docker.1ms.run/pgvector/pgvector:pg18",
    [string]$ReportPath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
$containerName = "stele-correction-$([guid]::NewGuid().ToString('N').Substring(0, 12))"
$password = "verify-$([guid]::NewGuid().ToString('N'))"
$reportPath = $ReportPath
$phase = "preflight"
$failurePhase = $null
$imageDigest = "unavailable"
$suiteResults = [System.Collections.Generic.List[object]]::new()
$verificationVariables = @("STELE_TEST_PROVIDER_CORRECTION_DSN", "STELE_TEST_PROVIDER_PROJECTION_DSN", "STELE_TEST_PROVIDER_CORRECTION_GOVERNED_DSN")
$previousVariables = @{}
foreach ($variable in $verificationVariables) {
    $previousVariables[$variable] = [Environment]::GetEnvironmentVariable($variable, "Process")
}

function Invoke-CorrectionSuite([string]$Name, [string]$Package, [string]$Pattern, [string]$Variable, [string]$Dsn) {
    $script:phase = $Name
    [Environment]::SetEnvironmentVariable($Variable, $Dsn, "Process")
    & go test $Package -run $Pattern -count=1 *> $null
    if ($LASTEXITCODE -ne 0) { throw "suite failed" }
    $script:suiteResults.Add([pscustomobject]@{ name = $Name; status = "passed" })
}

try {
    if ([string]::IsNullOrWhiteSpace($reportPath)) {
        $reportPath = Join-Path ([IO.Path]::GetTempPath()) "$containerName.provider-correction.json"
    }
    $reportPath = [IO.Path]::GetFullPath($reportPath)
    $reportParent = Split-Path -Parent $reportPath
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
        $postmasterStart = (& docker exec $containerName psql -U stele -d stele -At -c "SELECT pg_postmaster_start_time()" 2>$null | Out-String).Trim()
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
    foreach ($database in @("correction_api", "correction_resolution", "correction_projection")) {
        $phase = "owned_database_setup_$database"
        & docker exec $containerName createdb -U stele --maintenance-db=stele $database
        if ($LASTEXITCODE -ne 0) { throw "owned test database creation failed" }
    }

    $dsnBase = "postgres://stele:${password}@127.0.0.1:${port}"
    Invoke-CorrectionSuite "correction_resolver_replay_river_retry_handoff" "./internal/storage/postgres" '^TestProviderCorrectionPostgresResolutionReplayAndImmutableAttribution$' "STELE_TEST_PROVIDER_CORRECTION_DSN" "$dsnBase/correction_resolution?sslmode=disable"
    Invoke-CorrectionSuite "projection_feedback_supersession_active_summary" "./internal/storage/postgres" '^TestProjectionPostgresTransactions$' "STELE_TEST_PROVIDER_PROJECTION_DSN" "$dsnBase/correction_projection?sslmode=disable"
    Invoke-CorrectionSuite "correction_governed_successor_restart_stale_worker" "./internal/app" '^TestProviderCorrectionRiverWorkerAppendsGovernedSuccessor$' "STELE_TEST_PROVIDER_CORRECTION_GOVERNED_DSN" "$dsnBase/correction_api?sslmode=disable"

    $phase = "capability_contracts"
    & go test ./internal/provider ./internal/app -run 'TestStrictCorrectionConsumerRequiresAdvertisedCapabilityAndOperation|TestProviderCorrectionRoute' -count=1 *> $null
    if ($LASTEXITCODE -ne 0) { throw "suite failed" }
    $suiteResults.Add([pscustomobject]@{ name = "capability_negotiation_strict_consumer"; status = "passed" })
}
catch {
    $failurePhase = $phase
}
finally {
    foreach ($variable in $verificationVariables) {
        [Environment]::SetEnvironmentVariable($variable, $previousVariables[$variable], "Process")
    }
    if ((& docker ps -aq --filter "name=^$containerName$").Trim()) {
        & docker rm --force $containerName *> $null
    }
    $report = [ordered]@{
        contract = "provider-correction-governed-version-v1"
        status = if ($failurePhase) { "failed" } else { "passed" }
        failed_phase = $failurePhase
        image_digest = $imageDigest
        suites = @($suiteResults)
    }
    $reportJson = $report | ConvertTo-Json -Depth 4
    if ($reportJson.Length -gt 4096) { $reportJson = '{"contract":"provider-correction-governed-version-v1","status":"failed","failed_phase":"report_limit","suites":[]}' }
    Set-Content -LiteralPath $reportPath -Value $reportJson -Encoding UTF8
    Pop-Location
}

if ($failurePhase) { throw "Provider correction verification failed at phase '$failurePhase'; bounded report: $reportPath" }
Write-Output "Provider correction verification passed; bounded report: $reportPath"
